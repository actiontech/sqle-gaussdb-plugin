package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00232 = "SQLE00232"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00232,
			Desc:       "避免分区表分区数量过多",
			Annotation: "分区表的分区数量太大，会有管理复杂、维护复杂、定位分区消耗过大、元数据浪费过多的存储空间等缺点。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDDLConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "100",
				Desc:  "表分区个数上限",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "避免分区表分区数量过多",
		RawSQLHandler:        RuleSQLE00232,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00232): "在 TBase 中，避免分区表分区数量过多.表分区个数上限:100"
您应遵循以下逻辑：
1. 对于 “CREATE TABLE ... ” 语句，如果定义了partitions且partitions的分区数目大于阈值，则报告违反规则。
1. 对于 “CREATE TABLE ... ” 语句，
   1. 如果定义了partition，针对指定分区表创建分区子表，即包含关键词partition of，则取出of后的表名；
   2. 登录数据库，通过下述SQL语句统计存量的分区表个数，存量分区表个数+1大于阈值，则报告违反规则。
    select count(1) from pg_class where relparent =(select oid from pg_class where relname ='表名');
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00232(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取分区表个数上限
	partitionLimit := utilGetRuleParamInt(rule, DefaultSingleParamKeyName)
	if partitionLimit <= 0 {
		return "", errors.New("partitionLimit must be greater than 0")
	}

	// 检查 CREATE TABLE 语句
	checkCreateTableStmt := func(stmt *parser.CreateStmt) (bool, error) {
		// 获取分区数量
		if stmt.Partspec == nil {
			return false, nil
		}
		if stmt.Partspec.Interval == nil {
			return false, nil
		}
		partitions := int(stmt.Partspec.Interval.NPartitions)

		// 检查分区数量是否超过阈值
		if partitions > partitionLimit {
			return true, nil
		}

		return false, nil
	}

	// 检查 CREATE TABLE ... PARTITION OF 语句
	checkPartitionOfStmt := func(stmt *parser.CreateStmt) (bool, error) {
		for _, r := range stmt.InhRelations {
			for _, table := range utilGetRangeVars(r) {
				childTableList, err := utilGetChildPartitionTableList(ctx, table.Schemaname, table.Relname)
				if err != nil {
					return false, errors.Wrap(err, "get child partition table list")
				}
				if len(childTableList)+1 > partitionLimit {
					return true, nil // 违反规则
				}
			}
		}

		return false, nil
	}

	switch stmt := node.GetStmt().Node.(type) {
	case *parser.Node_CreateStmt:
		// 检查 CREATE TABLE 语句
		if violation, err := checkCreateTableStmt(stmt.CreateStmt); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}

		// 检查 CREATE TABLE ... PARTITION OF 语句
		if violation, err := checkPartitionOfStmt(stmt.CreateStmt); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
