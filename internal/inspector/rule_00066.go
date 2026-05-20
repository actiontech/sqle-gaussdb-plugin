package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00066 = "SQLE00066"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00066,
			Desc:       "禁止除索引外的DROP 操作",
			Annotation: "DROP 操作是数据定义语言（DDL）的一部分，一旦执行，将导致无法恢复的数据或结构丢失。在不恰当的情况下执行DROP操作可能导致数据丢失、系统功能缺失甚至业务中断。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止除索引外的DROP 操作",
		RawSQLHandler:        RuleSQLE00066,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00066): "在 TBase 中，禁止除索引外的DROP 操作."
您应遵循以下逻辑：
1. 对于 "ALTER TABLE ..."语句，如果存在以下任何一项，则报告违反规则：
  1. 句子中存在DROP操作且操作对象不是索引。
2. 对于 "Drop..."语句，如果操作对象不是索引，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00066(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_AlterTableStmt:
		// "alter table"
		for _ = range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt,
			parser.AlterTableType_AT_DropConstraint,
			parser.AlterTableType_AT_DropNotNull,
			parser.AlterTableType_AT_DropExpression,
			parser.AlterTableType_AT_DropColumn,
			parser.AlterTableType_AT_DropCluster,
			parser.AlterTableType_AT_DropOids,
			parser.AlterTableType_AT_DropOids,
			parser.AlterTableType_AT_DropInherit,
			parser.AlterTableType_AT_DropOf,
			parser.AlterTableType_AT_DropIdentity) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DropStmt:
		if stmt.DropStmt.RemoveType != parser.ObjectType_OBJECT_INDEX {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DropdbStmt, *parser.Node_DropRoleStmt, *parser.Node_DropTableSpaceStmt,
		*parser.Node_DropOwnedStmt, *parser.Node_DropSubscriptionStmt, *parser.Node_DropUserMappingStmt:
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
