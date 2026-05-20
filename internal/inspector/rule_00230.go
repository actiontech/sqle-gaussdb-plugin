package inspector

import (
	"context"
	"unicode"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00230 = "SQLE00230"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00230,
			Desc:       "禁止分片键包含中文",
			Annotation: "禁止分片键包含中文。包含中文的分片键会导致字符编码和排序的复杂性、查询效率的降低、扩展性和兼容性的限制以及潜在的安全风险。在实际应用中，应该选择那些具有高频性、高离散性和稳定性的字段作为分片键，如用户ID、订单号等数字或字符组合类型的字段。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeNamingConvention,
			Params:     nil,
		},
		Message:              "禁止分片键包含中文",
		RawSQLHandler:        RuleSQLE00230,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00230): "在 TBase 中，禁止分片键包含中文."
您应遵循以下逻辑：
1. 对于"INSERT ..." 语句
  1. 定义一个集合，把INSERT 对应插入的字段名以及对应的值以字段名：值，这样的方式存入集合。
  2. 登录数据库
  3. 使用INSERT 语句中的表名查找表的分片键。查询方式为：\d+ 表名，结果 shard(分片键)
  4. 使用上一步找到的分片键，在集合中查找对应的键值对，如果键值对应的值为中文，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00230(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查字段值是否包含中文
	containsChinese := func(values []string) bool {
		for _, value := range values {
			for _, r := range value {
				if unicode.Is(unicode.Scripts["Han"], r) {
					return true
				}
			}
		}
		return false
	}

	// 检查集合中的分片键值是否包含中文
	checkShardKeyViolation := func(table *parser.RangeVar, fieldValues map[string][]string) (bool, error) {
		// 获取表的分片键
		shardKey, err := utilGetTableDistributionColumn(ctx, table.Schemaname, table.Relname)
		if err != nil {
			return false, errors.Wrap(err, "get table distribution column")
		}

		// 检查分片键值是否包含中文
		if value, found := fieldValues[shardKey]; found {
			if containsChinese(value) {
				return true, nil // 违反规则
			}
		}
		return false, nil
	}

	// 处理 INSERT 语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_InsertStmt:
		insertStmt := stmt.InsertStmt
		fieldValues := map[string][]string{}
		// 获取插入的字段名和值
		if len(insertStmt.Cols) == 0 {
			columnsInfo, err := utilGetTableColumnsInfo(ctx, insertStmt.Relation.Schemaname, insertStmt.Relation.Relname)
			if err != nil {
				return "", errors.Wrap(err, "get table columns info")
			}
			for i, col := range columnsInfo {
				fieldName := col.ColumnName
				if selectStmt := insertStmt.SelectStmt.GetSelectStmt(); selectStmt != nil {
					if selectStmt := insertStmt.SelectStmt.GetSelectStmt(); selectStmt != nil {
						for _, list := range selectStmt.ValuesLists {
							list := list.GetList()
							if len(list.GetItems()) == len(columnsInfo) {
								if list.GetItems()[i].GetAConst() != nil && list.GetItems()[i].GetAConst().GetVal() != nil {
									if sval, ok := list.GetItems()[i].GetAConst().GetVal().(*parser.A_Const_Sval); ok {
										fieldValues[fieldName] = append(fieldValues[fieldName], sval.Sval.GetSval())
									}
								}
							} else {
							}
						}
					}
				}

			}
		} else {
			for i, col := range insertStmt.Cols {
				fieldName := ""
				if col.GetResTarget() != nil && col.GetResTarget().GetName() != "" {
					fieldName = col.GetResTarget().GetName()
				}

				if selectStmt := insertStmt.SelectStmt.GetSelectStmt(); selectStmt != nil {
					for _, list := range selectStmt.ValuesLists {
						list := list.GetList()
						if len(list.GetItems()) == len(insertStmt.Cols) {
							if list.GetItems()[i].GetAConst() != nil && list.GetItems()[i].GetAConst().GetVal() != nil {
								if sval, ok := list.GetItems()[i].GetAConst().GetVal().(*parser.A_Const_Sval); ok {
									fieldValues[fieldName] = append(fieldValues[fieldName], sval.Sval.GetSval())
								}
							}
						}
					}
				}
			}
		}

		// 检查分片键值是否包含中文
		if violation, err := checkShardKeyViolation(insertStmt.Relation, fieldValues); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
