package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00013 = "SQLE00013"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00013,
			Desc:       "建议使用 numeric 类型表示精确数值",
			Annotation: "在数据库中，精确数值的表示对于财务数据、统计数据等需要高精度计算的场景至关重要。使用非精确浮点类型如 REAL、DOUBLE PRECISION 可能导致精度丢失、计算误差，从而影响数据的准确性与可靠性。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "建议使用 numeric 类型表示精确数值",
		RawSQLHandler:        RuleSQLE00013,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00013): "在 TBase 中，建议使用 numeric 类型表示精确数值."
您应遵循以下逻辑：
1. 对于“CREATE TABLE ...” 语句，如果以下任意一项为真，则报告违反规则
  1. 语句中包含 REAL 数据类型
  2. 语句中包含 DOUBLE PRECISION 数据类型
2. 对于“ALTER TABLE” 语句，执行与上述同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00013(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 违规列名列表
	violateColumnNames := []string{}

	// 检查是否为 CREATE TABLE 语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// 遍历所有列定义
		for _, elt := range stmt.CreateStmt.TableElts {
			// 检查列类型是否为 REAL 或 DOUBLE PRECISION
			// note:REAL 等价于 float4,DOUBLE PRECISION 等价于 float8
			if utilIsColumnTypeEqual(elt, utilGetFloatTypes()...) {
				violateColumnNames = append(violateColumnNames, utilGetColumnName(elt))
			}
		}

	case *parser.Node_AlterTableStmt:
		// 遍历所有 ALTER TABLE 命令
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddColumn, parser.AlterTableType_AT_AlterColumnType) {
			// 检查新增或修改的列类型是否为 REAL 或 DOUBLE PRECISION
			if utilIsColumnTypeEqual(cmd.Def, utilGetFloatTypes()...) {
				violateColumnNames = append(violateColumnNames, utilGetColumnName(cmd.Def))
			}
		}
	}

	// 如果存在违规列，返回规则消息
	if len(violateColumnNames) > 0 {
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
