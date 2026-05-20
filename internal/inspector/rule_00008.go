package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00008 = "SQLE00008"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00008,
			Desc:       "表里必须存在主键",
			Annotation: "表必须存在主键。如果表没有明确指定主键，可能会导致一些问题，如数据一致性难以保证、查询性能下降、数据完整性问题、数据管理和维护困难以及数据库优化受限等。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "表里必须存在主键",
		RawSQLHandler:        RuleSQLE00008,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00008): "在 TBase 中，表里必须存在主键."
您应遵循以下逻辑：
1. 对于"CREATE TABLE..." 语句，如果下面任意一项为真，则报告违反规则
  1. 不存在子句 PRIMARY KEY
	2. 且不是分区子表，即不包含关键词：partition of
2. 对于"ALTER TABLE ...DROP ..." 语句，如果下面任意一项为真，则报告违反规则
  1. 存在子句 PRIMARY KEY
==== Prompt end ====
*/

// ==== Rule code start ====
// Assuming all necessary parser structures and utilities are imported appropriately

// RuleSQLE00008 function to enforce primary key constraint rules on SQL statements
func RuleSQLE00008(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// "CREATE TABLE..." 语句
		createStmt := stmt.CreateStmt
		// 检查是否为分区子表
		if len(createStmt.InhRelations) > 0 {
			// 存在分区子表，不违反规则
			return "", nil
		}

		if !utilIsTableHasConstraint(createStmt.TableElts, parser.ConstrType_CONSTR_PRIMARY) {
			// 不存在 PRIMARY KEY 子句，违反规则
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterTableStmt:
		alterTableStmt := stmt.AlterTableStmt
		// "ALTER TABLE ... DROP CONSTRAINT ..." 语句
		for _, cmd := range utilGetAlterTableCommandsByTypes(alterTableStmt, parser.AlterTableType_AT_DropConstraint) {
			if ok, err := utilIsConstraintPrimaryKey(ctx, alterTableStmt.Relation.Schemaname, alterTableStmt.Relation.Relname, cmd.Name); err != nil {
				return "", errors.Wrap(err, "check primary key constraint")
			} else if ok {
				// 删除了主键约束，违反规则
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}

		// "ALTER TABLE ... DROP COLUMN ..." 语句
		for _, cmd := range utilGetAlterTableCommandsByTypes(alterTableStmt, parser.AlterTableType_AT_DropColumn) {
			info, err := utilGetTableColumnsInfo(ctx, alterTableStmt.Relation.Schemaname, alterTableStmt.Relation.Relname)
			if err != nil {
				return "", errors.Wrap(err, "get table columns info")
			}
			for _, col := range info {
				if col.ColumnName == cmd.Name {
					if col.IsPrimaryColumn {
						// 删除了主键列，违反规则
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}
		}
	}

	return "", nil
}

// ==== Rule code end ====
