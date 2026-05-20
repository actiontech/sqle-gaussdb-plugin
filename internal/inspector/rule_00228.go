package inspector

import (
	"context"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00228 = "SQLE00228"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00228,
			Desc:       "建议SQL执行过程中使用分片键",
			Annotation: "不使用分片键，会使得SQL 下发到错误的节点，造成数据不必要的远程访问、数据重分布等问题，降低SQL的执行效率。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "建议SQL执行过程中使用分片键",
		RawSQLHandler:        RuleSQLE00228,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00228): "在 TBase 中，建议SQL执行过程中使用分片键."
您应遵循以下逻辑：
1. 对于所有的DML语句，检查SQL语句:
2. 定义一个集合，获取每个操作涉及的表的分片键，并加入到集合中
3. 执行explain (format json)，如果存在多个表和多个节点，确保每个涉及的表的分片键及其所属表名都出现在执行计划的 Relation Name 和 Index Cond 字段中，否则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00228(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取表的分片键
	getShardKey := func(schemaName, tableName string) (string, error) {
		return utilGetTableDistributionColumn(ctx, schemaName, tableName)
	}

	// 检查执行计划是否包含所有分片键
	checkShardKeyInExecutionPlan := func(sql string, shardKeys map[string]string) (bool, error) {
		plan, err := utilGetExecutionPlan(ctx, sql)
		if err != nil {
			return false, errors.Wrap(err, "get execution plan text")
		}
		// 遍历执行计划
		utilVisitPlan(plan, func(planNode *PlanType) bool {
			for table, shardKey := range shardKeys {
				if planNode.RelationName == table && (strings.Contains(planNode.IndexCond, shardKey) || strings.Contains(planNode.Filter, shardKey)) {
					delete(shardKeys, table)
				}
			}
			return false
		})

		return len(shardKeys) == 0, nil
	}

	// 获取 SELECT 语句中的表
	getSelectTables := func(stmt *parser.SelectStmt) []*parser.RangeVar {
		rangeVars := []*parser.RangeVar{}
		for _, from := range stmt.FromClause {
			for _, rangeVar := range utilGetRangeVars(from) {
				rangeVars = append(rangeVars, rangeVar)
			}
		}
		return rangeVars
	}

	// 去除基础的insert语句，这类语句不需要走分片键，如 insert into t1 (id) values ('1');
	if insert, ok := node.GetStmt().GetNode().(*parser.Node_InsertStmt); ok {
		if insert.InsertStmt.SelectStmt != nil {
			if selectStmt, ok := insert.InsertStmt.SelectStmt.GetNode().(*parser.Node_SelectStmt); ok {
				if selectStmt.SelectStmt.FromClause == nil {
					return "", nil
				}
			}
		}
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	tables := []*parser.RangeVar{}
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_UpdateStmt, *parser.Node_DeleteStmt, *parser.Node_InsertStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			tables = append(tables, getSelectTables(selectStmt)...)
		}
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_DeleteStmt:
		// "delete..."
		tables = append(tables, stmt.DeleteStmt.Relation)
	case *parser.Node_UpdateStmt:
		// "update..."
		tables = append(tables, stmt.UpdateStmt.Relation)
	case *parser.Node_InsertStmt:
		// "insert..."
		tables = append(tables, stmt.InsertStmt.Relation)
	}

	shardKeys := make(map[string]string)
	for _, table := range tables {
		shardKey, err := getShardKey(table.Schemaname, table.Relname)
		if err != nil {
			return "", err
		}
		if shardKey != "" {
			shardKeys[table.Relname] = shardKey
		}
	}

	if len(shardKeys) == 0 {
		return "", nil
	}
	if in, err := checkShardKeyInExecutionPlan(sql, shardKeys); err != nil {
		return "", err
	} else if !in {
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
