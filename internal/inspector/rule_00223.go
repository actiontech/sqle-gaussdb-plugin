package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00223 = "SQLE00223"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00223,
			Desc:       "禁止对分片表的分片键进行更新",
			Annotation: "分片表的分片键通常用于数据的分布和路由，对其进行更新可能会导致数据分布混乱和严重的性能问题。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "禁止对分片表的分片键进行更新",
		RawSQLHandler:        RuleSQLE00223,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00223): "在 TBase 中，禁止对分片表的分片键进行更新."
您应遵循以下逻辑：
1. 对于“UPDATE ...”语句，检查更新的字段是否为分片键，如果是，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00223(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查更新的字段是否为分片键
	checkShardKeyUpdateViolation := func(table *parser.RangeVar, targetList []*parser.Node) (bool, error) {
		// 获取表的分片键
		shardKey, err := utilGetTableDistributionColumn(ctx, table.Schemaname, table.Relname)
		if err != nil {
			return false, errors.Wrap(err, "get table distribution column")
		}

		// 检查更新的字段是否为分片键
		for _, target := range targetList {
			if resTarget, ok := target.Node.(*parser.Node_ResTarget); ok {
				if resTarget.ResTarget.GetName() == shardKey {
					return true, nil // 违反规则
				}
			}
		}
		return false, nil
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_UpdateStmt:
		// "update..." 语句
		if stmt.UpdateStmt.TargetList == nil {
			return RuleHandlerMap[rule.Name].Message, nil // 违反规则
		}
		if violation, err := checkShardKeyUpdateViolation(stmt.UpdateStmt.Relation, stmt.UpdateStmt.TargetList); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
