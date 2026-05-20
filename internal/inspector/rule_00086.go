package inspector

import (
	"context"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00086 = "SQLE00086"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00086,
			Desc:       "禁止使用子字符串或后缀匹配搜索",
			Annotation: "使用子字符串匹配或后缀匹配搜索将导致查询无法使用索引，导致全表扫描",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用子字符串匹配或后缀匹配搜索",
		RawSQLHandler:        RuleSQLE00086,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00086): "在 TBase 中，禁止使用子字符串匹配或后缀匹配搜索."
您应遵循以下逻辑：
1、检查SELECT句子中是否存在WHERE子句，如果存在，则进一步检查。
2、检查WHERE条件中是否存在LIKE关键词，如果存在，则进一步检查。
3、检查LIKE关联的条件值是否存在后缀或者子字符串匹配，如果存在，则报告违反规则。

1. 对于UNION...语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。

1、检查INSERT INTO 句子中是否存在SELECT子句，如果存在，则进一步检查。
2、检查SELECT句子中是否存在WHERE子句，如果存在，则进一步检查。
3、检查WHERE条件中是否存在LIKE关键词，如果存在，则进一步检查。
4、检查LIKE关联的条件值是否存在后缀或者子字符串匹配，如果存在，报告违反规则。

1. 对于UNION...语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。

1、检查UPDATE句子中是否存在WHERE子句，如果存在，则进一步检查。
2、检查WHERE条件中是否存在LIKE关键词，如果存在，则进一步检查。
3、检查LIKE关联的条件值是否存在后缀或者子字符串匹配，如果存在，报告违反规则。

1、检查DELETE句子中是否存在WHERE条件，如果存在，则进一步检查。
2、检查WHERE条件中是否存在LIKE关键词，如果存在，则进一步检查。
3、检查LIKE关联的条件值是否存在后缀或者子字符串匹配，如果存在，报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00086(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// Check if the WHERE clause contains a left LIKE clause or all LIKE clause
	checkLikeClause := func(whereClause *parser.Node) bool {
		found := false
		utilScanWhereStmt(func(expr *parser.Node) (skip bool) {
			switch condition := expr.GetNode().(type) {
			case *parser.Node_AExpr:
				kind := condition.AExpr.Kind
				if kind == parser.A_Expr_Kind_AEXPR_LIKE || kind == parser.A_Expr_Kind_AEXPR_ILIKE || kind == parser.A_Expr_Kind_AEXPR_SIMILAR {
					rexpr := condition.AExpr.Rexpr.GetAConst()
					if rexpr != nil {
						if strings.HasPrefix(rexpr.GetSval().GetSval(), "%") ||
							strings.HasSuffix(rexpr.GetSval().GetSval(), "_") {
							found = true

						}
					}
				}
			}
			return false
		}, whereClause)
		return found
	}

	whereClauses := utilGetWhereExprFromDMLStmt(node.GetStmt())
	for _, clause := range whereClauses {
		if checkLikeClause(clause) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
