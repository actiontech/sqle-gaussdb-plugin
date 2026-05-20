package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00110 = "SQLE00110"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00110,
			Desc:       "SQL查询条件的字段必须包含索引",
			Annotation: "使用索引可以显著提高SQL查询的性能。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeIndexConvention,
			Params:     nil,
		},
		Message:              "SQL查询条件的字段必须包含索引",
		RawSQLHandler:        RuleSQLE00110,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00110): "在 TBase 中, SQL查询条件的字段必须包含索引"
您应遵循以下逻辑：
1. 对于"SELECT..."语句，检查SQL语句:
	1. 创建一个集合，把表名、WHERE 条件的过滤字段、group by和order by的字段都写入集合中，
	2. 登录数据库，使用集合中保存的字段来检查其在对应表中有无索引。
	3. 如果没有索引，则报告违反规则。
2. 对于"INSERT...SELECT..."语句，执行与上面类似的检查。
3. 对于"UPDATE..."语句，执行与上面类似的检查。
4. 对于"DELETE..."语句，执行与上面类似的检查。
5. 对于"... UNION ALL ..."语句，执行与上面类似的检查。
6. 对于"WITH..."语句，执行与上面类似的检查。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00110(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	checkIndexViolation := func(tables []*parser.RangeVar, whereClause *parser.Node, groupClause, sortClause []*parser.Node) (violation bool, err error) {
		clause := []*parser.Node{}
		if whereClause != nil {
			clause = append(clause, whereClause)
		}
		for _, group := range groupClause {
			clause = append(clause, group)
		}
		for _, sort := range sortClause {
			clause = append(clause, sort)
		}
		columnsToCheck := make([]*parser.ColumnRef, 0)
		for _, c := range clause {
			columnsToCheck = append(columnsToCheck, utilGetColumnRefFromNodes(c)...)
		}

		for _, table := range tables {
			// 获取表的索引信息
			indexInfo, err := utilGetTableIndexInfo(ctx, table.Schemaname, table.Relname)
			if err != nil {
				return false, errors.Wrap(err, "get table index info")
			}

			// 检查每个字段是否有索引
			for _, column := range columnsToCheck {
				if !utilIsColumnIndexed(column, indexInfo) {
					// 报告违反规则
					return true, nil
				}
			}
		}
		return false, nil
	}

	getSelectTables := func(stmt *parser.SelectStmt) []*parser.RangeVar {
		// 获取表信息
		rangeVars := []*parser.RangeVar{}
		for _, from := range stmt.FromClause {
			for _, rangeVar := range utilGetRangeVars(from) {
				rangeVars = append(rangeVars, rangeVar)
			}
		}
		return rangeVars
	}

	// 处理不同的 SQL 语句类型
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		// "select..." 语句 和 "union..." 语句
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// "select..." in select statement
			if violation, err := checkIndexViolation(getSelectTables(selectStmt), selectStmt.WhereClause, selectStmt.GroupClause, selectStmt.SortClause); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}

	case *parser.Node_UpdateStmt:
		// "update..." 语句
		updateStmt := node.GetStmt().GetUpdateStmt()
		if violation, err := checkIndexViolation([]*parser.RangeVar{updateStmt.Relation}, updateStmt.WhereClause, nil, nil); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}

		// "select..." in update statement
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if violation, err := checkIndexViolation(getSelectTables(selectStmt), selectStmt.WhereClause, selectStmt.GroupClause, selectStmt.SortClause); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_DeleteStmt:
		// "delete..." 语句
		deleteStmt := node.GetStmt().GetDeleteStmt()
		if violation, err := checkIndexViolation([]*parser.RangeVar{deleteStmt.Relation}, deleteStmt.WhereClause, nil, nil); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}

		// "select..." in delete statement
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if violation, err := checkIndexViolation(getSelectTables(selectStmt), selectStmt.WhereClause, selectStmt.GroupClause, selectStmt.SortClause); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_InsertStmt:
		// "insert..." 语句

		// "select..." in insert statement
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// 排除作为 insert values 中的 select 子句
			if selectStmt.FromClause != nil {
				if violation, err := checkIndexViolation(getSelectTables(selectStmt), selectStmt.WhereClause, selectStmt.GroupClause, selectStmt.SortClause); err != nil {
					return "", err
				} else if violation {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	}

	return "", nil
}

// ==== Rule code end ====
