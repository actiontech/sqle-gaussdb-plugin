package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00076 = "SQLE00076"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00076,
			Desc:       "UPDATE/DELETE操作影响行数不建议超过阈值",
			Annotation: "在数据库中，进行修改或删除等数据变更操作时，一次性操作的数据量过大，会消耗大量的系统资源，产生长事务，会导致查询性能下降，影响其他事务或查询的执行。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDMLConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "100000",
				Desc:  "maxRow",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "UPDATE/DELETE操作影响行数不建议超过阈值",
		RawSQLHandler:        RuleSQLE00076,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00076): "在 TBase 中，UPDATE/DELETE操作影响行数不建议超过阈值.maxRow:100000"
您应遵循以下逻辑：
1. 使用Select count(*) from schema.table 并且带上 Update/Delete中的where下发到数据库中，获取影响行数
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00076(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取阈值
	maxRow := utilGetRuleParamInt(rule, DefaultSingleParamKeyName)
	if maxRow <= 0 {
		return "", errors.New("maxRow must be greater than 0")
	}

	// 检查 UPDATE 和 DELETE 语句
	switch stmt := node.GetStmt().Node.(type) {
	case *parser.Node_UpdateStmt:
		if count, err := utilGetRecordCountQuerySQL(ctx, stmt.UpdateStmt.Relation.Schemaname, stmt.UpdateStmt.Relation.Relname, stmt.UpdateStmt.GetWhereClause()); err != nil {
			return "", err
		} else if count > maxRow {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DeleteStmt:
		if count, err := utilGetRecordCountQuerySQL(ctx, stmt.DeleteStmt.Relation.Schemaname, stmt.DeleteStmt.Relation.Relname, stmt.DeleteStmt.GetWhereClause()); err != nil {
			return "", err
		} else if count > maxRow {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
