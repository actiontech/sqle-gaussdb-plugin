package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00247 = "SQLE00247"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00247,
			Desc:       "必须使用分片表，特殊场景下使用复制表。",
			Annotation: "必须使用分片表来打散数据到各个节点上，充分利用硬件资源，提升数据库系统的整体吞吐效率；在特殊场景下（如配置表、路由表等数据量较小的表）可以使用复制表。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "必须使用分片表，特殊场景下使用复制表。",
		RawSQLHandler:        RuleSQLE00247,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00247): "在 TBase 中，必须使用分片表，特殊场景下使用复制表。."
您应遵循以下逻辑：
1. 对于“CREATE TABLE... ”语句，解析SQL语句，
		1. 不是分区子表，即不包含关键词：partition of，则进行下一步检查
		2. 如果不存在distribute by shard关键词，则报告违反规则。

==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00247(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 CREATE TABLE 语句是否包含 distribute by shard 关键词
	checkCreateTableViolation := func(createStmt *parser.CreateStmt) bool {
		if createStmt.Distributeby != nil {
			if createStmt.Distributeby.Disttype == parser.DistributionType_DISTTYPE_SHARD {
				return false // 不违反规则
			}
		}
		return true // 违反规则
	}

	// 根据不同的 DDL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// 检查是否为分区子表
		if len(stmt.CreateStmt.InhRelations) > 0 {
			// 存在分区子表，不违反规则
			return "", nil
		}

		if checkCreateTableViolation(stmt.CreateStmt) {
			return RuleHandlerMap[rule.Name].Message, nil // 违反规则
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
