package inspector

import (
	"context"
	"fmt"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

const (
	RuleId6301 = "pg_63_01"
	RuleId6302 = "pg_63_02"
)

// 这两条规则由规则63拆分而来
func init() {
	newRules := []RuleHandler{
		{
			Rule: driverV2.Rule{
				Name:       RuleId6301,
				Desc:       "唯一索引中的索引字段必须要有非空约束",
				Annotation: "唯一约束（UNIQUE）默认允许多个 NULL 值存在，因为 NULL 在比较时不被视为相等。这会导致在包含 NULL 的情况下，唯一约束无法保证数据的实际唯一性。如果业务要求字段在所有记录中唯一（包括不允许多个 NULL），建议将字段定义为 NOT NULL，或使用 NULLS NOT DISTINCT 以确保 NULL 也参与唯一性校验。",
				Level:      driverV2.RuleLevelError,
				Category:   RuleTypeIndexConvention,
			},
			Message:      "唯一索引/约束字段(%v)建议设置为 NOT NULL，否则 NULL 值可能绕过唯一性约束",
			AstSQLHandler: rule6301,
			AllowOffline: false,
		},
		{
			Rule: driverV2.Rule{
				Name:       RuleId6302,
				Desc:       "普通索引中的索引字段需要有非空约束",
				Annotation: "普通索引字段允许 NULL 值。查询 NULL 时必须使用 IS NULL，而使用 = NULL 无法匹配数据，可能导致查询结果不符合预期。此外，当索引列中存在大量 NULL 或重复值时，会降低索引区分度，从而影响优化器选择执行计划，可能降低查询性能。建议根据业务语义合理设计是否允许 NULL。",
				Level:      driverV2.RuleLevelWarn,
				Category:   RuleTypeIndexConvention,
			},
			Message:      "普通索引字段(%v)包含 NULL 可能影响查询正确性或性能，请根据业务场景评估是否设置 NOT NULL",
			AstSQLHandler: rule6302,
			AllowOffline: false,
		},
	}

	RuleHandlers = append(RuleHandlers, newRules...)
	for _, rh := range newRules {
		RuleHandlerMap[rh.Rule.Name] = rh
	}
}

func rule6301(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}
	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		return handle6301CreateTable(rule, stmt)
	case *parser.Node_AlterTableStmt:
		return handle6301AlterTable(rule, pgContext, stmt)
	case *parser.Node_IndexStmt:
		return handle6301CreateIndex(rule, pgContext, stmt)
	default:
		return "", nil
	}
}

func rule6302(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}
	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_AlterTableStmt:
		return handle6302AlterTable(rule, pgContext, stmt)
	case *parser.Node_IndexStmt:
		return handle6302CreateIndex(rule, pgContext, stmt)
	default:
		// TBase的建表语句中不包含普通索引的创建
		return "", nil
	}
}

// handle6301CreateTable checks CREATE TABLE for UNIQUE/PRIMARY columns that are nullable.
// In PostgreSQL, PRIMARY KEY implies NOT NULL, so those columns are excluded from violations.
func handle6301CreateTable(rule *driverV2.Rule, stmt *parser.Node_CreateStmt) (string, error) {
	tableElts := stmt.CreateStmt.GetTableElts()
	notNullCols := collectNotNullColsFromTableElts(tableElts)
	indexCols := collectUniqueOrPrimaryColsFromTableElts(tableElts)
	nullableCols := findNullableCols(indexCols, notNullCols)
	if len(nullableCols) > 0 {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(nullableCols, ",")), nil
	}
	return "", nil
}

// handle6301AlterTable checks:
//   - ADD CONSTRAINT UNIQUE/PRIMARY: the constraint columns must be NOT NULL in the existing table.
//   - ALTER COLUMN ... DROP NOT NULL: if the column belongs to a unique or primary index.
func handle6301AlterTable(rule *driverV2.Rule, pgContext *PgContext, stmt *parser.Node_AlterTableStmt) (string, error) {
	schemaName := resolveSchemaName(stmt.AlterTableStmt.Relation.Schemaname, pgContext)
	tableName := stmt.AlterTableStmt.Relation.Relname
	notNullCols, tableExists := getTableNotNullCols(pgContext, schemaName, tableName)

	allNullable := make([]string, 0)
	for _, cmd := range stmt.AlterTableStmt.GetCmds() {
		cmdNode, ok := cmd.GetNode().(*parser.Node_AlterTableCmd)
		if !ok {
			continue
		}
		switch cmdNode.AlterTableCmd.GetSubtype() {
		case parser.AlterTableType_AT_DropNotNull:
			colName := cmdNode.AlterTableCmd.GetName()
			if isColumnInUniqueOrPrimaryIndex(pgContext, schemaName, tableName, colName) {
				allNullable = append(allNullable, colName)
			}
		case parser.AlterTableType_AT_AddConstraint:
			if !tableExists {
				continue
			}
			nullable := collectNullableColsFromAddUniqueOrPrimaryConstraint(cmdNode, notNullCols)
			allNullable = append(allNullable, nullable...)
		}
	}

	unique := deduplicatePreserveOrder(allNullable)
	if len(unique) > 0 {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(unique, ",")), nil
	}
	return "", nil
}

// handle6301CreateIndex checks CREATE UNIQUE INDEX for nullable columns.
// Only unique indexes are in scope; ordinary CREATE INDEX is handled by rule6302.
func handle6301CreateIndex(rule *driverV2.Rule, pgContext *PgContext, stmt *parser.Node_IndexStmt) (string, error) {
	if !stmt.IndexStmt.Unique {
		return "", nil
	}
	schemaName := resolveSchemaName(stmt.IndexStmt.Relation.Schemaname, pgContext)
	tableName := stmt.IndexStmt.Relation.Relname
	notNullCols, exists := getTableNotNullCols(pgContext, schemaName, tableName)
	if !exists {
		return "", nil
	}
	indexCols := collectIndexParamCols(stmt.IndexStmt.IndexParams)
	nullableCols := findNullableCols(indexCols, notNullCols)
	if len(nullableCols) > 0 {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(nullableCols, ",")), nil
	}
	return "", nil
}

// handle6302AlterTable checks ALTER COLUMN ... DROP NOT NULL where the column is in a normal (non-unique) index.
// Note: PostgreSQL does not support ALTER TABLE ADD INDEX; normal indexes are created via CREATE INDEX.
func handle6302AlterTable(rule *driverV2.Rule, pgContext *PgContext, stmt *parser.Node_AlterTableStmt) (string, error) {
	schemaName := resolveSchemaName(stmt.AlterTableStmt.Relation.Schemaname, pgContext)
	tableName := stmt.AlterTableStmt.Relation.Relname

	allNullable := make([]string, 0)
	for _, cmd := range stmt.AlterTableStmt.GetCmds() {
		cmdNode, ok := cmd.GetNode().(*parser.Node_AlterTableCmd)
		if !ok {
			continue
		}
		if cmdNode.AlterTableCmd.GetSubtype() != parser.AlterTableType_AT_DropNotNull {
			continue
		}
		colName := cmdNode.AlterTableCmd.GetName()
		if isColumnInNormalIndex(pgContext, schemaName, tableName, colName) {
			allNullable = append(allNullable, colName)
		}
	}

	unique := deduplicatePreserveOrder(allNullable)
	if len(unique) > 0 {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(unique, ",")), nil
	}
	return "", nil
}

// handle6302CreateIndex checks CREATE INDEX (non-unique) for nullable columns.
// CREATE UNIQUE INDEX is handled exclusively by rule6301.
func handle6302CreateIndex(rule *driverV2.Rule, pgContext *PgContext, stmt *parser.Node_IndexStmt) (string, error) {
	if stmt.IndexStmt.Unique {
		return "", nil
	}
	schemaName := resolveSchemaName(stmt.IndexStmt.Relation.Schemaname, pgContext)
	tableName := stmt.IndexStmt.Relation.Relname
	notNullCols, exists := getTableNotNullCols(pgContext, schemaName, tableName)
	if !exists {
		return "", nil
	}
	indexCols := collectIndexParamCols(stmt.IndexStmt.IndexParams)
	nullableCols := findNullableCols(indexCols, notNullCols)
	if len(nullableCols) > 0 {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(nullableCols, ",")), nil
	}
	return "", nil
}

// --- Shared helper functions ---

// resolveSchemaName returns schemaName if non-empty, otherwise falls back to the current schema from pgContext.
func resolveSchemaName(schemaName string, pgContext *PgContext) string {
	if schemaName != "" {
		return schemaName
	}
	if pgContext != nil && pgContext.DatabaseInfo != nil {
		return pgContext.DatabaseInfo.CurrentSchema
	}
	return ""
}

// collectNotNullColsFromTableElts builds the NOT NULL column set from a CREATE TABLE element list.
// Column-level NOT NULL and column-level/table-level PRIMARY KEY all imply NOT NULL in PostgreSQL.
func collectNotNullColsFromTableElts(tableElts []*parser.Node) map[string]struct{} {
	notNull := make(map[string]struct{})
	for _, elt := range tableElts {
		colName := elt.GetColumnDef().GetColname()
		if colName == "" {
			continue
		}
		for _, c := range elt.GetColumnDef().GetConstraints() {
			switch c.GetConstraint().GetContype() {
			case parser.ConstrType_CONSTR_NOTNULL, parser.ConstrType_CONSTR_PRIMARY:
				notNull[colName] = struct{}{}
			}
		}
	}
	// Table-level PRIMARY KEY: all key columns are implicitly NOT NULL.
	for _, elt := range tableElts {
		if elt.GetConstraint() == nil {
			continue
		}
		if elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_PRIMARY {
			for _, key := range elt.GetConstraint().GetKeys() {
				notNull[key.GetString_().GetSval()] = struct{}{}
			}
		}
	}
	return notNull
}

// collectUniqueOrPrimaryColsFromTableElts collects all columns referenced by UNIQUE or PRIMARY constraints
// in a CREATE TABLE statement, including both column-level and table-level constraints.
func collectUniqueOrPrimaryColsFromTableElts(tableElts []*parser.Node) []string {
	cols := make([]string, 0)
	for _, elt := range tableElts {
		colName := elt.GetColumnDef().GetColname()
		if colName == "" {
			continue
		}
		for _, c := range elt.GetColumnDef().GetConstraints() {
			switch c.GetConstraint().GetContype() {
			case parser.ConstrType_CONSTR_UNIQUE, parser.ConstrType_CONSTR_PRIMARY:
				cols = append(cols, colName)
			}
		}
	}
	// Table-level UNIQUE/PRIMARY constraints.
	for _, elt := range tableElts {
		if elt.GetConstraint() == nil {
			continue
		}
		switch elt.GetConstraint().GetContype() {
		case parser.ConstrType_CONSTR_UNIQUE, parser.ConstrType_CONSTR_PRIMARY:
			for _, key := range elt.GetConstraint().GetKeys() {
				cols = append(cols, key.GetString_().GetSval())
			}
		}
	}
	return cols
}

// getTableNotNullCols returns (notNullCols, tableExists) by looking up the table in pgContext.
// notNullCols maps column names that are NOT NULL; tableExists indicates the table was found.
func getTableNotNullCols(pgContext *PgContext, schemaName, tableName string) (map[string]struct{}, bool) {
	tableInfo, _ := getTableAndIndexesForRule63(pgContext, schemaName, tableName)
	if tableInfo == nil {
		return make(map[string]struct{}), false
	}
	notNull := make(map[string]struct{}, len(tableInfo.ColumnInfoList))
	for _, col := range tableInfo.ColumnInfoList {
		if col != nil && !col.IsNullable {
			notNull[col.ColumnName] = struct{}{}
		}
	}
	return notNull, true
}

// collectNullableColsFromAddUniqueOrPrimaryConstraint extracts nullable columns from an AT_AddConstraint
// command that specifies a UNIQUE or PRIMARY constraint. The caller must confirm the subtype is AT_AddConstraint.
func collectNullableColsFromAddUniqueOrPrimaryConstraint(
	cmdNode *parser.Node_AlterTableCmd,
	notNullCols map[string]struct{},
) []string {
	if cmdNode.AlterTableCmd.GetDef() == nil || cmdNode.AlterTableCmd.GetDef().GetConstraint() == nil {
		return nil
	}
	constraint := cmdNode.AlterTableCmd.GetDef().GetConstraint()
	contype := constraint.GetContype()
	if contype != parser.ConstrType_CONSTR_UNIQUE && contype != parser.ConstrType_CONSTR_PRIMARY {
		return nil
	}
	violations := make([]string, 0)
	for _, key := range constraint.GetKeys() {
		colName := key.GetString_().GetSval()
		if _, ok := notNullCols[colName]; !ok {
			violations = append(violations, colName)
		}
	}
	return violations
}

// collectIndexParamCols extracts column names from IndexParams, skipping functional index expressions.
func collectIndexParamCols(indexParams []*parser.Node) []string {
	cols := make([]string, 0, len(indexParams))
	for _, param := range indexParams {
		name := param.GetIndexElem().GetName()
		if name != "" {
			cols = append(cols, name)
		}
	}
	return cols
}

// findNullableCols returns a deduplicated, order-preserving list of columns not present in notNullCols.
func findNullableCols(indexCols []string, notNullCols map[string]struct{}) []string {
	seen := make(map[string]struct{}, len(indexCols))
	result := make([]string, 0, len(indexCols))
	for _, col := range indexCols {
		if _, isNotNull := notNullCols[col]; isNotNull {
			continue
		}
		if _, already := seen[col]; already {
			continue
		}
		seen[col] = struct{}{}
		result = append(result, col)
	}
	return result
}

// deduplicatePreserveOrder removes duplicates from cols while preserving first-occurrence order.
func deduplicatePreserveOrder(cols []string) []string {
	seen := make(map[string]struct{}, len(cols))
	result := make([]string, 0, len(cols))
	for _, col := range cols {
		if _, ok := seen[col]; ok {
			continue
		}
		seen[col] = struct{}{}
		result = append(result, col)
	}
	return result
}

func isColumnInUniqueOrPrimaryIndex(pgContext *PgContext, schemaName, tableName, columnName string) bool {
	tableInfo, indexInfoList := getTableAndIndexesForRule63(pgContext, schemaName, tableName)
	if tableInfo == nil {
		return false
	}
	for _, constraint := range tableInfo.ConstraintList {
		if constraint == nil {
			continue
		}
		if constraint.ConstraintType != ConstraintTypeP && constraint.ConstraintType != ConstraintTypeU {
			continue
		}
		if isColumnInList(columnName, constraint.ColumnList) {
			return true
		}
	}
	for _, indexInfo := range indexInfoList {
		if indexInfo == nil || !indexInfo.IsUnique {
			continue
		}
		if isColumnInList(columnName, indexInfo.ColumnList) {
			return true
		}
	}
	return false
}

func isColumnInNormalIndex(pgContext *PgContext, schemaName, tableName, columnName string) bool {
	_, indexInfoList := getTableAndIndexesForRule63(pgContext, schemaName, tableName)
	for _, indexInfo := range indexInfoList {
		if indexInfo == nil || indexInfo.IsUnique {
			continue
		}
		if isColumnInList(columnName, indexInfo.ColumnList) {
			return true
		}
	}
	return false
}

func getTableAndIndexesForRule63(pgContext *PgContext, schemaName, tableName string) (*TableInfo, []*IndexInfo) {
	if pgContext == nil || pgContext.DatabaseInfo == nil {
		return nil, nil
	}
	schemaInfo, ok := pgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if !ok || schemaInfo == nil {
		return nil, nil
	}
	var tableInfo *TableInfo
	for _, currentTable := range schemaInfo.TableInfoList {
		if currentTable != nil && currentTable.TableName == tableName {
			tableInfo = currentTable
			break
		}
	}
	indexInfoList := make([]*IndexInfo, 0)
	for _, indexInfo := range schemaInfo.IndexInfoList {
		if indexInfo != nil && indexInfo.TableName == tableName {
			indexInfoList = append(indexInfoList, indexInfo)
		}
	}
	return tableInfo, indexInfoList
}

func isColumnInList(columnName string, columns []string) bool {
	for _, column := range columns {
		if column == columnName {
			return true
		}
	}
	return false
}
