package utils

import (
	"fmt"
	"strconv"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
)

// ExtractSubQueries :解析出所有子查询
func ExtractSubQueries(node *parser.Node) (subQueries *[]*parser.SelectStmt) {
	subQueries = new([]*parser.SelectStmt)
	if node == nil || node.Node == nil {
		return
	}
	switch n := node.Node.(type) {
	case *parser.Node_SubLink:
		if n.SubLink.Subselect != nil {
			stmt, ok := n.SubLink.Subselect.Node.(*parser.Node_SelectStmt)
			if ok && stmt.SelectStmt != nil {
				if *subQueries == nil {
					*subQueries = make([]*parser.SelectStmt, 0)
				}
				*subQueries = append(*subQueries, stmt.SelectStmt)
				selectStmt := stmt.SelectStmt
				*subQueries = append(*subQueries, *RecursiveSubQuery(selectStmt)...)
				*subQueries = append(*subQueries, *handleLSelectStmtArgAndRArg(selectStmt)...)
			}
		}
	case *parser.Node_SelectStmt:
		*subQueries = append(*subQueries, n.SelectStmt)
		selectStmt := n.SelectStmt
		*subQueries = append(*subQueries, *RecursiveSubQuery(selectStmt)...)
		*subQueries = append(*subQueries, *handleLSelectStmtArgAndRArg(selectStmt)...)
	case *parser.Node_RangeSubselect:
		if n.RangeSubselect != nil && n.RangeSubselect.Subquery != nil {
			stmt, ok := n.RangeSubselect.Subquery.Node.(*parser.Node_SelectStmt)
			if ok && stmt.SelectStmt != nil {
				selectStmt := stmt.SelectStmt
				*subQueries = append(*subQueries, selectStmt)
				*subQueries = append(*subQueries, *RecursiveSubQuery(selectStmt)...)
				*subQueries = append(*subQueries, *handleLSelectStmtArgAndRArg(selectStmt)...)
			}
		}
	}
	return
}

// RecursiveSubQuery :递归调用子查询
func RecursiveSubQuery(selectStmt *parser.SelectStmt) (subQueries *[]*parser.SelectStmt) {
	subQueries = new([]*parser.SelectStmt)
	fromClause := selectStmt.FromClause
	for _, from := range fromClause {
		*subQueries = append(*subQueries, *ExtractSubQueries(from)...)
	}
	whereClause := selectStmt.WhereClause
	if whereClause != nil {
		*subQueries = append(*subQueries, *ExtractSubQueries(whereClause)...)
	}
	if whereClause != nil && whereClause.GetAExpr() != nil && whereClause.GetAExpr().GetRexpr() != nil {
		*subQueries = append(*subQueries, *ExtractSubQueries(whereClause.GetAExpr().GetRexpr())...)
	}
	if whereClause != nil && whereClause.GetBoolExpr() != nil && len(whereClause.GetBoolExpr().GetArgs()) > 0 {
		args := whereClause.GetBoolExpr().GetArgs()
		for _, arg := range args {
			*subQueries = append(*subQueries, *ExtractSubQueries(arg)...)
		}
	}
	targetList := selectStmt.TargetList
	for _, list := range targetList {
		if list.GetResTarget() != nil && list.GetResTarget().GetVal() != nil {
			*subQueries = append(*subQueries, *ExtractSubQueries(list.GetResTarget().GetVal())...)
		}
	}
	*subQueries = append(*subQueries, *ExtractSubQueries(selectStmt.HavingClause)...)
	if selectStmt.HavingClause != nil {
		aExpr := selectStmt.HavingClause.GetAExpr()
		if aExpr != nil && aExpr.GetLexpr() != nil {
			*subQueries = append(*subQueries, *ExtractSubQueries(aExpr.GetLexpr())...)
		}
		if aExpr != nil && aExpr.GetRexpr() != nil {
			*subQueries = append(*subQueries, *ExtractSubQueries(aExpr.GetRexpr())...)
		}
	}
	windowClause := selectStmt.WindowClause
	for _, window := range windowClause {
		*subQueries = append(*subQueries, *ExtractSubQueries(window)...)
	}
	valuesLists := selectStmt.ValuesLists
	for _, list := range valuesLists {
		*subQueries = append(*subQueries, *ExtractSubQueries(list)...)
	}
	withClause := selectStmt.WithClause
	if withClause != nil {
		*subQueries = append(*subQueries, *HandleWithClause(withClause)...)
	}
	return
}

// HandleWithClause :处理with临时表
func HandleWithClause(withClause *parser.WithClause) (subQueries *[]*parser.SelectStmt) {
	subQueries = new([]*parser.SelectStmt)
	if withClause != nil {
		clauses := withClause.Ctes
		for _, clause := range clauses {
			if clause.GetCommonTableExpr() == nil {
				continue
			}
			*subQueries = append(*subQueries, *ExtractSubQueries(clause.GetCommonTableExpr().GetCtequery())...)
			// 子查询
			if clause.GetCommonTableExpr().GetCtequery() == nil {
				continue
			}
			if clause.GetCommonTableExpr().GetCtequery().GetSelectStmt() == nil {
				continue
			}
			selectStmt := clause.GetCommonTableExpr().GetCtequery().GetSelectStmt()
			fromClause := selectStmt.FromClause
			for _, from := range fromClause {
				*subQueries = append(*subQueries, *ExtractSubQueries(from)...)
			}
			targetList := selectStmt.GetTargetList()
			for _, list := range targetList {
				if list.GetResTarget() != nil {
					*subQueries = append(*subQueries, *ExtractSubQueries(list.GetResTarget().GetVal())...)
				}
			}
			*subQueries = append(*subQueries, *ExtractSubQueries(selectStmt.GetWhereClause())...)
			if selectStmt.GetWhereClause() == nil {
				continue
			}
			if selectStmt.GetWhereClause().GetBoolExpr() != nil || selectStmt.GetWhereClause().GetAExpr() != nil {
				*subQueries = append(*subQueries, selectStmt)
			}
		}
	}
	return
}

// handleLSelectStmtArgAndRArg :处理SelectStmt的lArg And RArg
func handleLSelectStmtArgAndRArg(selectStmt *parser.SelectStmt) (subQueries *[]*parser.SelectStmt) {
	subQueries = new([]*parser.SelectStmt)
	if selectStmt != nil {
		if lArg := selectStmt.GetLarg(); lArg != nil {
			*subQueries = append(*subQueries, *RecursiveSubQuery(lArg)...)
		}
		if rArg := selectStmt.GetRarg(); rArg != nil {
			*subQueries = append(*subQueries, *RecursiveSubQuery(rArg)...)
		}
	}
	return
}

// GetTableIndexColumns :获取表索引对应的列，多个列英文逗号分割
func GetTableIndexColumns(e *executor.Executor, schemaName, tableName string) ([]string, error) {
	indexColumns := make([]string, 0)
	if e == nil {
		return indexColumns, nil
	}
	indexesInfo, err := e.GetTableIndexesInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}

	for _, info := range indexesInfo {
		start := strings.LastIndex(info.IndexDDL, "(")
		end := strings.LastIndex(info.IndexDDL, ")")
		indexColumns = append(indexColumns, info.IndexDDL[start+1:end])
	}
	return indexColumns, nil
}

// GetTableColumns :获取表的所有列
func GetTableColumns(e *executor.Executor, schemaName, tableName string) ([]string, error) {
	columns := make([]string, 0)
	if e == nil {
		return columns, nil
	}
	columnsInfo, err := e.GetTableColumnsInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}

	for _, column := range columnsInfo {
		columns = append(columns, strings.ToLower(column.ColumnName))
	}
	return columns, nil
}

// ParseCondition :解析条件
func ParseCondition(expr *parser.Node) string {
	switch n := expr.Node.(type) {
	case *parser.Node_BoolExpr:
		return ParseBoolExpr(n.BoolExpr)
	case *parser.Node_AExpr:
		return ParseAExpr(n.AExpr)
	case *parser.Node_ColumnRef:
		return ParseColumnRef(n.ColumnRef)
	case *parser.Node_AConst:
		return ParseAConst(n.AConst)
	case *parser.Node_SqlvalueFunction:
		return ParseSqlValueFunction(n.SqlvalueFunction)
	case *parser.Node_FuncCall:
		return ParserFuncCall(n.FuncCall)
	default:
		return ""
	}
}

// ParseBoolExpr :解析布尔表达式
func ParseBoolExpr(expr *parser.BoolExpr) string {
	boolOp := expr.Boolop
	clauses := expr.Args
	condition := ""
	for i, clause := range clauses {
		if i != 0 {
			condition += fmt.Sprintf(" %s ", strings.Replace(boolOp.String(), "_EXPR", "", -1))
		}
		condition += ParseCondition(clause)
	}
	return fmt.Sprintf("(%s)", condition)
}

// ParseAExpr :解析任意表达式
func ParseAExpr(expr *parser.A_Expr) string {
	left := ParseCondition(expr.Lexpr)
	right := ParseCondition(expr.Rexpr)
	switch expr.Kind {
	case parser.A_Expr_Kind_AEXPR_OP:
		return fmt.Sprintf("%s %s %s", left, expr.Name[0].GetString_().GetSval(), right)
	default:
		return ""
	}
}

// ParseColumnRef :解析出列名
func ParseColumnRef(expr *parser.ColumnRef) string {
	column := ""
	for _, field := range expr.Fields {
		column += field.GetString_().GetSval()
	}
	return column
}

// ParseAConst :解析出常量值
func ParseAConst(expr *parser.A_Const) string {
	switch expr.Val.(type) {
	case *parser.A_Const_Ival:
		return fmt.Sprintf("%d", expr.GetIval().GetIval())
	case *parser.A_Const_Fval:
		return fmt.Sprintf("%s", expr.GetFval().GetFval())
	case *parser.A_Const_Sval:
		return fmt.Sprintf("'%s'", expr.GetSval().GetSval())
	default:
		return ""
	}
}

// ParseSqlValueFunction :解析函数值
func ParseSqlValueFunction(expr *parser.SQLValueFunction) string {
	switch expr.Op {
	case parser.SQLValueFunctionOp_SVFOP_CURRENT_DATE:
		return "CURRENT_DATE"
	default:
		return ""
	}
}

// ParserFuncCall :解析函数
func ParserFuncCall(expr *parser.FuncCall) string {
	funcName := ""
	if len(expr.Funcname) > 0 {
		funcName = expr.Funcname[0].GetString_().GetSval()
	}
	parameters := make([]string, 0)
	// 递归处理参数
	for _, arg := range expr.Args {
		switch arg := arg.Node.(type) {
		case *parser.Node_String_:
			parameters = append(parameters, fmt.Sprintf("'%s'", arg.String_.GetSval()))
		case *parser.Node_Integer:
			parameters = append(parameters, strconv.Itoa(int(arg.Integer.Ival)))
		case *parser.Node_Float:
			parameters = append(parameters, arg.Float.GetFval())
		case *parser.Node_FuncCall:
			parameters = append(parameters, ParserFuncCall(arg.FuncCall))
		case *parser.Node_AConst:
			parameters = append(parameters, ParseAConst(arg.AConst))
		default:
			parameters = append(parameters, "")
		}
	}
	result := ""
	if funcName != "" {
		result = fmt.Sprintf("%s(%s)", funcName, strings.Join(parameters, ","))
	} else {
		result = strings.Join(parameters, ",")
	}
	return result
}
