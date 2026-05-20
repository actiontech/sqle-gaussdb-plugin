package inspector

import (
	"context"
	"fmt"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00055 = "SQLE00055"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00055,
			Desc:       "不建议创建冗余索引",
			Annotation: "数据库需要单独维护重复的索引，冗余索引增加维护成本，影响更新性能",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeIndexConvention,
			Params:     nil,
		},
		Message:              "不建议创建冗余索引.",
		RawSQLHandler:        RuleSQLE00055,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00055): "在 TBase 中，不建议创建冗余索引."
您应遵循以下逻辑：
1. 对于 "CREATE INDEX ..."语句，提取索引名称和索引字段，连接到数据库，查询目标表的现有索引信息，对比提取出的索引字段与数据库中现有索索引的字段:
  1. 检查新增索引字段是否与现有索引字段完全相同(考虑索引顺序和组合), 则报告违反规则。
  2. 检查新增索引字段是否与现有复合索引的前缀相同（最左前缀规则）, 则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00055(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	isIndexFieldsEqual := func(newIndexCols, existIndexCols []string) bool {
		if len(newIndexCols) != len(existIndexCols) {
			return false
		}
		for i := range newIndexCols {
			if newIndexCols[i] != existIndexCols[i] {
				return false
			}
		}
		return true
	}

	isLeftPrefix := func(newIndexCols, existIndexCols []string) bool {
		if len(newIndexCols) > len(existIndexCols) {
			return false
		}
		for i := range newIndexCols {
			if newIndexCols[i] != existIndexCols[i] {
				return false
			}
		}
		return true
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_IndexStmt:
		// "CREATE INDEX ..."
		tableName := stmt.IndexStmt.GetRelation().GetRelname()
		schemaName := stmt.IndexStmt.GetRelation().GetSchemaname()

		newIndexColNames := []string{}
		for _, elt := range stmt.IndexStmt.GetIndexParams() {
			newIndexColNames = append(newIndexColNames, utilGetIndexColumnName(elt))
		}

		if len(newIndexColNames) == 0 {
			return "", nil
		}

		// 获取目标表的现有索引信息
		indexInfo, err := utilGetTableIndexInfo(ctx, schemaName, tableName)
		if err != nil {
			return "", fmt.Errorf("get table index info failed, sql: %v, error: %v", stmt, err)
		}

		existIndexColNames := [][]string{}
		for _, idx := range indexInfo {
			existIndexColNames = append(existIndexColNames, idx.ColumnList)
		}

		// 检查新增索引字段是否与现有索引字段完全相同
		for _, existCols := range existIndexColNames {
			if isIndexFieldsEqual(newIndexColNames, existCols) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}

		// 检查新增索引字段是否与现有复合索引的前缀相同
		for _, existCols := range existIndexColNames {
			if isLeftPrefix(newIndexColNames, existCols) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

// ==== Rule code end ====
