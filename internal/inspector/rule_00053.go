package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00053 = "SQLE00053"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00053,
			Desc:       "不建议使用SELECT *",
			Annotation: "当表结构变更时，使用*通配符选择所有列将导致查询行为会发生更改，与业务期望不符；同时SELECT * 中的无用字段会带来不必要的磁盘I/O，以及网络开销，且无法覆盖索引进而回表，大幅度降低查询效率。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "不建议使用SELECT *",
		RawSQLHandler:        RuleSQLE00053,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00053): "在 TBase 中，不建议使用SELECT *."
您应遵循以下逻辑：
1. 对于所有DML、DQL 语句中的SELECT子句，如果存在SELECT目标是只有一个*符号，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00053(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 SELECT 语句是否违反规则
	checkSelectStarViolation := func(selectStmt *parser.SelectStmt) bool {
		// check select all column
		for _, target := range selectStmt.GetTargetList() {
			if target.GetResTarget() != nil && target.GetResTarget().GetVal() != nil {
				column, ok := target.GetResTarget().GetVal().GetNode().(*parser.Node_ColumnRef)
				if !ok {
					continue
				}
				for _, filed := range column.ColumnRef.GetFields() {
					_, ok = filed.GetNode().(*parser.Node_AStar)
					return ok
				}
			}
		}
		return false
	}

	// 处理所有DML语句中的SELECT子句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if checkSelectStarViolation(selectStmt) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
