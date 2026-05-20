package pg_query

type SelectStmtVisitor struct {
	SelectStmtList []*Node_SelectStmt
}

func (nc *SelectStmtVisitor) Enter(n IsNode_Node) (skipChildren bool) {
	selectStmt, ok := n.(*Node_SelectStmt)
	if ok {
		nc.SelectStmtList = append(nc.SelectStmtList, selectStmt)
	}
	return false
}

type FuncCallExprVisitor struct {
	FuncCallList []*Node_FuncCall
}

func (v *FuncCallExprVisitor) Enter(in IsNode_Node) (skipChildren bool) {
	switch stmt := in.(type) {
	case *Node_FuncCall:
		v.FuncCallList = append(v.FuncCallList, stmt)
	}
	return false
}
