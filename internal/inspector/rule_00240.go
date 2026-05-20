package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00240 = "SQLE00240"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00240,
			Desc:       "禁止使用自研分区",
			Annotation: "TBase数据库提供了两种分区表创建方法：pgxc原生分区方法和TBase自研分区方法。为了确保更高的灵活性、管理性和扩展性，建议用户尽量避免使用TBase自研的分区方法，转而使用pgxc原生的分区方法。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用自研分区",
		RawSQLHandler:        RuleSQLE00240,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00240): "在 TBase 中，禁止使用自研分区."
您应遵循以下逻辑：
1. 对于“CREATE TABLE ...” 语句，如果存在关键词： partitions，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00240(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 CREATE TABLE 语句是否包含 partitions 关键词
	checkCreateTableViolation := func(stmt *parser.CreateStmt) bool {
		// 检查 CREATE TABLE 语句
		if stmt.Partspec == nil {
			return false
		}
		if stmt.Partspec.Interval == nil {
			return false
		}

		if stmt.Partspec.Interval.NPartitions > 0 {
			return true
		}

		return false
	}

	// 根据不同的 DDL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		if checkCreateTableViolation(stmt.CreateStmt) {
			return RuleHandlerMap[rule.Name].Message, nil // 违反规则
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
