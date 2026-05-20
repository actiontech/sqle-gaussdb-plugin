package inspector

import (
	"context"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00227 = "SQLE00227"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00227,
			Desc:       "避免数据重分布",
			Annotation: "在分布式数据库中，数据查询进行跨分片操作会出现数据在节点间传输，导致性能下降、网络带宽占用增加、查询响应时间变长，并且增加数据库管理和维护的复杂性。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "避免数据重分布",
		RawSQLHandler:        RuleSQLE00227,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00227): "在 TBase 中，避免数据重分布."
您应遵循以下逻辑：
1. 对于所有的DML语句，登录数据库，执行Explain分析，如果执行计划结果中存在关键词 Distribute results，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00227(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查执行计划是否违反规则
	checkExecutionPlanViolation := func(sql string) (bool, error) {
		// 获取执行计划
		planText, err := utilGetExecutionPlanText(ctx, sql)
		if err != nil {
			return false, errors.Wrap(err, "get execution plan text")
		}

		// 检查执行计划中是否包含 "Distribute results"
		for _, line := range planText {
			if strings.Contains(line, "Distribute results") {
				return true, nil // 违反规则
			}
		}

		return false, nil
	}

	// 处理所有DML语句中的SELECT子句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		if violation, err := checkExecutionPlanViolation(sql); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
