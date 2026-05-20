package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00031 = "SQLE00031"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00031,
			Desc:       "禁止使用视图",
			Annotation: "视图的查询性能较差，同时基表结构变更，需要对视图进行维护。如果视图可读性差，且包含复杂的逻辑，会增加维护的成本。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用视图",
		RawSQLHandler:        RuleSQLE00031,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00031): "在 TBase 中，禁止使用视图."
您应遵循以下逻辑：
根据您的需求，下面是补全后的规则：

1. 对于 "CREATE ...VIEW..."语句，则报告违反规则。

在这个例子中，因为规则描述中没有关于`SELECT`语句的描述，也没有已存在的或与`UNION`语句相关的描述，所以按照要求原样输出，不需要补充`UNION`语句的描述。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00031(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查是否为 CREATE VIEW 语句
	if _, ok := node.GetStmt().GetNode().(*parser.Node_ViewStmt); ok {
		// 对于“CREATE VIEW ...” 语句，直接报告违反规则
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
