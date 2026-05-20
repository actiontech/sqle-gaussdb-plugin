package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00020 = "SQLE00020"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00020,
			Desc:       "避免表中包含有太多的列",
			Annotation: "数据库表中字段过多会导致数据操作效率降低、数据完整性检查成本增加，以及索引维护与更新效率之间的权衡成本。对于追求事务响应和处理速度的OLTP系统，应尽量避免宽表设计，采用规范化数据模型以提升性能。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDDLConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "40",
				Desc:  "表内列数上限",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "避免表中包含有太多的列",
		RawSQLHandler:        RuleSQLE00020,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00020): "在 TBase 中，避免表中包含有太多的列.表内列数上限:40"
您应遵循以下逻辑：
1. 对于“CREATE TABLE ...” 语句，统计定义的字段个数，若字段个数超过阈值，则报告违反规则。
1. 对于“ALTER TABLE  ...” 语句：
  1. 定义一个集合，
  2. 登录数据库，把变更表的字段个数写入集合中
  3. 统计当前语句中的DROP 个数、ADD 个数，最终算出最后的字段个数（加字段的个数多，结果为正数；删字段个数多，结果为负数）
  4. 使用集合中的字段个数和上一步计算的字段个数相加，如果结果超过阈值，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00020(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取表内列数上限
	maxColumns := utilGetRuleParamInt(rule, DefaultSingleParamKeyName)
	if maxColumns <= 0 {
		return "", errors.New("maxColumns must be greater than 0")
	}

	// 检查 CREATE TABLE 语句
	checkCreateTableViolation := func(createStmt *parser.CreateStmt) bool {
		columnCount := len(createStmt.TableElts)
		return columnCount > maxColumns
	}

	// 检查 ALTER TABLE 语句
	checkAlterTableViolation := func(alterStmt *parser.AlterTableStmt) (bool, error) {
		// 获取表名
		tableName := alterStmt.Relation.Relname
		schemaName := alterStmt.Relation.Schemaname

		// 获取表的当前列数
		tableColumns, err := utilGetTableColumnsInfo(ctx, schemaName, tableName)
		if err != nil {
			return false, errors.Wrap(err, "get table columns info")
		}
		currentColumnCount := len(tableColumns)

		// 计算新增和删除的列数
		addCount := 0
		dropCount := 0
		for _ = range utilGetAlterTableCommandsByTypes(alterStmt, parser.AlterTableType_AT_AddColumn) {
			addCount++
		}
		for _ = range utilGetAlterTableCommandsByTypes(alterStmt, parser.AlterTableType_AT_DropColumn) {
			dropCount++
		}

		// 最终列数
		finalColumnCount := currentColumnCount + addCount - dropCount
		return finalColumnCount > maxColumns, nil
	}

	// 主逻辑：根据不同的 DDL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// 处理 CREATE TABLE 语句
		if checkCreateTableViolation(stmt.CreateStmt) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterTableStmt:
		// 处理 ALTER TABLE 语句
		if violation, err := checkAlterTableViolation(stmt.AlterTableStmt); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
