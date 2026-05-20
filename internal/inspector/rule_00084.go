package inspector

import (
	"context"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00084 = "SQLE00084"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00084,
			Desc:       "禁止使用临时表",
			Annotation: "在数据库中使用临时表会导致代码可读性差、调试难度增加及数据一致性风险，因此禁止使用临时表。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用临时表",
		RawSQLHandler:        RuleSQLE00084,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00084): "在 TBase 中，禁止使用临时表."
您应遵循以下逻辑：
1. 对于“CREATE ... ”语句, 如果以下任意一项为真，则报告违反规则：
  1. 存在关键词 TEMPORARY（忽略大小写）
  2. 存在关键词 TEMP（忽略大小写）
3. 对于 DML 语句, 登录数据库，执行Explain分析，如果以下任意一项为真，则报告违反规则：
    1. 在Explain的结果中查找 Sort Method 字段，如果其内容存在 external merge 或 disk关键词；
    2. 在Explain的结果中查找 Sort Method 字段，如果其内容存在 external sort 或 disk关键词；
    3. 在Explain的结果中查找 Parallel Hash或者Hash节点，如果存在 Batches，且Batches的值大于1；
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00084(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 处理 CREATE 语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// "create temp table..."
		if stmt.CreateStmt.Relation != nil && stmt.CreateStmt.Relation.Relpersistence == "t" {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	// 检查 DML 语句中的执行计划是否包含违反规则的项
	checkDMLPlanViolation := func(plan *PlanType) bool {
		violationFound := false
		utilVisitPlan(plan, func(plan *PlanType) bool {
			if plan == nil {
				return false
			}
			if strings.Contains(strings.ToLower(plan.SortMethod), "external merge") || strings.Contains(strings.ToLower(plan.SortMethod), "external sort") || strings.Contains(strings.ToLower(plan.SortMethod), "disk") {
				violationFound = true
				return true
			}
			if strings.Contains(strings.ToLower(plan.NodeType), "hash") && (plan.HashBatches > 1) {
				violationFound = true
				return true
			}
			return false
		})
		return violationFound
	}

	// 处理 DML 语句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		explainPlan, err := utilGetExecutionPlan(ctx, sql)
		if err != nil {
			return "", errors.Wrap(err, "get execution plan")
		}
		if checkDMLPlanViolation(explainPlan) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// ==== Rule code end ====
