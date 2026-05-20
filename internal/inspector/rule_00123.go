package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00123 = "SQLE00123"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00123,
			Desc:       "禁止使用TRUNCATE操作",
			Annotation: "TRUNCATE 是DDL操作，可以快速清理全表数据，回收磁盘空间，且不支持事务保护的场景应用，应谨慎使用，避免数据丢失。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用TRUNCATE操作",
		RawSQLHandler:        RuleSQLE00123,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00123): "在 TBase 中，禁止使用TRUNCATE操作."
您应遵循以下逻辑：
1. 检查句子中是否存在TRUNCATE子句，如果存在，报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00123(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// Check the type of statement and report violations based on TRUNCATE operations
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_TruncateStmt:
		return RuleHandlerMap[rule.Name].Message, nil
	default:
		return "", nil
	}
}

// ==== Rule code end ====
