package inspector

import (
	"context"
	"fmt"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00112 = "SQLE00112"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00112,
			Desc:       "禁止WHERE子句中条件字段与值的数据类型不一致",
			Annotation: "WHERE子句中条件字段与值数据类型不一致会引发隐式数据类型转换，导致优化器选择错误的执行计划，在高并发、大数据量的情况下，不走索引会使得数据库的查询性能严重下降。",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeDMLConvention,
			Params:     nil,
		},
		Message:              "禁止WHERE子句中条件字段与值的数据类型不一致",
		RawSQLHandler:        RuleSQLE00112,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00112): "在 TBase 中，禁止WHERE子句中条件字段与值的数据类型不一致."
您应遵循以下逻辑：
1. 对于所有DML语句，解析SQL语句，获取所有的WHERE条件
  1. 如果左侧和右侧都是列字段，则登陆数据库，获取列字段的类型，检查左右的字段类型是否一致，若不一致，则报告违反规则；
  2. 如果左侧为列字段，右侧为常量，则登陆数据库，获取列字段的类型，检查右侧的常量类型是否与列字段类型一致，若不一致，则报告违反规则；
  3. 如果左侧为常量，右侧为列字段，则登陆数据库，获取列字段的类型，检查左侧的常量类型是否与列字段类型一致，若不一致，则报告违反规则；
  4. 如果左右都是常量，则检查左右的常量类型是否一致，若不一致，则报告违反规则；
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00112(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 定义别名信息
	var alias []*tableAliasInfo
	var defaultTable, defaultSchema string

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

	// 获取字段类型
	getValueType := func(val *parser.Node) string {
		if val == nil {
			return ""
		}
		valueType := ""
		if n, ok := val.GetNode().(*parser.Node_AConst); ok {
			if nil != n.AConst.GetIval() {
				valueType = "integer"
			} else if nil != n.AConst.GetSval() {
				valueType = "string"
			} else if nil != n.AConst.GetFval() {
				valueType = "float"
			} else if nil != n.AConst.GetBoolval() {
				valueType = "boolean"
			} else if nil != n.AConst.GetBsval() {
				valueType = "bytea"
			} else {
				valueType = ""
			}

		} else if n, ok := val.GetNode().(*parser.Node_FuncCall); ok {
			if len(n.FuncCall.GetFuncname()) == 1 {
				switch n.FuncCall.GetFuncname()[0].GetString_().Sval {
				case "to_date":
					valueType = "date"
				case "to_timestamp":
					valueType = "timestamp"
				default:
					valueType = ""
				}
			}
		} else if n, ok := val.GetNode().(*parser.Node_SqlvalueFunction); ok {
			switch n.SqlvalueFunction.GetOp() {
			case parser.SQLValueFunctionOp_SVFOP_CURRENT_DATE:
				valueType = "date"
			case parser.SQLValueFunctionOp_SVFOP_CURRENT_TIME:
				valueType = "time"
			case parser.SQLValueFunctionOp_SVFOP_CURRENT_TIMESTAMP:
				valueType = "timestamp"
			default:
				valueType = ""
			}
		}
		return valueType
	}

	convertColDataType := func(columnType string) string {
		realColumnType := ""
		switch columnType {
		case "smallint", "integer", "bigint":
			realColumnType = "integer"
		case "character", "character varying", "text":
			realColumnType = "string"
		case "real", "double_precision", "numeric":
			realColumnType = "float"
		case "time without time zone", "time with time zone":
			realColumnType = "time"
		case "timestamp without time zone", "timestamp with time zone":
			realColumnType = "timestamp"
		default:
			realColumnType = columnType
		}
		return realColumnType
	}

	// 获取字段的类型
	getFieldType := func(ctx context.Context, schemaName, tableName, columnName string) (string, error) {
		columnsInfo, err := utilGetTableColumnsInfo(ctx, schemaName, tableName)
		if err != nil {
			return "", errors.Wrap(err, "get table columns info")
		}
		for _, column := range columnsInfo {
			if column.ColumnName == columnName {
				return convertColDataType(column.ColumnType), nil
			}
		}
		return "", fmt.Errorf("column %s not found in table %s", columnName, tableName)
	}

	// 检查字段类型是否一致
	checkFieldTypes := func(ctx context.Context, left, right *parser.Node) (bool, error) {
		leftType := ""
		rightType := ""

		// 如果左侧是列字段
		if l, ok := left.Node.(*parser.Node_ColumnRef); ok {
			_, _, columnName := utilParseColumnRef(l.ColumnRef)
			schemaName, tableName := getTableName(l.ColumnRef)
			var err error
			leftType, err = getFieldType(ctx, schemaName, tableName, columnName)
			if err != nil {
				return false, err
			}
		} else if v := getValueType(left); "" != v {
			leftType = v
		}

		// 如果右侧是列字段
		if r, ok := right.Node.(*parser.Node_ColumnRef); ok {
			_, _, columnName := utilParseColumnRef(r.ColumnRef)
			schemaName, tableName := getTableName(r.ColumnRef)
			var err error
			rightType, err = getFieldType(ctx, schemaName, tableName, columnName)
			if err != nil {
				return false, err
			}
		} else if v := getValueType(right); "" != v {
			rightType = v
		}

		if leftType == "" || rightType == "" {
			return true, nil
		}

		return leftType == rightType, nil
	}

	// 检查 WHERE 子句中的所有条件，判断左右字段是否类型一致
	checkWhereClauseConsistent := func(ctx context.Context, whereClause *parser.Node) (consistent bool, err error) {
		consistent = true
		utilScanWhereStmt(func(expr *parser.Node) bool {
			switch n := expr.GetNode().(type) {
			case *parser.Node_AExpr:
				if n.AExpr != nil {
					left := n.AExpr.Lexpr
					right := n.AExpr.Rexpr
					if left != nil && right != nil {
						consistent, err = checkFieldTypes(ctx, left, right)
					}
				}
				return true // 停止递归
			case *parser.Node_BoolExpr:
				return false // 继续递归
			}
			return false
		}, whereClause)

		return consistent, err
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	switch node.GetStmt().Node.(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			// "select..."
			// 获取别名信息
			alias, defaultTable, defaultSchema = utilGetAliasInfoFromDML(&parser.Node{Node: &parser.Node_SelectStmt{SelectStmt: selectStmt}})
			if selectStmt.WhereClause != nil {
				consistent, err := checkWhereClauseConsistent(ctx, selectStmt.WhereClause)
				if err != nil {
					return "", err
				}
				if !consistent {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	}

	switch stmt := node.GetStmt().Node.(type) {
	case *parser.Node_DeleteStmt:
		// "delete..."
		// 获取别名信息
		alias, defaultTable, defaultSchema = utilGetAliasInfoFromDML(node.GetStmt())
		if stmt.DeleteStmt.WhereClause != nil {
			consistent, err := checkWhereClauseConsistent(ctx, stmt.DeleteStmt.WhereClause)
			if err != nil {
				return "", err
			}
			if !consistent {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_UpdateStmt:
		// "update..."
		// 获取别名信息
		alias, defaultTable, defaultSchema = utilGetAliasInfoFromDML(node.GetStmt())
		if stmt.UpdateStmt.WhereClause != nil {
			consistent, err := checkWhereClauseConsistent(ctx, stmt.UpdateStmt.WhereClause)
			if err != nil {
				return "", err
			}
			if !consistent {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
