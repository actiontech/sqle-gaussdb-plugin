package inspector

import (
	"context"
	"fmt"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00231 = "SQLE00231"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00231,
			Desc:       "禁止视图内嵌套视图",
			Annotation: "禁止视图内嵌套视图。内嵌视图代码编写复杂、对视图调用性能差。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "禁止视图内嵌套视图",
		RawSQLHandler:        RuleSQLE00231,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00231): "在 TBase 中，禁止视图内嵌套视图."
您应遵循以下逻辑：
1. 对于“CREATE VIEW ...” 语句：
  1. 定义一个集合
  2. 把视图定义中涉及到的表名以字符向量的方式，依次存入集合。
  3. 登录数据库，并从集合中拿出值，带入以下SQL语句查询：
      select count(*) from pg_views where schemaname='模式名' and viewname in (表列表) and viewowner='属主名';
  4. 如果以上SQL 返回结果大于0，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00231(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查视图定义是否违反规则
	checkViewViolation := func(viewStmt *parser.ViewStmt) (bool, error) {
		// 定义一个集合存储视图定义中涉及到的表名
		tableNames := make(map[string]struct{})
		schemaName := ""
		// 获取视图定义中的表名
		for _, rangeVar := range utilGetRangeVars(viewStmt.Query) {
			schemaName = rangeVar.Schemaname
			tableNames[rangeVar.Relname] = struct{}{}
		}

		// 将表名集合转换为字符向量
		var tableList []string
		for tableName := range tableNames {
			tableList = append(tableList, fmt.Sprintf("'%s'", tableName))
		}

		views, err := utilGetExistedViews(ctx, schemaName) // 获取已存在的视图
		if err != nil {
			return false, errors.Wrap(err, "get existed views")
		}

		// 检查视图中使用的表是否是视图
		for _, view := range views {
			if _, ok := tableNames[view.TableName]; ok {
				return true, nil
			}
		}
		return false, nil
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_ViewStmt:
		// 对于“CREATE VIEW ...” 语句
		if violation, err := checkViewViolation(stmt.ViewStmt); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
