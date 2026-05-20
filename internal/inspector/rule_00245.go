package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00245 = "SQLE00245"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00245,
			Desc:       "禁止在业务的更新类SQL语句中使用JOIN操作",
			Annotation: "在业务的更新类 SQL 语句中使用 JOIN 会导致性能下降、分布式锁、大量并发时的锁争用增加和维护困难，因此应避免使用。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "禁止在业务的更新类SQL语句中使用JOIN操作",
		RawSQLHandler:        RuleSQLE00245,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00245): "在 TBase 中，禁止在业务的更新类SQL语句中使用JOIN操作."
您应遵循以下逻辑：
1. 对于“UPDATE ... ”语句, FROM子句包含多个表或子查询，则报告违反规则。
2. 对于“DELETE ... ”语句, 使用了USING子句且包含多个表或子查询，则报告违反规则。
3. 对于“WITH ... ”语句, 如句子中存在UPDATE操作，且UPDATE操作的FROM子句包含多个表或子查询，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00245(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 UPDATE 语句的 FROM 子句是否包含多个表或子查询
	checkUpdateJoinViolation := func(updateStmt *parser.UpdateStmt) bool {
		if updateStmt.Relation != nil && len(updateStmt.FromClause) > 1 {
			return true
		}
		return false
	}

	// 检查 DELETE 语句的 USING 子句是否包含多个表或子查询
	checkDeleteJoinViolation := func(deleteStmt *parser.DeleteStmt) bool {
		if deleteStmt.UsingClause != nil && len(deleteStmt.UsingClause) > 1 {
			return true
		}
		return false
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_UpdateStmt:
		if checkUpdateJoinViolation(stmt.UpdateStmt) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DeleteStmt:
		if checkDeleteJoinViolation(stmt.DeleteStmt) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
