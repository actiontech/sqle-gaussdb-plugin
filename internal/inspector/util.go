package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

const DefaultSingleParamKeyName = "first_key" // For most of the rules, it is just has one param, this is first params.

// a helper function to get the PostgreSQL column name
func utilGetColumnName(columnDefNode *parser.Node) string {
	def := columnDefNode.GetColumnDef()
	if nil != def {
		return def.Colname
	}
	return ""
}

// a helper function to parse the PostgreSQL column reference
func utilParseColumnRef(columnRef *parser.ColumnRef) (schemaName, tableName, columnName string) {
	if columnRef == nil {
		return
	}

	if columnRef.Fields == nil {
		return
	}

	if len(columnRef.Fields) == 0 {
		return
	}

	if len(columnRef.Fields) == 3 {
		schemaName = columnRef.Fields[0].GetString_().GetSval()
		tableName = columnRef.Fields[1].GetString_().GetSval()
		columnName = columnRef.Fields[2].GetString_().GetSval()
	} else if len(columnRef.Fields) == 2 {
		tableName = columnRef.Fields[0].GetString_().GetSval()
		columnName = columnRef.Fields[1].GetString_().GetSval()
	} else {
		columnName = columnRef.Fields[0].GetString_().GetSval()
	}

	return
}

// a helper function to get the length of a PostgreSQL column in definition
func utilGetColumnLength(columnDefNode *parser.Node) int {
	columnDef := columnDefNode.GetColumnDef()
	if nil == columnDef {
		return 0
	}

	typeName := columnDef.GetTypeName()
	if nil == typeName {
		return 0
	}

	typmods := typeName.GetTypmods()
	if nil == typmods || len(typmods) < 1 {
		return 0
	}

	aConst := typmods[0].GetAConst()
	if nil == aConst {
		return 0
	}

	return int(aConst.GetIval().GetIval())
}

// a helper function to get the PostgreSQL column constraint whose type is the _targetConstraint_ (NOT NULL/PRIMARY/...)
func utilGetColumnConstraint(columnDefNode *parser.Node, targetConstraint parser.ConstrType) *parser.Constraint {
	columnDef := columnDefNode.GetColumnDef()
	if nil == columnDef {
		return nil
	}

	for _, constraint := range columnDef.Constraints {
		c := constraint.GetConstraint()
		if c != nil && c.Contype == targetConstraint {
			return c
		}
	}
	return nil
}

// a helper function to check if a PostgreSQL column is has constraint _targetConstraint_ or not
func utilIsColumnHasConstraint(columnDefNode *parser.Node, targetConstraint parser.ConstrType) bool {
	columnDef := columnDefNode.GetColumnDef()
	if nil == columnDef {
		return false
	}

	for _, constraint := range columnDef.Constraints {
		c := constraint.GetConstraint()
		if c != nil && c.Contype == targetConstraint {
			return true
		}
	}
	return false
}

// a helper function to get if PostgreSQL column collate clause
func utilGetColumnCollate(columnDefNode *parser.Node) string {
	columnDef := columnDefNode.GetColumnDef()
	if nil == columnDef {
		return ""
	}

	coll := columnDef.GetCollClause()
	if nil == coll {
		return ""
	}

	if len(coll.GetCollname()) < 1 {
		return ""
	}

	return coll.GetCollname()[0].GetString_().GetSval()
}

// a helper function to get if PostgreSQL column has the target collate clause
func utilIsColumnHasTargetCollate(columnDefNode *parser.Node, targetCollate string) bool {
	columnDef := columnDefNode.GetColumnDef()
	if nil == columnDef {
		return false
	}

	coll := columnDef.GetCollClause()
	if nil == coll {
		return false
	}

	if len(coll.GetCollname()) < 1 {
		return false
	}

	return strings.EqualFold(coll.GetCollname()[0].GetString_().GetSval(), targetCollate)
}

// a helper function to get the PostgreSQL table collate
func utilGetTableCollate(nodes []*parser.Node) string {
	for _, node := range nodes {
		collate := node.GetCollateClause()
		if nil == collate {
			continue
		}

		if len(collate.GetCollname()) < 1 {
			continue
		}

		return collate.GetCollname()[0].GetString_().GetSval()
	}
	return ""
}

// a helper function to check if a PostgreSQL table has constraint _targetConstraint_ or not
func utilIsTableHasConstraint(nodes []*parser.Node, targetConstraint parser.ConstrType) bool {
	for _, node := range nodes {
		c := new(constraintExtractor)
		node.GetNode().Accept(c)
		for _, constraint := range c.ConstraintList {
			if constraint.Contype == targetConstraint {
				return true
			}
		}
	}
	return false
}

// a helper function to get the PostgreSQL table constraint whose type is the _targetConstraint_ (PRIMARY/FOREIGN/...)
func utilGetTableConstraint(nodes []*parser.Node, targetConstraint parser.ConstrType) *parser.Constraint {
	for _, node := range nodes {
		constraint := node.GetConstraint()
		if nil != constraint && constraint.Contype == targetConstraint {
			return constraint
		}
	}
	return nil
}

// a helper function to get the PostgreSQL serial column types
func utilGetSerialTypes() []ColumnType {
	return []ColumnType{
		SqlTypeSerial,
		SqlTypeBigSerial,
	}
}

// a helper function to get the PostgreSQL float column types
func utilGetFloatTypes() []ColumnType {
	return []ColumnType{
		SqlTypeFloat4,
		SqlTypeFloat8,
	}
}

// a helper function to get the PostgreSQL time column types
func utilGetTimeTypes() []ColumnType {
	return []ColumnType{
		SqlTypeTimestamp,
		SqlTypeTimestampTZ,
		SqlTypeDate,
		SqlTypeTime,
		SqlTypeTimeTZ,
	}
}

type ColumnType string

const (
	SqlTypeChar        ColumnType = "bpchar"
	SqlTypeVarchar     ColumnType = "varchar"
	SqlTypeText        ColumnType = "text"
	SqlTypeFloat4      ColumnType = "float4"
	SqlTypeFloat8      ColumnType = "float8"
	SqlTypeTimestamp   ColumnType = "timestamp"
	SqlTypeTimestampTZ ColumnType = "timestamptz"
	SqlTypeDate        ColumnType = "date"
	SqlTypeTime        ColumnType = "time"
	SqlTypeTimeTZ      ColumnType = "timetz"
	SqlTypeSerial      ColumnType = "serial"
	SqlTypeBigSerial   ColumnType = "bigserial"
	SqlTypeNumeric     ColumnType = "numeric"
	SqlTypeBytea       ColumnType = "bytea"
	SqlTypeInt4        ColumnType = "int4" // int
	SqlTypeInt8        ColumnType = "int8" // bigint
	SqlTypeInt2        ColumnType = "int2" // smallint
	// more types can be added here
)

// a helper function to get the PostgreSQL column type
func utilGetColumnType(ts ColumnType) string {
	return string(ts)
}

// a helper function to check if a PostgreSQL column is of type _targetType_ (varchar/integer/...) or not
func utilIsColumnTypeEqual(columnDefNode *parser.Node, targetType ...ColumnType) bool {
	colDef := columnDefNode.GetColumnDef()
	if colDef == nil {
		return false
	}
	for _, n := range colDef.GetTypeName().GetNames() {
		for _, target := range targetType {
			if n.GetString_().GetSval() == string(target) {
				return true
			}
		}
	}
	return false
}

// a helper function to get the PostgreSQL index column name
func utilGetIndexColumnName(indexParam *parser.Node) string {
	if indexParam.GetIndexElem() == nil {
		return ""
	}
	return indexParam.GetIndexElem().GetName()
}

// a helper function to check if a PostgreSQL column constraints is a NULL constraint
func utilIsConstraintValIsNull(constraint *parser.Constraint) bool {
	aConst := constraint.RawExpr.GetAConst()
	if nil == aConst {
		return false
	}
	return aConst.GetIsnull()
}

// a helper function to get the PostgreSQL func name in column constraints
func utilGetColumnConstraintFuncCall(columnDefNode *parser.Node) (funcName string) {
	colDef := columnDefNode.GetColumnDef()
	if colDef == nil {
		return ""
	}
	for _, c := range colDef.GetConstraints() {
		if c.GetConstraint() == nil {
			continue
		}
		if c.GetConstraint().GetRawExpr() == nil {
			continue
		}
		if c.GetConstraint().GetRawExpr().GetFuncCall() == nil {
			continue
		}
		for _, name := range c.GetConstraint().GetRawExpr().GetFuncCall().GetFuncname() {
			return strings.ToLower(name.GetString_().GetSval())
		}
	}
	return ""
}

// a helper function to check if PostgreSQL column constraint's value is a function call _expectedFuncCall_
func utilIsConstraintAFuncCall(constraint *parser.Constraint, expectedFuncCall parser.SQLValueFunctionOp) bool {
	if constraint == nil {
		return false
	}

	fn := constraint.RawExpr.GetSqlvalueFunction()
	return nil != fn && expectedFuncCall == fn.Op
}

// a helper function to get the parameter value of a Rule by parameter name
func utilGetRuleParamInt(rule *driverV2.Rule, paramName string) int {
	param := rule.Params.GetParam(paramName)
	return param.Int()
}

// a helper function to get the parameter value of a Rule by parameter name
func utilGetRuleParamString(rule *driverV2.Rule, paramName string) string {
	param := rule.Params.GetParam(paramName)
	return param.String()
}

// a helper function to get PostgreSQL alter-table commands by command types
func utilGetAlterTableCommandsByTypes(alterTableStmt *parser.AlterTableStmt, ts ...parser.AlterTableType) []*parser.AlterTableCmd {
	ret := []*parser.AlterTableCmd{}

	if alterTableStmt == nil || alterTableStmt.GetCmds() == nil {
		return ret
	}

	for _, cmd := range alterTableStmt.GetCmds() {
		alterTableCmd := cmd.GetAlterTableCmd()
		for _, tp := range ts {
			if alterTableCmd.GetSubtype() == tp {
				ret = append(ret, alterTableCmd)
			}
		}
	}
	return ret
}

// a helper function to check if the alter table is altering table option _expectOption_ in PostgreSQL
func utilIsAlterTableCommandAlterOption(optionNode *parser.Node, targetOption string) bool {
	lis := optionNode.GetList()
	if nil == lis {
		return false
	}
	items := lis.GetItems()
	if nil == items {
		return false
	}
	for _, item := range items {
		defElem := item.GetDefElem()
		if nil != defElem {
			if strings.EqualFold(defElem.GetDefname(), targetOption) {
				return true
			}
		}
	}
	return false
}

// a helper function to get the value of the target option
func utilGetTargetOption(optionNode []*parser.Node, targetOption string) string {
	for _, item := range optionNode {
		defElem := item.GetDefElem()
		if nil != defElem {
			if strings.EqualFold(defElem.GetDefname(), targetOption) {
				if defElem.GetArg() != nil {
					return defElem.GetArg().GetString_().GetSval()
				}
			}
		}
	}
	return ""
}

// a helper function to get table foreign key names online
func utilGetTableForeignKeyNames(ctx context.Context, schemaName, tableName string) (constraintNames []string, err error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return nil, fmt.Errorf("cannot get context for rule handler")
	}

	if schemaName == "" {
		var err error
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	schemaInfo := c.PgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if nil != schemaInfo {
		for _, tableInfo := range schemaInfo.TableInfoList {
			if strings.EqualFold(tableInfo.TableName, tableName) {
				for _, c := range tableInfo.ConstraintList {
					if c.ConstraintType == ConstraintTypeF {
						constraintNames = append(constraintNames, c.ConstraintName)
					}
				}
			}
		}
	}
	return constraintNames, nil
}

// a helper function to check if a PostgreSQL constraint is a foreign key constraint
func utilIsConstraintForeignKey(ctx context.Context, schemaName, tableName string, constraintName string) (bool, error) {
	names, err := utilGetTableForeignKeyNames(ctx, schemaName, tableName)
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if constraintName == n {
			return true, nil
		}
	}
	return false, nil
}

// a helper function to get table primary key names online
func utilGetTablePrimaryKeyNames(ctx context.Context, schemaName, tableName string) (constraintNames []string, err error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return nil, fmt.Errorf("cannot get context for rule handler")
	}

	if schemaName == "" {
		var err error
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	schemaInfo := c.PgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if nil != schemaInfo {
		for _, tableInfo := range schemaInfo.TableInfoList {
			if strings.EqualFold(tableInfo.TableName, tableName) {
				for _, c := range tableInfo.ConstraintList {
					if c.ConstraintType == ConstraintTypeP {
						constraintNames = append(constraintNames, c.ConstraintName)
					}
				}
			}
		}
	}
	return constraintNames, nil
}

// a helper function to check if a PostgreSQL constraint is a primary key constraint
func utilIsConstraintPrimaryKey(ctx context.Context, schemaName, tableName string, constraintName string) (bool, error) {
	names, err := utilGetTablePrimaryKeyNames(ctx, schemaName, tableName)
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if constraintName == n {
			return true, nil
		}
	}
	return false, nil
}

// a helper function to check if a PostgreSQL node is a ddl sql
func utilIsSqlDDL(node *parser.Node) bool {
	return sqlType(node.GetNode()) == driverV2.SQLTypeDDL
}

// a helper function to check if a PostgreSQL table has the target column
func utilIsTableHasColumn(els []*parser.Node, targetColumnName string) bool {
	for _, node := range els {
		def := node.GetColumnDef()
		if nil != def {
			if strings.EqualFold(def.Colname, targetColumnName) {
				return true
			}
		}
	}
	return false
}

// a helper function to get a PostgreSQL column by column name
func utilGetColumnByName(columns []*parser.Node, columnName string) *parser.Node {
	for _, column := range columns {
		def := column.GetColumnDef()
		if nil != def {
			if strings.EqualFold(def.Colname, columnName) {
				return column
			}
		}
	}
	return nil
}

// a helper function to get a PostgreSQL table in comment stmt
func utilGetCommentStmtTable(commentStmt *parser.Node_CommentStmt) (schemaTableName [2]string) {
	if parser.ObjectType_OBJECT_TABLE != commentStmt.CommentStmt.GetObjtype() {
		return
	}
	if strings.TrimSpace(commentStmt.CommentStmt.GetComment()) == "" {
		return
	}

	items := commentStmt.CommentStmt.GetObject().GetList().GetItems()
	if len(items) == 2 {
		if items[0].GetString_() != nil {
			schemaTableName[0] = items[0].GetString_().GetSval()
		}
		if items[1].GetString_() != nil {
			schemaTableName[1] = items[1].GetString_().GetSval()
		}
	} else if len(items) == 1 {
		// FIXME: get current schema if schema is not specified
		// schema
		schemaTableName[0] = ""

		if items[0].GetString_() != nil {
			schemaTableName[1] = items[0].GetString_().GetSval()
		}
	} else {
		return
	}

	return
}

// a helper function to get a PostgreSQL column in comment stmt
func utilGetCommentStmtColumn(commentStmt *parser.Node_CommentStmt) (columnName [3]string) {
	if parser.ObjectType_OBJECT_COLUMN != commentStmt.CommentStmt.GetObjtype() {
		return [3]string{}
	}
	if strings.TrimSpace(commentStmt.CommentStmt.GetComment()) == "" {
		return [3]string{}
	}
	items := commentStmt.CommentStmt.GetObject().GetList().GetItems()

	//e.g. COMMENT ON COLUMN schema.table.column IS 'test'  // item[0] -- schema  item[1] -- table  item[2] -- column
	//e.g. COMMENT ON COLUMN table.column IS 'test'  // item[0] -- table  item[1] -- column

	if len(items) == 3 { // 显式指定schema
		// schema
		if items[0].GetString_() != nil {
			columnName[0] = items[0].GetString_().GetSval()
		}

		// table
		if items[1].GetString_() != nil {
			columnName[1] = items[1].GetString_().GetSval()
		}

		// column
		if items[2].GetString_() != nil {
			columnName[2] = items[2].GetString_().GetSval()
		}

	} else if len(items) == 2 { // 没有显式指定schema
		// FIXME: get current schema if schema is not specified
		// schema
		columnName[0] = ""

		// table
		if items[0].GetString_() != nil {
			columnName[1] = items[0].GetString_().GetSval()
		}

		// column
		if items[1].GetString_() != nil {
			columnName[2] = items[1].GetString_().GetSval()
		}
	} else {
		return [3]string{}
	}
	return columnName
}

// a helper function to get schema name from AST or current schema.
func utilGetSchemaName(ctx context.Context, schemaName string) (string, error) {
	if schemaName == "" {
		var err error
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	return schemaName, nil
}

type ScanType string

const (
	IndexScan       = "Index Scan"
	IndexOnlyScan   = "Index Only Scan"
	BitmapIndexScan = "Bitmap Index Scan"
	BitmapHeapScan  = "Bitmap Heap Scan"
)

// a helper function to check if a PostgreSQL scan type is an index scan type
func utilIsIndexScanType(scanType ScanType) bool {
	switch scanType {
	case IndexScan, IndexOnlyScan, BitmapIndexScan, BitmapHeapScan:
		return true
	}
	return false
}

var matchNodesReg = regexp.MustCompile(`(?m)(\s*"[^"]+":\s*".*")\s*("Remote plan":)`)

// a helper function to get the execution plan of a SQL statement in PostgreSQL
func utilGetExecutionPlan(ctx context.Context, sql string) (*PlanType, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	plan, err := e.GetExecutionPlan(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %v", err)
	}
	if len(plan) == 0 {
		return nil, fmt.Errorf("no execution plans found")
	}

	// 如果执行计划结果 "Node/s": "节点名" 后没有 , 号,加上逗号
	// 例子: "Node/s": "dn002"
	// 	    "Remote plan": [
	plan[0] = matchNodesReg.ReplaceAllString(plan[0], `$1,$2`)
	// 定义一个切片来存储解析后的数据
	var wrappers []PlanType

	// 解析 JSON
	if err := json.Unmarshal([]byte(plan[0]), &wrappers); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	// 检查是否有执行计划
	if len(wrappers) == 0 {
		return nil, fmt.Errorf("no execution plans found")
	}

	planType := wrappers[0].Plan

	if planType.NodeType == "" {
		return nil, fmt.Errorf("missing 'Plan' key in JSON")
	}

	// 解析 "Remote Plan" 字符串中的 JSON 内容
	if planType.RemotePlanStr != "" {
		var remotePlans []PlanType
		if err := json.Unmarshal([]byte(planType.RemotePlanStr), &remotePlans); err != nil {
			return nil, fmt.Errorf("JSON unmarshal Remote Plan error: %v", err)
		}
		planType.RemotePlanArray = &remotePlans
	}
	return planType, nil
}

// a helper function to get the execution plan text of a SQL statement in PostgreSQL
func utilGetExecutionPlanText(ctx context.Context, sql string) ([]string, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	rows, err := e.GetExecutionPlanNoJson(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %v", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no execution plans found")
	}
	return rows, nil
}

// a helper function to get the execution analyze plan of a SQL statement in PostgreSQL
func utilGetExecutionAnalyzePlan(ctx context.Context, sql string) (*PlanType, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	plan, err := e.GetExecutionAnalyzePlan(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to get analyze plan: %v", err)
	}
	if len(plan) == 0 {
		return nil, fmt.Errorf("no execution analyze plans found")
	}

	var jsonQueryPlans []*AnalyzePlan
	if err := json.Unmarshal([]byte(plan[0]), &jsonQueryPlans); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	if len(jsonQueryPlans) == 0 {
		return nil, fmt.Errorf("no execution plans found")
	}

	return &jsonQueryPlans[0].Plan, nil
}

// a helper function recursively visits each node in the plan tree
func utilVisitPlan(plan *PlanType, processor NodeProcessor) (skipVisit bool) {
	if plan == nil {
		return false
	}

	// Process the current node
	if processor(plan) {
		return true
	}

	// Recursively process each sub-plan.
	// Plans 可能为两种 JSON 形态：1) 包装格式 [{ "Plan": { "Node Type": "..." } }]；2) 直接格式 [{ "Node Type": "...", ... }]（PostgreSQL 实际返回）。
	// 若子元素带 Plan 则递归其 Plan，否则若子元素自身有 NodeType 则递归子元素本身。
	if plan.Plans != nil {
		for i := range *plan.Plans {
			sub := &(*plan.Plans)[i]
			next := sub.Plan
			if next == nil && sub.NodeType != "" {
				next = sub
			}
			if next != nil && utilVisitPlan(next, processor) {
				return true
			}
		}
	}

	// Recursively process each Remote Plan
	// TBase is a distributed database system that distributes update tasks to various nodes for execution, and encapsulates these distributed execution plans in remote_plan
	if plan.RemotePlanArray != nil {
		for i := range *plan.RemotePlanArray {
			for ii := range *(*plan.RemotePlanArray)[i].Plan.Plans {
				if utilVisitPlan(&(*(*plan.RemotePlanArray)[i].Plan.Plans)[ii], processor) {
					return true
				}
			}
			if utilVisitPlan((*plan.RemotePlanArray)[i].Plan, processor) {
				return true
			}
		}
	}
	return false
}

// a helper function to extract all range vars from a given parser Node
func utilGetRangeVars(node *parser.Node) []*parser.RangeVar {
	if node == nil {
		return nil
	}
	e := rangeVarExtractor{}
	node.Node.Accept(&e)
	return e.RangeVars
}

// a helper function to extract all select stmt from a given parser Node in PostgreSQL
func utilGetSelectStmt(node *parser.Node) []*parser.SelectStmt {
	if node == nil {
		return nil
	}
	selectStmtExtractor := selectStmtExtractor{}
	node.Node.Accept(&selectStmtExtractor)
	return selectStmtExtractor.SelectStmts
}

// a helper function to extract all join expr from a given parser Node in PostgreSQL
func utilGetJoinExpr(node *parser.Node) []*parser.JoinExpr {
	if node == nil {
		return nil
	}
	joinExprExtractor := joinExprExtractor{}
	node.Node.Accept(&joinExprExtractor)
	return joinExprExtractor.JoinExprs
}

// a helper function to extract all common table expr from a given parser Node in PostgreSQL
func utilGetCommonTableExpr(node *parser.Node) []*parser.CommonTableExpr {
	if node == nil {
		return nil
	}
	commonTableExprExtractor := commonTableExprExtractor{}
	node.Node.Accept(&commonTableExprExtractor)
	return commonTableExprExtractor.exprs
}

// a helper function to extract all func calls from a given parser Node in PostgreSQL
func utilGetFuncCall(node *parser.Node) []*parser.FuncCall {
	if node == nil {
		return nil
	}
	funcCallExtractor := funcCallExtractor{}
	node.Node.Accept(&funcCallExtractor)
	return funcCallExtractor.funcs
}

// a helper function to extract all sublink from a given parser Node in PostgreSQL
func utilGetSubLink(node *parser.Node) []*parser.SubLink {
	if node == nil {
		return nil
	}
	sublinkExtractor := subLinkExtractor{}
	node.Node.Accept(&sublinkExtractor)
	return sublinkExtractor.SubLinks
}

// a helper function to extract all column refs from given parser Node in PostgreSQL
func utilGetColumnRefFromNodes(nodes ...*parser.Node) []*parser.ColumnRef {
	columnRefExtractor := columnRefExtractor{}
	for _, node := range nodes {
		if node != nil {
			node.Node.Accept(&columnRefExtractor)
		}
	}
	return columnRefExtractor.columnRef
}

// a helper function to extract raw sql from context in PostgreSQL
func utilGetRawSQLFromCtx(ctx context.Context) (string, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return "", fmt.Errorf("cannot get context for rule handler")
	}

	return c.GetRawSQL(), nil
}

// a helper function to get the default collate
func utilGetDefaultCollate(ctx context.Context) (string, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return "", fmt.Errorf("cannot get context for rule handler")
	}

	return c.PgContext.GetDatabaseDefaultCollate(), nil
}

// a helper function to get the PostgreSQL table column info
func utilGetTableColumnsInfo(ctx context.Context, schemaName, tableName string) ([]*ColumnInfo, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return nil, fmt.Errorf("cannot get context for rule handler")
	}

	if schemaName == "" {
		var err error
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	schemaInfo := c.PgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if nil != schemaInfo {
		for _, tableInfo := range schemaInfo.TableInfoList {
			if strings.EqualFold(tableInfo.TableName, tableName) {
				return tableInfo.ColumnInfoList, nil
			}
		}
	}
	return nil, fmt.Errorf("schema %v or table %v not found", schemaName, tableName)
}

// a helper function to get the PostgreSQL table index info
func utilGetTableIndexInfo(ctx context.Context, schemaName, tableName string) ([]*IndexInfo, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return nil, fmt.Errorf("cannot get context for rule handler")
	}

	if schemaName == "" {
		var err error
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	schemaInfo := c.PgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if nil == schemaInfo {
		return nil, fmt.Errorf("schema %v not found", schemaName)

	}
	ret := []*IndexInfo{}
	for _, indexInfo := range schemaInfo.IndexInfoList {
		if strings.EqualFold(indexInfo.TableName, tableName) {
			ret = append(ret, indexInfo)
		}
	}
	return ret, nil
}

// a helper function to get the PostgreSQL table sharding column
func utilGetTableDistributionColumn(ctx context.Context, schemaName, tableName string) (string, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return "", fmt.Errorf("cannot get context for rule handler")
	}

	if schemaName == "" {
		var err error
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	schemaInfo := c.PgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if nil != schemaInfo {
		for _, tableInfo := range schemaInfo.TableInfoList {
			if strings.EqualFold(tableInfo.TableName, tableName) {
				return tableInfo.DistributionColumnName, nil
			}
		}
	}
	return "", fmt.Errorf("schema %v or table %v not found", schemaName, tableName)
}

// a helper function to check if a given PostgreSQL table is distributed table
func utilIsTableDistributed(ctx context.Context, schemaName, tableName string) (bool, error) {
	distributionKey, err := utilGetTableDistributionColumn(ctx, schemaName, tableName)
	if err != nil {
		return false, err
	}
	return distributionKey != "", nil
}

// a helper function to get the PostgreSQL views
func utilGetExistedViews(ctx context.Context, schemaName string) ([]*TableInfo, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return nil, fmt.Errorf("cannot get context for rule handler")
	}

	if schemaName == "" {
		var err error
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	schemaInfo := c.PgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if nil == schemaInfo {
		return nil, fmt.Errorf("schema %v not found", schemaName)
	}

	ret := []*TableInfo{}
	for _, v := range schemaInfo.TableInfoList {
		if v.IsView {
			ret = append(ret, v)
		}
	}

	return ret, nil
}

// a helper function to check if a PostgreSQL column is of type enum
func utilIsColumnTypeEnum(columnDefNode *parser.Node, tableColumnInfo []*ColumnInfo) bool {
	// TODO: 需要补全
	return false
}

// a helper function to check if a string is in list
func utilIsStrInSlice(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

// a helper function to get the PostgreSQL table type
func utilGetTableType(ctx context.Context, schemaName string, relName string) (executor.TableType, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	tableType, err := e.GetTableType(schemaName, relName)
	if err != nil {
		return "", fmt.Errorf("failed to get table type: %v", err)
	}

	return tableType, nil
}

// a helper function to get the PostgreSQL table sub tables
func utilGetSubTables(ctx context.Context, tableType executor.TableType, schemaName, tableName string) ([]*executor.SubTable, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	switch tableType {
	case executor.RelationTableType:
		return []*executor.SubTable{{schemaName, tableName}}, nil
	case executor.OriginPartTableType:
		return e.GetOriginPartSubTables(schemaName, tableName)
	case executor.SelfStudyPartTableType:
		return e.GetSelfStudyPartSubTables(schemaName, tableName)
	}

	return nil, fmt.Errorf("table type %v is not supported", tableType)
}

// a helper function to get the PostgreSQL all sub tables size
func utilGetTableSizeGBByNode(ctx context.Context, dataNode, schemaName string, tableName []string) ([]*executor.TableSize, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	tableSizeList, err := e.GetTableSizeGBByNode(dataNode, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table size: %v", err)
	}

	return tableSizeList, nil
}

// a helper function to get the PostgreSQL current table size
func utilGetTableSizeGBBySingleNode(ctx context.Context, schemaName string, tableName []string) ([]*executor.TableSize, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	tableSizeList, err := e.GetTableSizeGBBySingleNode(schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table size: %v", err)
	}

	return tableSizeList, nil
}

// a helper function to get the PostgreSQL table size of a table
func utilGetTableSize(ctx context.Context, schemaName, tableName string) (int, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	size, err := e.GetTableSizeMB(schemaName, tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to get table size: %v", err)
	}
	return size, nil
}

// a helper function to get the discriminator of an index in PostgreSQL
func utilCalculateIndexDiscrimination(ctx context.Context, schemaName, tableName string, indexColumns []string) (map[string]float64, error) {
	if len(indexColumns) == 0 {
		return nil, fmt.Errorf("no index columns provided")
	}

	executor, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from context: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from context: %v", err)
		}
	}

	columnSelectivityMap, err := executor.GetColumnSelectivity(schemaName, tableName, indexColumns)
	if err != nil {
		return nil, fmt.Errorf("failed to get selectivity for tableName %s: %v", tableName, err)
	}

	return columnSelectivityMap, nil
}

// a helper function to get the default table from a given select statement in PostgreSQL
func utilGetDefaultTable(selectStmt *parser.SelectStmt) *parser.RangeVar {
	if selectStmt == nil {
		return nil
	}

	// Ensure the FROM clause is present
	if selectStmt.FromClause == nil || len(selectStmt.FromClause) == 0 {
		return nil
	}

	// Check if the first item in the FROM clause is a RangeVar
	if rangeVar, ok := selectStmt.FromClause[0].GetNode().(*parser.Node_RangeVar); ok {
		return rangeVar.RangeVar
	}

	return nil
}

// a helper function to get the table alias name from a given table in PostgreSQL
func utilGetTableAliasName(table *parser.RangeVar) string {
	if table.Alias != nil {
		return table.Alias.Aliasname
	}
	return ""
}

// a helper function to recursively extracts TableAliasInfo from a parser.Node in PostgreSQL
func utilGetTableAliasInfoFromNode(node *parser.Node) []*tableAliasInfo {
	var tableAliasInfos []*tableAliasInfo

	if joinExpr, ok := node.Node.(*parser.Node_JoinExpr); ok {
		// Recursively process the left and right parts of the join
		tableAliasInfos = append(tableAliasInfos, utilGetTableAliasInfoFromNode(joinExpr.JoinExpr.Larg)...)
		tableAliasInfos = append(tableAliasInfos, utilGetTableAliasInfoFromNode(joinExpr.JoinExpr.Rarg)...)
	} else if rangeVar, ok := node.Node.(*parser.Node_RangeVar); ok {
		// Extract alias information from the RangeVar node
		tableAliasInfo := &tableAliasInfo{
			TableAliasName: utilGetTableAliasName(rangeVar.RangeVar),
			TableName:      rangeVar.RangeVar.Relname,
			SchemaName:     rangeVar.RangeVar.Schemaname,
		}
		tableAliasInfos = append(tableAliasInfos, tableAliasInfo)
	}

	return tableAliasInfos
}

// a helper function to get the alias information from a given DML statement in PostgreSQL
func utilGetAliasInfoFromDML(node *parser.Node) (alias []*tableAliasInfo, defaultTable, defaultSchema string) {
	switch node.Node.(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		selectStmts := utilGetSelectStmt(node)
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
		}
	}

	switch stmt := node.Node.(type) {
	case *parser.Node_DeleteStmt:
		// "delete..."
		defaultTable = stmt.DeleteStmt.Relation.Relname
		defaultSchema = stmt.DeleteStmt.Relation.Schemaname
	case *parser.Node_UpdateStmt:
		// "update..."
		defaultSchema = stmt.UpdateStmt.Relation.Schemaname
		defaultTable = stmt.UpdateStmt.Relation.Relname
	case *parser.Node_InsertStmt:
		// "insert..."
		defaultSchema = stmt.InsertStmt.Relation.Schemaname
		defaultTable = stmt.InsertStmt.Relation.Relname
	}

	return alias, defaultTable, defaultSchema
}

// a helper function to extracts the WHERE clause from a DML statement in PostgreSQL.
func utilGetWhereExprFromDMLStmt(node *parser.Node) (whereList []*parser.Node) {
	switch node.Node.(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		for _, selectStmt := range utilGetSelectStmt(node) {
			// "select..."
			if selectStmt.WhereClause != nil {
				whereList = append(whereList, selectStmt.WhereClause)
			}
		}
	}

	switch stmt := node.Node.(type) {
	case *parser.Node_DeleteStmt:
		// "delete..."
		if stmt.DeleteStmt.WhereClause != nil {
			whereList = append(whereList, stmt.DeleteStmt.WhereClause)
		}
	case *parser.Node_UpdateStmt:
		// "update..."
		if stmt.UpdateStmt.WhereClause != nil {
			whereList = append(whereList, stmt.UpdateStmt.WhereClause)
		}
	}
	return whereList
}

// a helper function to scan the where clause of a SQL statement and apply the given function to each expression node
func utilScanWhereStmt(fn func(expr *parser.Node) (skip bool), exprs ...*parser.Node) {
	for _, expr := range exprs {
		if expr == nil {
			continue
		}
		// skip all children node
		if fn(expr) {
			continue
		}

		switch node := (*expr).Node.(type) {
		case *parser.Node_AExpr:
			utilScanWhereStmt(fn, node.AExpr.GetLexpr(), node.AExpr.GetRexpr())
		case *parser.Node_BoolExpr:
			for _, arg := range node.BoolExpr.GetArgs() {
				utilScanWhereStmt(fn, arg)
			}
		case *parser.Node_SubLink:
			utilScanWhereStmt(fn, node.SubLink.GetSubselect())
		case *parser.Node_CaseExpr:
			utilScanWhereStmt(fn, node.CaseExpr.GetDefresult())
			for _, arg := range node.CaseExpr.GetArgs() {
				utilScanWhereStmt(fn, arg)
			}
			// Add other node types as needed
		}
	}

}

// a helper function to check if a TBase expression is a constant true
func utilIsExprConstTrue(expr *parser.Node) bool {
	getConstantValue := func(expr *parser.Node) string {
		// 检查表达式是否为常量节点
		constant, ok := expr.Node.(*parser.Node_AConst)
		if ok {
			switch nodeType := constant.AConst.Val.(type) {
			case *parser.A_Const_Ival:
				return strconv.Itoa(int(nodeType.Ival.GetIval()))
			case *parser.A_Const_Fval:
				return nodeType.Fval.GetFval()
			case *parser.A_Const_Sval:
				return nodeType.Sval.GetSval()
			}
		}
		typeCast, ok := expr.Node.(*parser.Node_TypeCast)
		if ok {
			return getConstantValue(typeCast.TypeCast.Arg)
		}
		columnRef, ok := expr.Node.(*parser.Node_ColumnRef)
		if ok {
			if columnRef != nil && columnRef.ColumnRef != nil {
				fields := columnRef.ColumnRef.Fields
				columnNames := make([]string, 0)
				for _, field := range fields {
					columnNames = append(columnNames, strings.ToLower(field.GetString_().GetSval()))
				}
				return strings.Join(columnNames, ".")
			}
		}
		return ""
	}

	switch clause := expr.Node.(type) {
	case *parser.Node_BoolExpr:
		// 检查子表达式
		if clause.BoolExpr.Boolop == parser.BoolExprType_AND_EXPR {
			leftTrue := utilIsExprConstTrue(clause.BoolExpr.Args[0])
			rightTrue := utilIsExprConstTrue(clause.BoolExpr.Args[1])
			// 若子表达式都为 true，则整个表达式为 true
			if leftTrue && rightTrue {
				return true
			}
		} else if clause.BoolExpr.Boolop == parser.BoolExprType_OR_EXPR {
			leftTrue := utilIsExprConstTrue(clause.BoolExpr.Args[0])
			rightTrue := utilIsExprConstTrue(clause.BoolExpr.Args[1])
			// 若子表达式有一个为true，则整个表达式为 true
			if leftTrue || rightTrue {
				return true
			}
		}
	case *parser.Node_AExpr:
		// 检查节点类型为 Op 的表达式
		if clause.AExpr.Kind == parser.A_Expr_Kind_AEXPR_OP {
			// 获取左右值
			leftValue := getConstantValue(clause.AExpr.Lexpr)
			rightValue := getConstantValue(clause.AExpr.Rexpr)
			switch clause.AExpr.Name[0].GetString_().GetSval() {
			case "=":
				// 检查左右值是否相等
				if leftValue == rightValue {
					return true
				}
			case "!=", "<>":
				// 检查左右值是否不相等
				if leftValue != rightValue {
					return true
				}
			case ">", ">=", "<", "<=":
				// 将值转换为适当的类型 (例如，将字符串转换为数字)
				leftNum, leftErr := strconv.ParseFloat(leftValue, 64)
				rightNum, rightErr := strconv.ParseFloat(rightValue, 64)
				// 左右都是数值才比较大小
				if leftErr == nil && rightErr == nil {
					// 根据运算符进行比较
					switch clause.AExpr.Name[0].GetString_().GetSval() {
					case ">":
						if leftNum > rightNum {
							return true
						}
					case ">=":
						if leftNum >= rightNum {
							return true
						}
					case "<":
						if leftNum < rightNum {
							return true
						}
					case "<=":
						if leftNum <= rightNum {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// a helper function to determine if a given column is indexed according to the provided index information.
func utilIsColumnIndexed(column *parser.ColumnRef, indexInfo []*IndexInfo) bool {
	_, _, c := utilParseColumnRef(column)
	for _, index := range indexInfo {
		for _, col := range index.ColumnList {
			if c == col {
				return true
			}
		}
	}
	return false
}

// a helper function to determine if a given func call is indexed according to the provided index information.
func utilIsFuncCallIndexed(f *parser.FuncCall, indexInfo []*IndexInfo) bool {
	for _, index := range indexInfo {
		for _, funcCall := range index.FuncCallList {
			if extractProtoMessage(f.String()) == extractProtoMessage(funcCall.String()) {
				return true
			}
		}
	}
	return false
}

// a helper function to get create database stmt encoding option
func utilGetCreateDatabaseEncoding(stmt *parser.CreatedbStmt) string {
	for _, option := range stmt.Options {
		if option.GetDefElem() != nil && option.GetDefElem().Defname == "encoding" && option.GetDefElem().Arg.GetString_() != nil {
			return option.GetDefElem().Arg.GetString_().GetSval()
		}
	}
	return ""
}

// a helper function to get PostgreSQL table partition child table list
func utilGetChildPartitionTableList(ctx context.Context, schemaName, tableName string) ([]string, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	list, err := e.GetChildPartitionTableList(schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get child partition table: %v", err)
	}
	return list, nil
}

// A helper function to get the count(*) value with where filtering
func utilGetRecordCountQuerySQL(ctx context.Context, schemaName, tableName string, whereClause *parser.Node) (int, error) {
	e, err := getExecutorFromCtx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get executor from ctx: %v", err)
	}

	if schemaName == "" {
		schemaName, err = getCurrentSchemaFromCtx(ctx)
		if err != nil {
			return -1, fmt.Errorf("failed to get current schema from ctx: %v", err)
		}
	}

	value, err := e.GetRecordCountQuerySQL(schemaName, tableName, whereClause)
	if err != nil {
		return -1, fmt.Errorf("failed to get child partition table: %v", err)
	}
	return value, nil
}

// end helper function file. this line which used for ai scanner should be at the end of the file, please do not delete it

// NodeProcessor defines the function type for processing nodes
type NodeProcessor func(*PlanType) (skipVisit bool)

type tableAliasInfo struct {
	TableName      string
	SchemaName     string
	TableAliasName string
}

// removeLocation removes the location element from the input string
func removeLocation(input string) string {
	// Use regex to remove the location element
	re := regexp.MustCompile(`\s+location:\d+`)
	return re.ReplaceAllString(input, "")
}

// extractElements extracts the elements from the input string and sorts them
func extractProtoMessage(input string) string {
	input = removeLocation(input)
	// Split the input string on spaces to get individual elements
	elements := strings.Fields(input)

	// Sort the elements
	sort.Strings(elements)

	// Join the elements back into a single string
	return strings.Join(elements, " ")
}
