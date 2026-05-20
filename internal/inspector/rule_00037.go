package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00037 = "SQLE00037"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00037,
			Desc:       "避免一张表内二级索引的个数过多",
			Annotation: "在表上建立的每个索引都会增加存储开销，索引对于插入、删除、更新操作也会增加维护索引处理上的开销（TPS），且太多与不充分、不正确的索引对性能都毫无益处。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeIndexConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "5",
				Desc:  "二级索引个数",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "避免一张表内二级索引的个数过多",
		RawSQLHandler:        RuleSQLE00037,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00037): "在 TBase 中，避免一张表内二级索引的个数过多.二级索引个数:5"
您应遵循以下逻辑：
1. 对于"CREATE INDEX..." 语句，
    1. 定义一个变量
    2. 登录数据库，获取表现有二级索引的个数，更新到变量上
    3. 进行检查，如果变量上的索引个数+1后仍然大于等于规则变量值，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00037(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取规则中定义的二级索引个数阈值
	maxSecondaryIndexes := rule.Params.GetParam(DefaultSingleParamKeyName).Int()
	if maxSecondaryIndexes == 0 {
		return "", errors.New("invalid max secondary indexes")
	}

	// 检查 CREATE INDEX 语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_IndexStmt:
		// 获取表名和模式名
		tableName := stmt.IndexStmt.GetRelation().GetRelname()
		schemaName := stmt.IndexStmt.GetRelation().GetSchemaname()

		// 获取表的索引信息
		indexInfo, err := utilGetTableIndexInfo(ctx, schemaName, tableName)
		if err != nil {
			return "", errors.Wrap(err, "get table index info")
		}

		// 统计二级索引的数量
		secondaryIndexCount := 0
		for _, index := range indexInfo {
			if !index.IsPrimaryKey {
				secondaryIndexCount++
			}
		}

		// 检查二级索引数量是否超过阈值
		if secondaryIndexCount+1 > int(maxSecondaryIndexes) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
