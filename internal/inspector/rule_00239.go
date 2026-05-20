package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00239 = "SQLE00239"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00239,
			Desc:       "禁止对 WHERE 条件的分区键进行数学运算和函数运算",
			Annotation: "对分区键使用函数或数学运算，会导致索引失效和分区裁剪失败，从而严重影响查询性能。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "禁止对 WHERE 条件的分区键进行数学运算和函数运算",
		RawSQLHandler:        RuleSQLE00239,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00239): "在 TBase 中，禁止对 WHERE 条件的分区键进行数学运算和函数运算."
您应遵循以下逻辑：
````
1. 对于所有的DML语句的"SELECT..."子句，检查SQL语句:
  1. 创建一个集合，把WHERE 条件后的使用函数或者数学运算的字段、相应的表名写入集合中，
  2. 登录数据库，使用集合中的表和字段来在线检查，字段是否为表的分区字段。
  3. 如果字段为分区字段，则报告违反规则。
2. 对于"UPDATE..."语句，执行与上面类似的检查。
3. 对于"DELETE..."语句，执行与上面类似的检查。

1. 对于"UNION..."语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。
```
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00239(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查字段集合是否包含分区键并且是否进行了数学运算和函数运算
	checkPartitionKeyViolation := func(tables []*parser.RangeVar, fieldsSet map[string]map[string]struct{}) (bool, error) {
		for _, table := range tables {
			// 获取表的分区键
			partitionKey, err := utilGetTableDistributionColumn(ctx, table.Schemaname, table.Relname)
			if err != nil {
				return false, errors.Wrap(err, "get table distribution column")
			}

			// 检查分区键是否存在于字段集合中并且是否进行了数学运算和函数运算
			if _, found := fieldsSet[""][partitionKey]; found {
				return true, nil // 违反规则
			}
			if alias := utilGetTableAliasName(table); alias != "" {
				if _, found := fieldsSet[alias][partitionKey]; found {
					return true, nil // 违反规则
				}
			}
			if _, found := fieldsSet[table.Relname][partitionKey]; found {
				return true, nil // 违反规则
			}
		}
		return false, nil
	}

	// 获取 WHERE 子句中的字段集合，只包含函数和表达式内的字段
	getFieldsSetFromWhereClause := func(whereClause *parser.Node) map[string]map[string]struct{} {
		fieldsSet := make(map[string]map[string]struct{})
		utilScanWhereStmt(func(expr *parser.Node) (skip bool) {
			switch n := expr.Node.(type) {
			case *parser.Node_AExpr:
				if n.AExpr != nil {
					if n.AExpr.Lexpr != nil {
						if _, ok := n.AExpr.Lexpr.Node.(*parser.Node_ColumnRef); !ok {
							for _, col := range utilGetColumnRefFromNodes(n.AExpr.Lexpr) {
								_, tableName, colName := utilParseColumnRef(col)
								if _, exists := fieldsSet[tableName]; !exists {
									fieldsSet[tableName] = make(map[string]struct{})
								}
								fieldsSet[tableName][colName] = struct{}{}
							}
						}
					}

					if n.AExpr.Rexpr != nil {
						if _, ok := n.AExpr.Rexpr.Node.(*parser.Node_ColumnRef); !ok {
							for _, col := range utilGetColumnRefFromNodes(n.AExpr.Rexpr) {
								_, tableName, colName := utilParseColumnRef(col)
								if _, exists := fieldsSet[tableName]; !exists {
									fieldsSet[tableName] = make(map[string]struct{})
								}
								fieldsSet[tableName][colName] = struct{}{}
							}
						}
					}
				}

				return true // 不再递归
			case *parser.Node_FuncCall:
				for _, col := range utilGetColumnRefFromNodes(n.FuncCall.Args...) {
					_, tableName, colName := utilParseColumnRef(col)
					if _, exists := fieldsSet[tableName]; !exists {
						fieldsSet[tableName] = make(map[string]struct{})
					}
					fieldsSet[tableName][colName] = struct{}{}
				}
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

	// 对于所有的DML语句的"SELECT..."子句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_UpdateStmt, *parser.Node_DeleteStmt, *parser.Node_InsertStmt:
		// 通过 utilGetSelectStmt 获取"select..." 语句 和 "union..." 语句 中的所有子select语句
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if selectStmt.WhereClause == nil {
				return "", nil // 没有 WHERE 子句，不违反规则
			}
			// 获取字段集合
			fieldsSet := getFieldsSetFromWhereClause(selectStmt.WhereClause)
			// 检查分区键违反规则
			if violation, err := checkPartitionKeyViolation(getSelectTables(selectStmt), fieldsSet); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	// 对于"UPDATE..."语句和"DELETE..."语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_UpdateStmt:
		// "update..." 语句
		if stmt.UpdateStmt.WhereClause == nil {
			return "", nil // 没有 WHERE 子句，不违反规则
		}
		fieldsSet := getFieldsSetFromWhereClause(stmt.UpdateStmt.WhereClause)
		if violation, err := checkPartitionKeyViolation([]*parser.RangeVar{stmt.UpdateStmt.Relation}, fieldsSet); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DeleteStmt:
		// "delete..." 语句
		if stmt.DeleteStmt.WhereClause == nil {
			return "", nil // 没有 WHERE 子句，不违反规则
		}
		fieldsSet := getFieldsSetFromWhereClause(stmt.DeleteStmt.WhereClause)
		if violation, err := checkPartitionKeyViolation([]*parser.RangeVar{stmt.DeleteStmt.Relation}, fieldsSet); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
