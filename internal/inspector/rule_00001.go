package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00001 = "SQLE00001"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00001,
			Desc:       "建议在DML语句中使用有效的（非恒为TRUE）WHERE条件",
			Annotation: "使用有效的WHERE条件能够避免全表扫描，提高SQL执行效率；而恒为TRUE的WHERE条件，如where 1=1，在执行时会进行全表扫描产生额外开销",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "建议在DML语句中使用有效的（非恒为TRUE）WHERE条件",
		RawSQLHandler:        RuleSQLE00001,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00001): "在 TBase 中，建议在DML语句中使用有效的（非恒为TRUE）WHERE条件."
您应遵循以下逻辑：
1. 对于 DML 语句:
  1. 检查语句中是否含有WHERE关键字，若没有则报告违反规则
  2. 检查语句中WHERE是否含有一个以上的条件，且条件不全为恒为TRUE的条件，如1=1，若不满足则报告违反规则
1. 对于 UNION... 语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00001(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 WHERE 子句是存在且恒为真
	checkWhereClause := func(whereClause *parser.Node) bool {
		if whereClause == nil {
			return true
		}
		// 使用辅助函数检查 WHERE 条件是恒为真
		return utilIsExprConstTrue(whereClause)
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// "select..."

			if checkWhereClause(selectStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_InsertStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// "select..." in insert statement
			if selectStmt.FromClause != nil {
				if checkWhereClause(selectStmt.WhereClause) {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	case *parser.Node_DeleteStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// "select..."
			if checkWhereClause(selectStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		// "delete..."
		if checkWhereClause(stmt.DeleteStmt.WhereClause) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_UpdateStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// "select..."
			if checkWhereClause(selectStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		// "update..."
		if checkWhereClause(stmt.UpdateStmt.WhereClause) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// ==== Rule code end ====
