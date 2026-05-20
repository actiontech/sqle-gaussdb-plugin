package inspector

import (
	"context"
	"fmt"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00015 = "SQLE00015"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00015,
			Desc:       "建议库内使用一致的数据库排序规则",
			Annotation: "在表结构定义过程中，如果定义了字段的排序规则，很有可能出现不同表的字段排序规则不一致，在多表关联过程中，为了正确能够正确的排序，导致数据库选择错误的执行计划，索引失效导致全表扫描、引发数据库查询性能低下。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "建议库内使用一致的数据库排序规则. 默认排序规则: %v",
		RawSQLHandler:        RuleSQLE00015,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00015): "在 TBase 中，建议库内使用一致的数据库排序规则."
您应遵循以下逻辑：
1. 对于 “CREATE TABLE ...” 语句，
  1. 创建一个集合，
  2. 登录数据库，查询当前数据库的默认排序规则（查看数据库的默认排序规则方法：使用\l 数据库名，查看结果中的Collate 结果，默认为C），并把排序规则写入到集合中,
  3. 如果语句中包含 collate 子句，把 collate 子句对应的排序规则与集合中的排序规则对比，如果不一样，则报告违反规则。
2. 对于 “ALTER TABLE ...” 语句，执行与上述同样检查。
3. 对于 “CREATE INDEX ...” 语句，执行与上述同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00015(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// get default collation
	defaultCollation, err := utilGetDefaultCollate(ctx)
	if err != nil {
		return "", err
	}

	var columnCollations []string

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// check collate in column definition
		for _, elt := range stmt.CreateStmt.TableElts {
			if c := utilGetColumnCollate(elt); c != "" {
				columnCollations = append(columnCollations, c)
			}
		}

	case *parser.Node_AlterTableStmt:

		// "alter table"

		// check collate in column definition
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddColumn, parser.AlterTableType_AT_AlterColumnType) {
			if c := utilGetColumnCollate(cmd.GetDef()); c != "" {
				columnCollations = append(columnCollations, c)
			}
		}

	case *parser.Node_IndexStmt:
		for _, param := range stmt.IndexStmt.IndexParams {
			if param.GetIndexElem() != nil {
				coll := param.GetIndexElem().GetCollation()
				if nil != coll && len(coll) > 0 && coll[0].GetString_() != nil {
					columnCollations = append(columnCollations, coll[0].GetString_().GetSval())
				}
			}
		}
	}

	for _, cs := range columnCollations {
		if !strings.EqualFold(cs, defaultCollation) {
			// the collate is not the same as param
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, defaultCollation), nil
		}
	}
	return "", nil
}

// ==== Rule code end ====
