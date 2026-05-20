package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00017 = "SQLE00017"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00017,
			Desc:       "不建议使用BYTEA或TEXT类型",
			Annotation: "BYTEA或TEXT类型有可能消耗大量的磁盘空间、网络IO带宽、应该限制其存入太多过量的数据。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "不建议使用BYTEA或TEXT类型",
		RawSQLHandler:        RuleSQLE00017,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00017): "在 TBase 中，不建议使用BYTEA或TEXT类型."
您应遵循以下逻辑：
1. 对于“CREATE TABLE ... ” 语句，如果下面任意一项为真，则报告违反规则：
  1. 存在字段类型为 BYTEA的定义
  2. 存在字段类型为 TEXT 的定义
2. 对于“ALTER TABLE ...” 语句，执行与上述同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00017(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 违规列名列表
	violateColumnNames := []string{}

	// 检查 CREATE TABLE 语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		for _, elt := range stmt.CreateStmt.TableElts {
			if utilIsColumnTypeEqual(elt, SqlTypeBytea, SqlTypeText) {
				violateColumnNames = append(violateColumnNames, utilGetColumnName(elt))
			}
		}

	// 检查 ALTER TABLE 语句
	case *parser.Node_AlterTableStmt:
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddColumn) {
			if utilIsColumnTypeEqual(cmd.Def, SqlTypeBytea, SqlTypeText) {
				violateColumnNames = append(violateColumnNames, utilGetColumnName(cmd.Def))
			}
		}

		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AlterColumnType) {
			// "alter table ... alter column ... TYPE"
			if utilIsColumnTypeEqual(cmd.Def, SqlTypeBytea, SqlTypeText) {
				violateColumnNames = append(violateColumnNames, cmd.Name)
			}
		}
	}

	// 如果存在违反规则的列，返回相应的规则消息
	if len(violateColumnNames) > 0 {
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

// 规则函数实现结束

// 辅助函数实现开始
// 这里没有新的辅助函数
// 辅助函数实现结束
// ==== Rule code end ====
