package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00087 = "SQLE00087"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00087,
			Desc:       "避免WHERE条件内IN语句中的参数值个数过多",
			Annotation: "当IN值过多时，有可能会出现无法使用索引，导致查询走全表扫描、性能变差、资源消耗过多等问题。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDMLConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "500",
				Desc:  "IN的参数值个数",
				Type:  params.ParamTypeString,
			}},
		},
		Message:              "避免WHERE条件内IN语句中的参数值个数过多",
		RawSQLHandler:        RuleSQLE00087,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00087): "在 TBase 中，尽量避免in 操作，若实在避免不了，需要仔细评估 in 后边的集合元素数量，控制在 500 个之内."
您应遵循以下逻辑：
1. 对于“SELECT ... ”语句,
  1. 定义一个集合，计算 WHERE 条件后 IN  列表的元素数量。
  2. 从集合中拿出保存的元素数量和当前规则的阈值对比，如果比它大，则报告违反规则。
2. 对于“WITH ... ”语句, 执行与上述同样检查。
3. 对于“INSERT ... SELECT”语句, 执行与上述同样检查。
4. 对于“UPDATE ... ”语句, 执行与上述同样检查。
5. 对于“DELETE ... ”语句, 执行与上述同样检查。

1. 对于“UNION ...”语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。
==== Prompt end ====
*/

// ==== Rule code start ====

// 规则函数实现开始
func RuleSQLE00087(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取阈值
	threshold := utilGetRuleParamInt(rule, DefaultSingleParamKeyName)

	// 检查 IN 子句参数数量是否超过阈值
	checkInClause := func(inExpr *parser.Node_AExpr) bool {
		if inExpr.AExpr.Kind != parser.A_Expr_Kind_AEXPR_IN {
			return false
		}
		inValuesList := inExpr.AExpr.Rexpr.GetList()
		if inValuesList == nil {
			return false
		}
		return len(inValuesList.Items) > threshold
	}

	// 扫描 WHERE 子句中的 IN 表达式并检查参数数量
	checkInClauseInWhere := func(whereClause *parser.Node) bool {
		foundViolation := false
		utilScanWhereStmt(func(expr *parser.Node) bool {
			switch condition := expr.GetNode().(type) {
			case *parser.Node_AExpr:
				if checkInClause(condition) {
					foundViolation = true
					return true
				}
			}
			return false
		}, whereClause)
		return foundViolation
	}

	// 处理 DML 语句
	handleDMLStmt := func(stmt *parser.Node) bool {
		whereClauses := utilGetWhereExprFromDMLStmt(stmt)
		for _, clause := range whereClauses {
			if checkInClauseInWhere(clause) {
				return true
			}
		}
		return false
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		// 处理 SELECT 语句和 UNION 中的 SELECT 语句 和 WITH 中的 SELECT 语句
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if checkInClauseInWhere(selectStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_InsertStmt:
		// 处理 INSERT 语句
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt().GetInsertStmt().SelectStmt) {
			if checkInClauseInWhere(selectStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_UpdateStmt:
		// 处理 UPDATE 语句
		if handleDMLStmt(node.GetStmt()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DeleteStmt:
		// 处理 DELETE 语句
		if handleDMLStmt(node.GetStmt()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
