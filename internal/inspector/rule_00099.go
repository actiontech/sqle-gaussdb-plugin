package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00099 = "SQLE00099"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00099,
			Desc:       "不建议在SELECT语句中存在FOR UPDATE操作",
			Annotation: "SELECT FOR UPDATE 会对查询结果集中每行数据都添加排他锁，其他线程对该记录的更新与删除操作都会阻塞，在高并发下，容易造成数据库大量锁等待，影响数据库查询性能。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "不建议在SELECT语句中存在FOR UPDATE操作",
		RawSQLHandler:        RuleSQLE00099,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00099): "在 TBase 中，不建议在SELECT语句中存在FOR UPDATE操作."
您应遵循以下逻辑：
1. 对于所有DML语句中的SELECT子句，检查SQL语句中是否存在FOR UPDATE 操作，如果存在，则报告违反规则。

1. 对于UNION...语句，对于其中的所有SELECT子句进行与SELECT语句相同的检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00099(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 处理所有DML语句中的SELECT子句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if len(selectStmt.LockingClause) >= 1 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
