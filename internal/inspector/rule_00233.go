package inspector

import (
	"context"
	"regexp"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00233 = "SQLE00233"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00233,
			Desc:       "禁止对分片表使用CTE递归",
			Annotation: "在TBase数据库中，不支持对分片表使用CTE递归。通常，TBase默认创建的表都是分片表，若在分片表上使用CTE递归查询或处理数据，无论是通过视图、函数、存储过程以及单独使用，在SQL语句执行过程中都会报语法错误。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "禁止对分片表使用CTE递归",
		RawSQLHandler:        RuleSQLE00233,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00233): "在 TBase 中，禁止对分片表使用CTE递归."
您应遵循以下逻辑：
1. 对于"WITH ..."语句，若存在关键词：RECURSIVE，且WITH中的查询涉及分片表，则报告违反规则。
2. 对于“CREATE VIEW ...” 语句，执行与上述同样检查。
3. 对于“CREATE FUNCTION ...” 语句，执行与上述同样检查。
4. 对于“CREATE PROCEDURE ...” 语句，执行与上述同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00233(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_ViewStmt:
		// 检查 WITH 语句、CREATE VIEW ...中是否存在 RECURSIVE 关键字
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if selectStmt.WithClause != nil && selectStmt.WithClause.Recursive {
				distributed := false
				for _, cte := range selectStmt.WithClause.Ctes {
					for _, expr := range utilGetCommonTableExpr(cte) {

						// 检查 CTE 中的表是否为分片表
						for _, table := range utilGetRangeVars(expr.Ctequery) {
							d, err := utilIsTableDistributed(ctx, table.Schemaname, table.Relname)
							if err != nil {
								return "", err
							}
							if d {
								distributed = true
								break
							}
						}

					}
				}

				if distributed {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}

	case *parser.Node_CreateFunctionStmt:
		// CREATE FUNCTION ... 和 CREATE PROCEDURE ... 中是否存在 RECURSIVE 关键字
		// 解析器目前无法解析CreateFunctionStmt方法体内部的语法，所以这里使用字符串匹配进行，且不区分分片表还是复制表
		// 正则表达式匹配 WITH RECURSIVE
		re := regexp.MustCompile(`WITH\s+RECURSIVE`)
		if re.MatchString(strings.ToUpper(sql)) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// ==== Rule code end ====
