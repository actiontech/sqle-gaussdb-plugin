package inspector

import (
	"context"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00082 = "SQLE00082"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00082,
			Desc:       "禁止使用文件排序",
			Annotation: "大数据量的情况下，文件排序意味着SQL性能较低，会增加OS的开销，影响数据库性能。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用文件排序",
		RawSQLHandler:        RuleSQLE00082,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00082): "在 TBase 中，禁止使用文件排序."
您应遵循以下逻辑：
1. 对于 DQL 语句, 登录数据库，执行 explain (analyze,json) 分析，如果以下任意一项为真，则报告违反规则：
	1. 在 explain (analyze,json) 的结果中查找 Sort Method 字段，如果其内容存在 external sort 关键词；
	2. 在 explain (analyze,json) 的结果中查找 Sort Space Type 字段，如果其内容存在 disk 关键词；
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00082(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取执行计划并检查排序方法
	checkSortMethod := func(plan *PlanType) bool {
		found := false
		utilVisitPlan(plan, func(plan *PlanType) bool {
			if plan.SortMethod != "" && (strings.Contains(strings.ToLower(plan.SortMethod), "external sort")) {
				found = true
				return true // 提前终止遍历
			}
			if plan.SortSpaceType != "" && (strings.Contains(strings.ToLower(plan.SortSpaceType), "disk")) {
				found = true
				return true // 提前终止遍历
			}
			return false
		})
		return found
	}

	// Main logic: Check different DQL types
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		// 获取执行计划
		plan, err := utilGetExecutionAnalyzePlan(ctx, sql)
		if err != nil {
			return "", errors.Wrap(err, "get execution analyze plan")
		}

		// 检查执行计划中的排序方法
		if checkSortMethod(plan) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// ==== Rule code end ====
