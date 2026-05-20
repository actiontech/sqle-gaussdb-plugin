package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00030 = "SQLE00030"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00030,
			Desc:       "禁止使用触发器",
			Annotation: "触发器难以开发和维护，不能高效移植，且在复杂的逻辑以及高并发下，容易出现死锁影响业务。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用触发器",
		RawSQLHandler:        RuleSQLE00030,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00030): "在 TBase 中，禁止使用触发器."
您应遵循以下逻辑：
在这个特定的情境中，由于规则描述中不涉及`SELECT...`语句，因此不需要补全任何`UNION`语句的描述。我们只需要按照原样输出规则即可。

1. 对于 "create...TRIGGER..."语句，直接报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00030(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查是否为 CREATE TRIGGER 语句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateTrigStmt:
		// 直接报告违反规则
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
