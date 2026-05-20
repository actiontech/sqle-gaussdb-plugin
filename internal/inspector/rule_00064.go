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
	SQLE00064 = "SQLE00064"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00064,
			Desc:       "不建议索引字段是VARCHAR类型时其长度大于阈值",
			Annotation: "建立索引时没有限制索引的大小，索引长度会根据该字段实际存储的值来计算，VARCHAR 定义的长度越长，导致业务写入的内容越多，则建立的索引其存储大小将会越大。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeIndexConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "2000",
				Desc:  "VARCHAR类型的索引最大长度",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "不建议索引字段是VARCHAR类型时其长度大于阈值",
		RawSQLHandler:        RuleSQLE00064,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00064): "在 TBase 中，不建议索引字段是VARCHAR类型时其长度大于阈值.VARCHAR类型的索引最大长度:2000"
您应遵循以下逻辑：
1. 对于"CREATE INDEX..."语句，进行下述检查：
  1. 定义一个集合
  2. 将索引列放到集合中。
  3. 登录数据库，获取索引列的字段类型是varchar的定义长度，如果存在索索引列的varchar长度大于等于阈值，则报告违反规则。
2. 对于“ALTER TABLE ... ADD PRIMARY KEY ...” 语句，和 “ALTER TABLE ... ADD COLUMN ... PRIMARY KEY” 执行与上述同样检查。
3. 对于“CREATE TABLE ... PRIMAEY KEY...”语句，执行与上述同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00064(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取阈值
	threshold := utilGetRuleParamInt(rule, DefaultSingleParamKeyName)
	if threshold <= 0 {
		return "", fmt.Errorf("invalid threshold value, should be greater than 0")
	}

	// 检查索引列是否为 VARCHAR 类型且长度超过阈值
	checkIndexColumns := func(schemaName, tableName string, indexColNames []string) (bool, error) {
		// 获取表的列信息
		columnsInfo, err := utilGetTableColumnsInfo(ctx, schemaName, tableName)
		if err != nil {
			return false, errors.Wrap(err, "get table columns info failed")
		}

		for _, column := range columnsInfo {
			if utilIsStrInSlice(column.ColumnName, indexColNames) {
				if strings.EqualFold(column.ColumnType, "character varying") {
					if column.ColumnLength > threshold {
						return true, nil // 违反规则
					}
				}
			}
		}
		return false, nil
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_IndexStmt:
		// "CREATE INDEX ..."
		schemaName := stmt.IndexStmt.Relation.Schemaname
		tableName := stmt.IndexStmt.Relation.Relname

		indexColNames := []string{}
		for _, elt := range stmt.IndexStmt.IndexParams {
			indexColNames = append(indexColNames, utilGetIndexColumnName(elt))
		}

		if len(indexColNames) == 0 {
			return "", nil
		}

		if violation, err := checkIndexColumns(schemaName, tableName, indexColNames); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}

	case *parser.Node_CreateStmt:
		// check primary key in column definition
		for _, elt := range stmt.CreateStmt.TableElts {
			if utilIsColumnHasConstraint(elt, parser.ConstrType_CONSTR_PRIMARY) {

				// column type varchar
				if !utilIsColumnTypeEqual(elt, SqlTypeVarchar) {
					return "", nil
				}

				if utilGetColumnLength(elt) > threshold {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}

		// check primary key in table constraint
		constraint := utilGetTableConstraint(stmt.CreateStmt.TableElts, parser.ConstrType_CONSTR_PRIMARY)
		if nil != constraint {
			// this is a table primary key definition
			for _, key := range constraint.GetKeys() {
				for _, elt := range stmt.CreateStmt.TableElts {

					// this is primary key column
					if key.GetString_().GetSval() == utilGetColumnName(elt) {

						// column type varchar
						if !utilIsColumnTypeEqual(elt, SqlTypeVarchar) {
							return "", nil
						}

						if utilGetColumnLength(elt) > threshold {
							return RuleHandlerMap[rule.Name].Message, nil
						}
					}
				}
			}
		}

	case *parser.Node_AlterTableStmt:
		// ALTER TABLE ... ADD COLUMN ... PRIMARY KEY;
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddColumn) {

			// check primary key in column definition
			if utilIsColumnHasConstraint(cmd.GetDef(), parser.ConstrType_CONSTR_PRIMARY) {

				// column type varchar
				if !utilIsColumnTypeEqual(cmd.GetDef(), SqlTypeVarchar) {
					return "", nil
				}

				if utilGetColumnLength(cmd.GetDef()) > threshold {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}

		// "ALTER TABLE ... ADD PRIMARY KEY ..."
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddConstraint) {
			constraint := utilGetTableConstraint([]*parser.Node{cmd.GetDef()}, parser.ConstrType_CONSTR_PRIMARY)
			if nil != constraint {
				primaryKeys := []string{}
				for _, key := range constraint.GetKeys() {
					primaryKeys = append(primaryKeys, key.GetString_().GetSval())
				}

				// ALTER TABLE test
				// ADD COLUMN id VARCHAR(2001),
				// ADD PRIMARY KEY (id);
				for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddColumn) {

					// this is primary key column
					if utilIsStrInSlice(utilGetColumnName(cmd.GetDef()), primaryKeys) {

						// column type varchar
						if !utilIsColumnTypeEqual(cmd.GetDef(), SqlTypeVarchar) {
							return "", nil
						}

						if utilGetColumnLength(cmd.GetDef()) > threshold {
							return RuleHandlerMap[rule.Name].Message, nil
						}
					}

				}

				// "ALTER TABLE exist_tb_2 ADD PRIMARY KEY (c1);"
				if violation, err := checkIndexColumns(stmt.AlterTableStmt.Relation.Schemaname, stmt.AlterTableStmt.Relation.Relname, primaryKeys); err != nil {
					return "", err
				} else if violation {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
