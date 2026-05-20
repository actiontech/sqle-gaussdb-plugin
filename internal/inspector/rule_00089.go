package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00089 = "SQLE00089"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00089,
			Desc:       "禁止INSERT ... SELECT",
			Annotation: "INSERT... SELECT、INSERT ... WITH 的缺陷主要在SELECT、WITH子句部分。如果SELECT、WITH 子句性能很差，则会导致整体插入性能差；如果整体插入性能差、插入时间长，则会影响数据库系统资源，降低其他正常请求的执行效率。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "禁止INSERT ... SELECT",
		RawSQLHandler:        RuleSQLE00089,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00089): "在 TBase 中，禁止INSERT ... SELECT."
您应遵循以下逻辑：
1. 对于 "INSERT INTO ..."语句，如果存在SELECT子句，则报告违反规则：
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00089(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_InsertStmt:
		// "INSERT INTO ..." 语句
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// "select..." in insert statement
			if selectStmt.FromClause != nil {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
