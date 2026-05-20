package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00091 = "SQLE00091"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00091,
			Desc:       "建议表连接时有连接条件",
			Annotation: "为了确保连接操作的正确性和可靠性，应该始终指定连接条件，定义正确的关联关系。缺少连接条件，可能导致连接操作失败，最终数据库会使用笛卡尔积的方式进行处理，产生不正确的连接结果，并导致性能问题，消耗大量的CPU和内存资源。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "建议表连接时有连接条件",
		RawSQLHandler:        RuleSQLE00091,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00091): "在 TBase 中，建议表连接时有连接条件."
您应遵循以下逻辑：
1. 对于 DML 语句中的所有 select 子句:
  1、检查句子中是否存在FROM子句，如果存在，则进一步检查。
  2、检查FROM子句后面参与表连接的个数是否大于等于2个，如果是，则进一步检查。
  3、检查参与表连接的各表是否存在条件字段，如不存在或条件为恒真，则触发违反规则。

1. 对于UNION...语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。

1. 对于 update...语句，
	1、检查句子中是否存在FROM子句，如果存在，则进一步检查。
	2、检查参与表连接的各表是否存在条件字段，如不存在或条件为恒真，则触发违反规则。
2. 对于 delete...语句，
	1、检查句子中是否存在USING子句，如果存在，则进一步检查。
	2、检查参与表连接的各表是否存在条件字段，如不存在或条件为恒真，则触发违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00091(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 判断是否为 from t1, t2, t3 的形式
	getImplicitJoinTables := func(selectStmt *parser.SelectStmt) (tables []*parser.RangeVar) {
		if selectStmt.FromClause == nil {
			return nil
		}
		for _, node := range selectStmt.FromClause {
			if t, ok := (*node).Node.(*parser.Node_RangeVar); ok {
				tables = append(tables, t.RangeVar)
			}
		}
		return tables
	}

	// Main logic: Check different DML types
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_UpdateStmt, *parser.Node_DeleteStmt:
		// Iterate through all select statements in the DML
		selectStmts := utilGetSelectStmt(node.GetStmt())
		for _, selectStmt := range selectStmts {
			// Check if FROM clause exists
			if len(selectStmt.GetFromClause()) > 0 {
				// check implicit join
				tables := getImplicitJoinTables(selectStmt)
				// Check if the number of tables joined is greater than or equal to 2
				if len(tables) >= 2 {
					// Check if where condition exists
					if selectStmt.WhereClause == nil || utilIsExprConstTrue(selectStmt.WhereClause) {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}

				// check join condition
				for _, joinExpr := range utilGetJoinExpr(&parser.Node{Node: &parser.Node_SelectStmt{SelectStmt: selectStmt}}) {
					if joinExpr.Quals == nil || utilIsExprConstTrue(joinExpr.Quals) {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}
		}
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_UpdateStmt:
		// Check if FROM clause exists
		if stmt.UpdateStmt.GetFromClause() != nil {
			// Check if where condition exists
			if stmt.UpdateStmt.WhereClause == nil || utilIsExprConstTrue(stmt.UpdateStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_DeleteStmt:
		// Check if USING clause exists
		if stmt.DeleteStmt.GetUsingClause() != nil {
			// Check if where condition exists
			if stmt.DeleteStmt.WhereClause == nil || utilIsExprConstTrue(stmt.DeleteStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}

	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
