package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00113 = "SQLE00113"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00113,
			Desc:       "不建议对条件字段使用负向查询",
			Annotation: "SQL查询条件中存在NOT IN、NOT LIKE、NOT EXISTS、不等于等负向查询条件；特别是针对负向查询的结果集占比整张表结果集非常少的场景下，查询效率将会非常差。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "不建议对条件字段使用负向查询",
		RawSQLHandler:        RuleSQLE00113,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00113): "在 TBase 中，不建议对条件字段使用负向查询."
您应遵循以下逻辑：
1. 对于"SELECT...WHERE..."语句，检查以下条件，如果有任意一个条件触发，则报告违反规则：
  1. WHERE条件中存在NOT、NOT IN、NOT EXISTS、NOT LIKE、NOT BETWEEN关键词中任意一个，但不包含IS NOT NULL。
  2. WHERE条件中存在 操作符是不等于的，如"!="。
2. 对于"INSERT..."语句，对INSERT语句中的SELECT子句进行与上述相同的检查。
3. 对于"UNION..."语句，对于语句中的每个SELECT子句进行与上述相同的检查。
4. 对于"UNION..."语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。
5. 对于"UPDATE...WHERE..."语句，执行与上述相同的检查。
6. 对于"DELETE...WHERE..."语句，执行与上述相同的检查。
7. 对于"WITH..."语句，执行与上述相同的检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00113(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 WHERE 子句是否包含否定查询
	checkNegativeQuery := func(whereClause *parser.Node) bool {
		negative := false
		scanWhereClause := func(callback func(expr *parser.Node) (skip bool)) {
			utilScanWhereStmt(callback, whereClause)
		}
		scanWhereClause(func(expr *parser.Node) (skip bool) {
			switch condition := expr.GetNode().(type) {
			case *parser.Node_AExpr:
				kind := condition.AExpr.Kind
				if kind == parser.A_Expr_Kind_AEXPR_OP {
					// 检查操作符是否为不等于
					if condition.AExpr.Name[0].GetString_().GetSval() == "<>" {
						negative = true
						return true
					}
				} else if kind == parser.A_Expr_Kind_AEXPR_LIKE || kind == parser.A_Expr_Kind_AEXPR_ILIKE || kind == parser.A_Expr_Kind_AEXPR_SIMILAR {
					if condition.AExpr.Name[0].GetString_().GetSval() == "!~~" {
						negative = true
						return true
					}
				} else if kind == parser.A_Expr_Kind_AEXPR_IN {
					if condition.AExpr.Name[0].GetString_().GetSval() == "<>" {
						negative = true
						return true
					}
				} else if kind == parser.A_Expr_Kind_AEXPR_NOT_BETWEEN || kind == parser.A_Expr_Kind_AEXPR_NOT_BETWEEN_SYM {
					negative = true
					return true
				}
			case *parser.Node_BoolExpr:
				if condition.BoolExpr.Boolop == parser.BoolExprType_NOT_EXPR {
					negative = true
					return true
				}
			}
			return false
		})
		return negative
	}

	// 获取 WHERE 子句
	whereClauses := utilGetWhereExprFromDMLStmt(node.GetStmt())
	for _, clause := range whereClauses {
		if checkNegativeQuery(clause) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
