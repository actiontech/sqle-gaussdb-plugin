package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00132 = "SQLE00132"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00132,
			Desc:       "不推荐使用子查询",
			Annotation: "有些情况下，子查询并不能使用到索引，同时对于返回结果集比较大的子查询，会产生大量的临时表，消耗过多的CPU和IO资源，产生大量的慢查询",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "不推荐使用子查询",
		RawSQLHandler:        RuleSQLE00132,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00132): "在 TBase 中，不推荐使用子查询."
您应遵循以下逻辑：
1. 对于“SELECT ...” 语句，如果以下任意一项为真，则报告违反规则：
  1. 语句中有子查询
2. 对于“INSERT ... SELECT” 语句，执行与上述同样检查。
3. 对于“UPDATE ...” 语句，执行与上述同样检查。
4. 对于“DELETE ...” 语句，执行与上述同样检查。
5. 对于“UNION ALL ...” 语句，执行与上述同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00132(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_UpdateStmt, *parser.Node_DeleteStmt:
		if len(utilGetSubLink(node.GetStmt())) >= 1 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
