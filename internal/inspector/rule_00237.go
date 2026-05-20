package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00237 = "SQLE00237"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00237,
			Desc:       "存储过程或函数不允许使用OUT参数",
			Annotation: "TBase数据库默认不支持存储过程语法，只有部署的是兼容Oracle语法时，允许定义存储过程。在定义存储过程中，添加out参数后，实际上数据库自动识别为in。且在上述情况下，若使用的 为jdbc进行交互，会导致驱动端自动将out参数过滤，导致无法命中目标存储过程。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "存储过程或函数不允许使用OUT参数",
		RawSQLHandler:        RuleSQLE00237,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00237): "在 TBase 中，存储过程或函数不允许使用OUT参数."
您应遵循以下逻辑：
1. 对于 " CREATE FUNCTION..."语句，如果存在以下任何一项，则报告违反规则：
  1. 语句中参数部分包含OUT或者IN OUT 关键词声明的参数定义。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00237(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 CREATE FUNCTION 语句是否违反规则
	checkFunctionViolation := func(funcStmt *parser.CreateFunctionStmt) bool {
		for _, arg := range funcStmt.Parameters {
			// 检查参数定义中是否包含 OUT 或 IN OUT 关键词
			param := arg.GetFunctionParameter()
			if param != nil && (param.Mode == parser.FunctionParameterMode_FUNC_PARAM_OUT || param.Mode == parser.FunctionParameterMode_FUNC_PARAM_INOUT) {
				return true
			}
		}
		return false
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateFunctionStmt:
		if checkFunctionViolation(stmt.CreateFunctionStmt) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
