package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00221 = "SQLE00221"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00221,
			Desc:       "对分片表的SELECT、UPDATE、DELETE 操作，必须带分片键",
			Annotation: "对分片表的SELECT、UPDATE、DELETE 操作，必须包含分片键，且不能对分片使用函数、表达式等计算操作；如果不然，数据将下发到不正确的分片节点。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "对分片表的SELECT、UPDATE、DELETE 操作，必须带分片键",
		RawSQLHandler:        RuleSQLE00221,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00221): "在 TBase 中，对分片表的SELECT、UPDATE、DELETE 操作，必须带分片键."
您应遵循以下逻辑：
1. 对于所有DML语句的“SELECT ...”子句， 分两种场景：
	1. 不带 WHERE 条件的，直接报告违反规则。
	2. 带 WHERE 条件的，
		1. 解析 WHERE 条件中的字段和表达式，生成抽象语法树 (AST)。
		2. 登录数据库，使用 "\d+ 表名" 获取对应表的分片键。
		3. 检查 WHERE 子句中是否包含分片键：
			1. 如果分片键不在字段集合中，报告违反规则。
			2. 如果分片键在字段集合中，但对分片键使用了函数、表达式或计算操作，报告违反规则。

1. 对于UNION...语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。

2. 对于“UPDATE ...”语句，执行与上述同样检查。

3. 对于“DELETE ...”语句，执行与上述同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00221(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查字段集合是否包含分片键
	checkShardKeyViolation := func(tables []*parser.RangeVar, fieldsSet map[string]map[string]struct{}) (bool, error) {

		for _, table := range tables {
			// 获取表的分片键
			shardKey, err := utilGetTableDistributionColumn(ctx, table.Schemaname, table.Relname)
			if err != nil {
				return false, errors.Wrap(err, "get table distribution column")
			}

			// 检查分片键是否存在于字段集合中

			// 不使用表前缀的情况：where v1 = 1
			if _, found := fieldsSet[""][shardKey]; found {
				return false, nil // 不违反规则

			}
			// 使用表别名的情况：where a.v1 = 1
			if alias := utilGetTableAliasName(table); alias != "" {
				if _, found := fieldsSet[alias][shardKey]; found {
					return false, nil // 不违反规则
				}
			}
			// 不使用表别名的情况：where tb1.v1 = 1
			if _, found := fieldsSet[table.Relname][shardKey]; found {
				return false, nil // 不违反规则
			}
		}

		return true, nil
	}

	// 获取 WHERE 子句中的字段集合，去除包含函数和表达式内的字段
	getFieldsSetFromWhereClause := func(whereClause *parser.Node) map[string]map[string]struct{} {
		fieldsSet := make(map[string]map[string]struct{})
		utilScanWhereStmt(func(expr *parser.Node) (skip bool) {
			switch n := expr.Node.(type) {
			case *parser.Node_AExpr:
				if n.AExpr != nil {
					if n.AExpr.Lexpr != nil {
						if n, ok := n.AExpr.Lexpr.Node.(*parser.Node_ColumnRef); ok {
							_, tableName, colName := utilParseColumnRef(n.ColumnRef)
							if _, exists := fieldsSet[tableName]; !exists {
								fieldsSet[tableName] = make(map[string]struct{})
							}
							fieldsSet[tableName][colName] = struct{}{}
						}
					}

					if n.AExpr.Rexpr != nil {
						if n, ok := n.AExpr.Rexpr.Node.(*parser.Node_ColumnRef); ok {
							_, tableName, colName := utilParseColumnRef(n.ColumnRef)
							if _, exists := fieldsSet[tableName]; !exists {
								fieldsSet[tableName] = make(map[string]struct{})
							}
							fieldsSet[tableName][colName] = struct{}{}
						}
					}
				}

				return true // 不再递归
			case *parser.Node_BoolExpr:
				return false // 继续递归
			}

			return false
		}, whereClause)

		return fieldsSet
	}

	// 获取 SELECT 语句中的表
	getSelectTables := func(stmt *parser.SelectStmt) []*parser.RangeVar {
		rangeVars := []*parser.RangeVar{}
		for _, from := range stmt.FromClause {
			for _, rangeVar := range utilGetRangeVars(from) {
				rangeVars = append(rangeVars, rangeVar)
			}
		}
		return rangeVars
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		// 通过 utilGetSelectStmt 获取"select..." 语句 和 "union..." 语句 中的所有子select语句
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if selectStmt.WhereClause == nil {
				return RuleHandlerMap[rule.Name].Message, nil // 违反规则
			}
			// 获取字段集合
			fieldsSet := getFieldsSetFromWhereClause(selectStmt.WhereClause)
			// 检查分片键违反规则
			if violation, err := checkShardKeyViolation(getSelectTables(selectStmt), fieldsSet); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_UpdateStmt:
		// "update..." 语句
		if stmt.UpdateStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil // 违反规则
		}
		fieldsSet := getFieldsSetFromWhereClause(stmt.UpdateStmt.WhereClause)
		if violation, err := checkShardKeyViolation([]*parser.RangeVar{stmt.UpdateStmt.Relation}, fieldsSet); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}

	case *parser.Node_DeleteStmt:
		// "delete..." 语句
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil // 违反规则
		}
		fieldsSet := getFieldsSetFromWhereClause(stmt.DeleteStmt.WhereClause)
		if violation, err := checkShardKeyViolation([]*parser.RangeVar{stmt.DeleteStmt.Relation}, fieldsSet); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
