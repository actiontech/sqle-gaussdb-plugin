package inspector

import (
	"context"
	"fmt"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00056 = "SQLE00056"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00056,
			Desc:       "表建议使用指定的字符集",
			Annotation: "数据库内使用非标准的字符集，可能导致字符无法编码或者编码不全引起的乱码，最终出现应用写入数据失败或者查询结果显示乱码，影响数据库服务可用性。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "UTF8MB4",
				Desc:  "标准字符集",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "表建议使用指定的字符集：%s",
		RawSQLHandler:        RuleSQLE00056,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00056): "在 TBase 中，表建议使用指定的字符集.标准字符集:UTF8MB4"
您应遵循以下逻辑：
1. 对于 “CREATE DATABASE ...” 语句，存在encoding关键词且encoding的值与阈值不一致时，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00056(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查 CREATE DATABASE 语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreatedbStmt:
		// 获取 encoding 参数值
		encoding := utilGetCreateDatabaseEncoding(stmt.CreatedbStmt)
		// 获取规则指定的标准字符集
		expectedEncoding := utilGetRuleParamString(rule, DefaultSingleParamKeyName)
		if encoding != "" && !strings.EqualFold(encoding, expectedEncoding) {
			message := fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedEncoding)
			// 如果存在 encoding 参数且与标准字符集不一致，则报告违反规则
			return message, nil
		}
	}

	return "", nil
}

// ==== Rule code end ====
