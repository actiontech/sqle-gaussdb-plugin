package inspector

import (
	"context"
	"fmt"
	"log"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00039 = "SQLE00039"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00039,
			Desc:       "建议使用数据区分度高的索引字段",
			Annotation: " 为了提高查询效率，建议在执行SQL时优先使用区分度高的索引字段。区分度高的索引可以更快地定位数据，减少不必要的数据扫描，从而加速查询响应时间。规则检查将会计算候选索引字段的区分度，如果索引的区分度低于设定的阈值，则建议调整索引策略。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeIndexConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "0.7",
				Desc:  "selectivity",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "建议使用数据区分度高的索引字段. 字段 %v 的区分度为 %v",
		RawSQLHandler:        RuleSQLE00039,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00039): "在 TBase 中，建议使用数据区分度高的索引字段.selectivity:0.7"
您应遵循以下逻辑：
1. 对于CREATE INDEX...语句，
    1. 定义一个集合A
    2. 将本次新增索引涉及的字段放到集合A中
    3. 计算集合A中的每个字段的区分度，如果区分度小于规则变量selectivity值，则报告违反规则。
      区分度计算需要在线进行，公式： select 1- (
        Select count(*) as record_count from (select column_name from table_name limit 50000) group by column_name order by record_count desc limit 1
        )/ (select count(*) from table_name limit 50000);
1. 对于所有DML语句中的SELECT子句，如果where 条件不是恒为真，
    1. 定义一个集合A
    2. 将WHERE子句中用到的条件字段放到集合A中
    3. 连接数据库，根据集合A的字段，将存在索引的字段放到集合B中
    3. 计算集合B中的每个字段的区分度，如果区分度小于规则变量selectivity值，则报告违反规则。
    区分度计算需要在线进行，公式： select 1- (
          Select count(*) as record_count from (select column_name from table_name limit 50000) group by column_name order by record_count desc limit 1
          )/ (select count(*) from table_name limit 50000);

1. 对于UNION...语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00039(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// get expected selectivity field name in config
	threshold := rule.Params.GetParam(DefaultSingleParamKeyName).Float64()

	if threshold > 1 || threshold <= 0 {
		return "", fmt.Errorf("param should be in range (0, 1]")
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_IndexStmt:
		// "CREATE INDEX ..."
		tableName := stmt.IndexStmt.GetRelation().GetRelname()
		schemaName := stmt.IndexStmt.GetRelation().GetSchemaname()

		indexColumns := []string{}
		for _, elt := range stmt.IndexStmt.GetIndexParams() {
			indexColumns = append(indexColumns, utilGetIndexColumnName(elt))
		}

		if len(indexColumns) == 0 {
			return "", nil
		}

		indexColumns = RemoveDuplicates(indexColumns)

		discrimination, err := utilCalculateIndexDiscrimination(ctx, schemaName, tableName, indexColumns)
		if err != nil {
			return "", fmt.Errorf("get index discrimination failed, sql: %v, error: %v", stmt, err)
		}

		for col, d := range discrimination {
			if d < threshold {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, col, d), nil
			}
		}

	case *parser.Node_SelectStmt, *parser.Node_UpdateStmt, *parser.Node_DeleteStmt, *parser.Node_InsertStmt:
		// "SELECT ...", "UPDATE ...", "DELETE ...", "INSERT ..."
		var defaultTable string
		var defaultSchema string
		var alias []*tableAliasInfo
		getTableName := func(col *parser.ColumnRef) (schemaName, tableName string) {
			schemaName, tableName, _ = utilParseColumnRef(col)
			if tableName != "" {
				for _, a := range alias {
					if a.TableAliasName == tableName {
						return "", a.TableName
					}
				}
			}

			return defaultSchema, defaultTable
		}

		selectStmts := utilGetSelectStmt(node.GetStmt())

		for _, selectStmt := range selectStmts {
			// select...

			if t := utilGetDefaultTable(selectStmt); t != nil {
				defaultTable = t.GetRelname()
				defaultSchema = t.GetSchemaname()
			} else {
				defaultTable = ""
				defaultSchema = ""
			}

			for _, from := range selectStmt.FromClause {
				alias = append(alias, utilGetTableAliasInfoFromNode(from)...)
			}

			table2cols := map[string] /*table name*/ []*parser.ColumnRef /*col names*/ {}
			for _, col := range utilGetColumnRefFromNodes(selectStmt.WhereClause) {
				schemaName, tableName := getTableName(col)
				table2cols[strings.Join([]string{schemaName, tableName}, ".")] = append(table2cols[tableName], col)
			}

			for table, colRefs := range table2cols {
				schemaName := strings.Split(table, ".")[0]
				tableName := strings.Split(table, ".")[1]
				indexesInfo, err := utilGetTableIndexInfo(ctx, schemaName, tableName)
				if err != nil {
					return "", fmt.Errorf("get table index info failed, sql: %v, error: %v", stmt, err)
				}
				// get index columns
				indexColumns := make([]string, 0)
				for _, colRef := range colRefs {

					for _, index := range indexesInfo {
						// check if the column is index
						for _, col := range index.ColumnList {
							_, _, c := utilParseColumnRef(colRef)
							if c == col {
								indexColumns = append(indexColumns, c)
							}
						}
					}
				}

				if len(indexColumns) == 0 {
					// no index
					continue
				}

				indexColumns = RemoveDuplicates(indexColumns)

				discrimination, err := utilCalculateIndexDiscrimination(ctx, schemaName, tableName, indexColumns)
				if err != nil {
					log.Printf("get index discrimination failed, sql: %v, error: %v", stmt, err)
					return "", nil
				}

				for col, d := range discrimination {
					if d < threshold {
						return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, col, d), nil
					}
				}
			}
		}

	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
