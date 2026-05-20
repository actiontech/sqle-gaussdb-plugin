package inspector

import parser "actiontech.cloud/sqle/pg_query_go/v5"

type constraintExtractor struct {
	ConstraintList []*parser.Constraint
}

func (ce *constraintExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_Constraint:
		ce.ConstraintList = append(ce.ConstraintList, node.Constraint)
	}
	return false
}

// rangeVarExtractor implements pg_query.Visitor interface.
type rangeVarExtractor struct {
	RangeVars []*parser.RangeVar
}

func (e *rangeVarExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_RangeVar:
		e.RangeVars = append(e.RangeVars, node.RangeVar)
	}
	return false
}

type selectStmtExtractor struct {
	SelectStmts []*parser.SelectStmt
}

func (se *selectStmtExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_SelectStmt:
		if node.SelectStmt.Op == parser.SetOperation_SETOP_UNION {
			return false
		}
		if node.SelectStmt.Op == parser.SetOperation_SETOP_EXCEPT {
			return false
		}
		if node.SelectStmt.Op == parser.SetOperation_SETOP_INTERSECT {
			return false
		}
		se.SelectStmts = append(se.SelectStmts, node.SelectStmt)
	}
	return false
}

type columnRefExtractor struct {
	columnRef []*parser.ColumnRef
}

func (se *columnRefExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_ColumnRef:
		se.columnRef = append(se.columnRef, node.ColumnRef)
	}
	return false
}

type funcCallExtractor struct {
	funcs []*parser.FuncCall
}

func (f *funcCallExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_FuncCall:
		f.funcs = append(f.funcs, node.FuncCall)
	}
	return false
}

type commonTableExprExtractor struct {
	exprs []*parser.CommonTableExpr
}

func (f *commonTableExprExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_CommonTableExpr:
		f.exprs = append(f.exprs, node.CommonTableExpr)
	}
	return false
}

type joinExprExtractor struct {
	JoinExprs []*parser.JoinExpr
}

func (f *joinExprExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_JoinExpr:
		f.JoinExprs = append(f.JoinExprs, node.JoinExpr)
	}
	return false
}

type subLinkExtractor struct {
	SubLinks []*parser.SubLink
}

func (f *subLinkExtractor) Enter(in parser.IsNode_Node) (skipChildren bool) {
	switch node := in.(type) {
	case *parser.Node_SubLink:
		f.SubLinks = append(f.SubLinks, node.SubLink)
	}
	return false
}
