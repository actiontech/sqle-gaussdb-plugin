package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00140 = "SQLE00140"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00140,
			Desc:       "建议对表、视图等对象进行操作时指定schema或者库名",
			Annotation: "对表、视图等对象进行创建、修改、查询、更新、删除等DDL、DML操作时，如未指定schema或者库名，会导致在不确定的schema或者数据库下执行，与实际业务预期不符合，而且会导致SQL语句执行错误。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "建议对表、视图等对象进行操作时指定schema或者库名",
		RawSQLHandler:        RuleSQLE00140,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00140): "在 TBase 中，建议对表、视图等对象进行操作时指定schema或者库名."
您应遵循以下逻辑：
1. 对所有SQL语句进行检查，包括但不限于以下类型：
    - DDL语句：CREATE、ALTER、DROP等。
    - DML语句：SELECT、INSERT、UPDATE、DELETE等。
    - 存储过程调用：CALL。
2. 对每条SQL语句进行解析，检查是否指定了库名（schema）。
3. 如果发现任意SQL语句中存在未指定数据库的表、视图、存储过程、函数、序列、类型、索引、物化视图，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00140(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取CTE表
	var cteTableNames []string
	for _, expr := range utilGetCommonTableExpr(node.GetStmt()) {
		cteTableNames = append(cteTableNames, expr.Ctename)
	}

	// 检查表名是否指定了 schema
	checkSchemaViolation := func(tables []*parser.RangeVar) bool {
		for _, table := range tables {
			// 如果表名未指定 schema 且不是 CTE 表，则违反规则
			if table.Schemaname == "" && !utilIsStrInSlice(table.Relname, cteTableNames) {
				return true // 违反规则
			}
		}
		return false
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

	// 主逻辑：根据不同的 SQL 类型执行相应的检查

	// 先检查所有DML中的 SELECT 子句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if checkSchemaViolation(getSelectTables(selectStmt)) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	// 再检查 INSERT、UPDATE、DELETE、ALTER TABLE、CREATE TABLE 等语句
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_InsertStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.InsertStmt.Relation}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_UpdateStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.UpdateStmt.Relation}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DeleteStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.DeleteStmt.Relation}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterTableStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.AlterTableStmt.Relation}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterSeqStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.AlterSeqStmt.GetSequence()}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_IndexStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.IndexStmt.Relation}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateStmt:
		// 检查是否为分区子表
		if len(stmt.CreateStmt.InhRelations) > 0 {
			// 存在分区子表，不违反规则
			return "", nil
		}
		if checkSchemaViolation([]*parser.RangeVar{stmt.CreateStmt.Relation}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateSeqStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.CreateSeqStmt.GetSequence()}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CompositeTypeStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.CompositeTypeStmt.GetTypevar()}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateTableAsStmt:
		asQuery := stmt.CreateTableAsStmt.GetQuery()
		if asQuery != nil {
			for _, selectStmt := range utilGetSelectStmt(asQuery) {
				if checkSchemaViolation(getSelectTables(selectStmt)) {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}

		if checkSchemaViolation([]*parser.RangeVar{stmt.CreateTableAsStmt.Into.GetRel()}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CallStmt:
		if len(stmt.CallStmt.GetFunccall().GetFuncname()) == 1 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateFunctionStmt:
		if len(stmt.CreateFunctionStmt.GetFuncname()) == 1 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_ViewStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.ViewStmt.GetView()}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_RefreshMatViewStmt:
		if checkSchemaViolation([]*parser.RangeVar{stmt.RefreshMatViewStmt.GetRelation()}) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DropStmt:
		if stmt.DropStmt.RemoveType == parser.ObjectType_OBJECT_TABLE || stmt.DropStmt.RemoveType == parser.ObjectType_OBJECT_MATVIEW || stmt.DropStmt.RemoveType == parser.ObjectType_OBJECT_SEQUENCE || stmt.DropStmt.RemoveType == parser.ObjectType_OBJECT_INDEX {
			for _, o := range stmt.DropStmt.GetObjects() {
				if list, ok := o.Node.(*parser.Node_List); ok {
					if len(list.List.GetItems()) == 1 {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}
		}
		if stmt.DropStmt.RemoveType == parser.ObjectType_OBJECT_FUNCTION {
			for _, o := range stmt.DropStmt.GetObjects() {
				if objArgs, ok := o.Node.(*parser.Node_ObjectWithArgs); ok {
					if len(objArgs.ObjectWithArgs.GetObjname()) == 1 {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}

		}
		if stmt.DropStmt.RemoveType == parser.ObjectType_OBJECT_TYPE {
			for _, o := range stmt.DropStmt.GetObjects() {
				if typeName, ok := o.Node.(*parser.Node_TypeName); ok {
					if len(typeName.TypeName.GetNames()) == 1 {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
