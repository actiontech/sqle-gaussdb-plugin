package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00029 = "SQLE00029"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00029,
			Desc:       "禁止使用存储过程",
			Annotation: "存储过程在一定程度上能使程序难以调试和拓展，各种数据库端的存储过程语法相差很大，给将来的数据移植带来很大的困难，且会极大的出现BUG的几率",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用存储过程",
		RawSQLHandler:        RuleSQLE00029,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00029): "在 TBase 中，禁止使用存储过程."
您应遵循以下逻辑：
1. 对于 "create...procedure..."语句，直接报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00029(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查是否为 CREATE PROCEDURE 语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateFunctionStmt:
		if stmt.CreateFunctionStmt.IsProcedure {
			return RuleHandlerMap[rule.Name].Message, nil // 违反规则
		}
	}

	return "", nil
}
