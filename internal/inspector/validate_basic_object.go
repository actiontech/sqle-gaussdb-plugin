package inspector

import (
	"fmt"
	"github.com/actiontech/dms/pkg/dms-common/i18nPkg"
	"github.com/actiontech/sqle-pg-plugin/internal/utils"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	parser "github.com/pganalyze/pg_query_go/v2"
	"golang.org/x/text/language"
	"reflect"
	"strings"
	"sync"
)

// newAuditResult creates an AuditResult with I18nAuditResultInfo instead of the removed Message field
func newAuditResult(level driverV2.RuleLevel, message string) *driverV2.AuditResult {
	return &driverV2.AuditResult{
		Level: level,
		I18nAuditResultInfo: map[language.Tag]driverV2.AuditResultInfo{
			i18nPkg.DefaultLang: {Message: message},
		},
	}
}

// getAuditResultMessage extracts the default language message from an AuditResult
func getAuditResultMessage(ar *driverV2.AuditResult) string {
	if ar == nil {
		return ""
	}
	if info, ok := ar.I18nAuditResultInfo[i18nPkg.DefaultLang]; ok {
		return info.Message
	}
	return ""
}

const (
	SchemaNotExist = "schema[%s]不存在"
	SchemaExist    = "schema[%s]已存在"
	TableNotExist  = "表[%s.%s]不存在"
	TableExist     = "表[%s.%s]已存在"
	IndexNotExist  = "索引[%s.%s]不存在"
	IndexExist     = "索引[%s.%s]已存在"
	ColumnNotExist = "列[%s]不存在"
	ColumnExist    = "列[%s]已存在"
)

type ValidateBasicObject struct {
	PgContext *PgContext
	RawStmt   *parser.RawStmt
	rwMutex   sync.RWMutex
}

type OperatedTable struct {
	schema     string
	tableName  string
	tableAlias string
	columns    []string
}

// Validate :校验基础对象
func (v *ValidateBasicObject) Validate() (*driverV2.AuditResult, error) {
	auditResult := newAuditResult(driverV2.RuleLevelNull, "")
	var subQueries []*parser.SelectStmt
	var err error
	switch stmt := v.RawStmt.GetStmt().GetNode().(type) {
	case *parser.Node_CreateSchemaStmt:
		auditResult, err = v.ValidateCreateSchema(stmt.CreateSchemaStmt.Schemaname)
	case *parser.Node_DropStmt:
		auditResult, err = v.DropStatement(stmt.DropStmt)
	case *parser.Node_CreateStmt:
		ownerName, tableName := GetOwnerNameAndTableName(stmt.CreateStmt.Relation, v.PgContext)
		auditResult, err = v.ValidateCreateTable(ownerName, tableName)
	case *parser.Node_AlterTableStmt:
		// 区分alter table/alter index
		if stmt.AlterTableStmt.Relkind == parser.ObjectType_OBJECT_TABLE {
			auditResult, err = v.ValidateAlterTable(stmt.AlterTableStmt)
		} else if stmt.AlterTableStmt.Relkind == parser.ObjectType_OBJECT_INDEX {
			schemaName := stmt.AlterTableStmt.Relation.Schemaname
			if len(schemaName) == 0 {
				schemaName = v.PgContext.DatabaseInfo.CurrentSchema
			}
			indexName := stmt.AlterTableStmt.Relation.Relname
			isIndex, err := v.PgContext.IsExistIndex(schemaName, "", indexName)
			if err != nil {
				return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf("校验索引[%s.%s]是否存在错误，err=%s", schemaName, indexName, err)), err
			}
			if !isIndex {
				return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(IndexNotExist, schemaName, indexName)), nil
			}
		}
	case *parser.Node_IndexStmt:
		auditResult, err = v.ValidateIndex(stmt.IndexStmt)
	case *parser.Node_RenameStmt:
		auditResult, err = v.ValidateRename(stmt.RenameStmt)
	/**
	 * 1.基础对象校验增加对schema和table的insert、update、delete，select校验
	 * 2.对于列的修改要求如下：
	 * update：只检查set后的列是否存在，列后面条件中的列不校验，where条件中的列不校验
	 * delete：校验where后面的列，列关联子查询中列不校验
	 * insert：校验insert into test(column1,...)中的column1...,where条件后的列不校验
	 * select：列不校验
	 */
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(v.RawStmt.GetStmt())...)
		auditResult, err = validateDmlSchemaAndTableExist(subQueries, v)
	case *parser.Node_InsertStmt:
		auditResult, err = validateDmlSchemaNameAndTableNameExist(stmt.InsertStmt.Relation, v)
		if auditResult.Level != driverV2.RuleLevelNull {
			return auditResult, nil
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
		auditResult, err = validateDmlSchemaAndTableExist(subQueries, v)
		if auditResult.Level != driverV2.RuleLevelNull {
			return auditResult, nil
		}
		// insert：校验insert into test(column1,...)中的column1...,where条件后的列不校验
		auditResult, err = v.ValidateInsertColumns(stmt)
	case *parser.Node_UpdateStmt:
		auditResult, err = validateDmlSchemaNameAndTableNameExist(stmt.UpdateStmt.Relation, v)
		if auditResult.Level != driverV2.RuleLevelNull {
			return auditResult, nil
		}
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			// 处理sql：UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' FROM exist_db.exist_tb_2 WHERE exist_tb_1.id_shd_key = exist_tb_2.id;
			// 校验当中：FROM exist_db.exist_tb_2 schema和表是否存在
			if from.GetRangeVar() != nil {
				schemaname := from.GetRangeVar().GetSchemaname()
				if len(schemaname) == 0 {
					schemaname = v.PgContext.DatabaseInfo.CurrentSchema
				}
				relname := from.GetRangeVar().GetRelname()
				if isExistSchema, _ := v.PgContext.IsExistSchema(v.PgContext.DatabaseInfo.DatabaseName, schemaname); !isExistSchema {
					auditResult.Level = driverV2.RuleLevelWarn
					auditResult.I18nAuditResultInfo[i18nPkg.DefaultLang] = driverV2.AuditResultInfo{Message: fmt.Sprintf(SchemaNotExist, schemaname)}
					return auditResult, nil
				}
				isExistTable, err := v.PgContext.IsExistTable(schemaname, relname)
				if err != nil {
					auditResult.Level = driverV2.RuleLevelWarn
					auditResult.I18nAuditResultInfo[i18nPkg.DefaultLang] = driverV2.AuditResultInfo{Message: err.Error()}
					return auditResult, nil
				}
				if !isExistTable {
					auditResult.Level = driverV2.RuleLevelWarn
					auditResult.I18nAuditResultInfo[i18nPkg.DefaultLang] = driverV2.AuditResultInfo{Message: fmt.Sprintf(TableNotExist, schemaname, relname)}
					return auditResult, nil
				}
			}
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
		auditResult, err = validateDmlSchemaAndTableExist(subQueries, v)
		if auditResult.Level != driverV2.RuleLevelNull {
			return auditResult, nil
		}
		// update：只检查set后的列是否存在，列后面条件中的列不校验，where条件中的列不校验
		auditResult, err = v.ValidateUpdateColumns(stmt)
	case *parser.Node_DeleteStmt:
		auditResult, err = validateDmlSchemaNameAndTableNameExist(stmt.DeleteStmt.Relation, v)
		if auditResult.Level != driverV2.RuleLevelNull {
			return auditResult, nil
		}
		// 校验：DELETE FROM xxx USING test.exist_tb_2, test.exist_tb_3中USING后的表是否存在
		usingClauses := stmt.DeleteStmt.GetUsingClause()
		auditResult, err = validateDeleteUsingTable(usingClauses, v)
		if auditResult.Level != driverV2.RuleLevelNull {
			return auditResult, nil
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
		auditResult, err = validateDmlSchemaAndTableExist(subQueries, v)
		if auditResult.Level != driverV2.RuleLevelNull {
			return auditResult, nil
		}
		// delete：校验where后面的列，列关联子查询中列不校验
		auditResult, err = v.ValidateDeleteColumns(stmt)
	}

	if err != nil {
		return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf("基础对象校验错误，err=%s", err)), err
	}
	return auditResult, nil
}

func validateDeleteUsingTable(usingClauses []*parser.Node, v *ValidateBasicObject) (*driverV2.AuditResult, error) {
	notUsingTables := make([]string, 0)
	for _, usingClause := range usingClauses {
		schemaName := usingClause.GetRangeVar().Schemaname
		if len(schemaName) == 0 {
			schemaName = v.PgContext.DatabaseInfo.CurrentSchema
		}
		realName := usingClause.GetRangeVar().Relname
		// 校验schema是否存在
		isExistSchema, err := v.PgContext.IsExistSchema(v.PgContext.DatabaseInfo.DatabaseName, schemaName)
		if err != nil {
			notUsingTables = append(notUsingTables, fmt.Sprintf(SchemaNotExist, schemaName))
			continue
		}
		if !isExistSchema {
			notUsingTables = append(notUsingTables, fmt.Sprintf(SchemaNotExist, schemaName))
			continue
		}
		// 校验schema是否存在
		isExistTable, err := v.PgContext.IsExistTable(schemaName, realName)
		if err != nil {
			notUsingTables = append(notUsingTables, fmt.Sprintf(TableNotExist, schemaName, realName))
		}
		if !isExistTable {
			notUsingTables = append(notUsingTables, fmt.Sprintf(TableNotExist, schemaName, realName))
		}
	}
	if len(notUsingTables) > 0 {
		return newAuditResult(driverV2.RuleLevelWarn, strings.Join(notUsingTables, ";")), nil
	}
	return newAuditResult(driverV2.RuleLevelNull, ""), nil
}

// ValidateCreateSchema :校验创建Schema
func (v *ValidateBasicObject) ValidateCreateSchema(schemaName string) (*driverV2.AuditResult, error) {
	level, message := driverV2.RuleLevelNull, ""
	isSchema, err := v.PgContext.IsExistSchema(v.PgContext.CurrentDatabase, schemaName)
	if err != nil {
		return nil, err
	}
	if isSchema {
		level = driverV2.RuleLevelWarn
		message = fmt.Sprintf(SchemaExist, schemaName)
	}
	return newAuditResult(level, message), nil
}

// DropStatement :校验删除Statement
func (v *ValidateBasicObject) DropStatement(stmt *parser.DropStmt) (*driverV2.AuditResult, error) {
	switch stmt.RemoveType {
	case parser.ObjectType_OBJECT_SCHEMA:
		objects := stmt.Objects
		for _, object := range objects {
			schemaName := object.GetString_().GetStr()
			isSchema, err := v.PgContext.IsExistSchema(v.PgContext.CurrentDatabase, schemaName)
			if err != nil {
				return nil, err
			}
			if !isSchema {
				return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(SchemaNotExist, schemaName)), nil
			}
		}
	case parser.ObjectType_OBJECT_TABLE:
		objects := stmt.Objects
		for _, object := range objects {
			schemaName, tableName := "", ""
			var err error
			var result *driverV2.AuditResult
			var isTable bool
			items := object.GetList().GetItems()
			if len(items) == 1 {
				schemaName = v.PgContext.DatabaseInfo.CurrentSchema
				tableName = items[0].GetString_().GetStr()
			} else if len(items) == 2 {
				schemaName = items[0].GetString_().GetStr()
				tableName = items[1].GetString_().GetStr()
			}
			result, err = validateSchemaExist(schemaName, v)
			if err != nil {
				return nil, err
			}
			if result.Level != driverV2.RuleLevelNull {
				return result, nil
			}
			isTable, err = v.PgContext.IsExistTable(schemaName, tableName)
			if err != nil {
				return nil, err
			}
			if !isTable {
				return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(TableNotExist, schemaName, tableName)), nil
			}
		}
	case parser.ObjectType_OBJECT_INDEX:
		objects := stmt.Objects
		for _, object := range objects {
			schemaName, indexName := "", ""
			var err error
			var result *driverV2.AuditResult
			var isIndex bool
			items := object.GetList().GetItems()
			if len(items) == 1 {
				schemaName = v.PgContext.DatabaseInfo.CurrentSchema
				indexName = items[0].GetString_().GetStr()
			} else if len(items) == 2 {
				schemaName = items[0].GetString_().GetStr()
				indexName = items[1].GetString_().GetStr()
			}
			result, err = validateSchemaExist(schemaName, v)
			if err != nil {
				return nil, err
			}
			if result.Level != driverV2.RuleLevelNull {
				return result, nil
			}
			isIndex, err = v.PgContext.IsExistIndex(schemaName, "", indexName)
			if err != nil {
				return nil, err
			}
			if !isIndex {
				return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(IndexNotExist, schemaName, indexName)), nil
			}
		}
	}
	return newAuditResult(driverV2.RuleLevelNull, ""), nil
}

// ValidateCreateTable :校验创建table
func (v *ValidateBasicObject) ValidateCreateTable(schemaName, tableName string) (*driverV2.AuditResult, error) {
	var err error
	var result *driverV2.AuditResult
	var isTable bool
	result, err = validateSchemaExist(schemaName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	level, message := driverV2.RuleLevelNull, ""
	isTable, err = v.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	if isTable {
		level = driverV2.RuleLevelWarn
		message = fmt.Sprintf(TableExist, schemaName, tableName)
	}
	return newAuditResult(level, message), nil
}

// ValidateAlterTable :校验修改table
func (v *ValidateBasicObject) ValidateAlterTable(stmt *parser.AlterTableStmt) (*driverV2.AuditResult, error) {
	level, message := driverV2.RuleLevelNull, ""
	var err error
	var result *driverV2.AuditResult
	ownerName, tableName := GetOwnerNameAndTableName(stmt.Relation, v.PgContext)
	result, err = validateSchemaExist(ownerName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	result, err = validateTableExist(ownerName, tableName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	for _, cmd := range stmt.GetCmds() {
		switch cmd.GetAlterTableCmd().GetSubtype() {
		case parser.AlterTableType_AT_AddColumn:
			columnInfo := GetColumnInfo(ownerName, tableName, cmd.GetAlterTableCmd().GetDef().GetColumnDef())
			if v.PgContext.IsExistColumn(columnInfo.OwnerName, columnInfo.TableName, columnInfo.ColumnName) {
				level = driverV2.RuleLevelWarn
				message = fmt.Sprintf(ColumnExist, columnInfo.ColumnName)
			}
		case parser.AlterTableType_AT_AlterColumnType:
			alterColumnName := cmd.GetAlterTableCmd().GetName()
			columnInfo := GetColumnInfo(ownerName, tableName, cmd.GetAlterTableCmd().GetDef().GetColumnDef())
			columnInfo.ColumnName = alterColumnName
			if !v.PgContext.IsExistColumn(columnInfo.OwnerName, columnInfo.TableName, columnInfo.ColumnName) {
				level = driverV2.RuleLevelWarn
				message = fmt.Sprintf(ColumnNotExist, columnInfo.ColumnName)
			}
		case parser.AlterTableType_AT_DropColumn:
			columnName := cmd.GetAlterTableCmd().GetName()
			if !v.PgContext.IsExistColumn(ownerName, tableName, columnName) {
				level = driverV2.RuleLevelWarn
				message = fmt.Sprintf(ColumnNotExist, columnName)
			}
		}
	}
	return newAuditResult(level, message), nil
}

// ValidateIndex :校验索引
func (v *ValidateBasicObject) ValidateIndex(stmt *parser.IndexStmt) (*driverV2.AuditResult, error) {
	level, message := driverV2.RuleLevelNull, ""
	var result *driverV2.AuditResult
	var err error
	var isIndex bool
	indexName := stmt.GetIdxname()
	schemaName, tableName := GetOwnerNameAndTableName(stmt.Relation, v.PgContext)
	result, err = validateSchemaExist(schemaName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	result, err = validateTableExist(schemaName, tableName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	columns := make([]string, 0)
	indexParams := stmt.IndexParams
	for _, indexParam := range indexParams {
		columns = append(columns, indexParam.GetIndexElem().GetName())
	}
	isIndex, err = v.PgContext.IsExistIndex(schemaName, tableName, indexName)
	if err != nil {
		return nil, err
	}
	if isIndex {
		level = driverV2.RuleLevelWarn
		message = fmt.Sprintf(IndexExist, schemaName, indexName)
	}
	v.rwMutex.RLock()
	defer v.rwMutex.RUnlock()
	indexInfoList := v.PgContext.DatabaseInfo.SchemaInfoMap[schemaName].IndexInfoList
	for _, indexInfo := range indexInfoList {
		if tableName != indexInfo.TableName {
			continue
		}
		columnList := indexInfo.ColumnList
		equal := reflect.DeepEqual(columns, columnList)
		if equal {
			level = driverV2.RuleLevelWarn
			message = fmt.Sprintf("表[%s.%s]上列[%s]对应索引已存在",
				schemaName, tableName, strings.Join(columns, ","))
			break
		}
	}
	return newAuditResult(level, message), nil
}

// ValidateRename :校验重命名
func (v *ValidateBasicObject) ValidateRename(stmt *parser.RenameStmt) (*driverV2.AuditResult, error) {
	level, message := driverV2.RuleLevelNull, ""
	switch stmt.RenameType {
	case parser.ObjectType_OBJECT_INDEX:
		var err error
		var result *driverV2.AuditResult
		var isIndex bool
		ownerName, indexName := GetOwnerNameAndIndexName(stmt.Relation, v.PgContext)
		result, err = validateSchemaExist(ownerName, v)
		if err != nil {
			return nil, err
		}
		if result.Level != driverV2.RuleLevelNull {
			return result, nil
		}
		isIndex, err = v.PgContext.IsExistIndex(ownerName, "", indexName)
		if err != nil {
			return nil, err
		}
		if !isIndex {
			level = driverV2.RuleLevelWarn
			message = fmt.Sprintf(IndexNotExist, ownerName, indexName)
		} else {
			isIndex, err = v.PgContext.IsExistIndex(ownerName, "", stmt.Newname)
			if err != nil {
				if !strings.HasPrefix(err.Error(), "index not exist, index name=") {
					return nil, err
				}
				isIndex = false
			}
			if isIndex {
				level = driverV2.RuleLevelWarn
				message = fmt.Sprintf(IndexExist, ownerName, stmt.Newname)
			}
		}
	case parser.ObjectType_OBJECT_TABLE:
		var err error
		var result *driverV2.AuditResult
		var isTable bool
		ownerName, tableName := GetOwnerNameAndTableName(stmt.Relation, v.PgContext)
		result, err = validateSchemaExist(ownerName, v)
		if err != nil {
			return nil, err
		}
		if result.Level != driverV2.RuleLevelNull {
			return result, nil
		}
		// 校验当前表名，不存在报错
		isTable, err = v.PgContext.IsExistTable(ownerName, tableName)
		if err != nil {
			return nil, err
		}
		if !isTable {
			return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(TableNotExist, ownerName, tableName)), nil
		}
		// 校验新表名，存在报错
		isTable, err = v.PgContext.IsExistTable(ownerName, stmt.Newname)
		if err != nil {
			return nil, err
		}
		if isTable {
			level = driverV2.RuleLevelWarn
			return newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(TableExist, ownerName, stmt.Newname)), nil
		}
	case parser.ObjectType_OBJECT_COLUMN:
		ownerName, tableName := GetOwnerNameAndTableName(stmt.Relation, v.PgContext)
		result, err := validateSchemaExist(ownerName, v)
		if err != nil {
			return nil, err
		}
		if result.Level != driverV2.RuleLevelNull {
			return result, nil
		}
		result, err = validateTableExist(ownerName, tableName, v)
		if err != nil {
			return nil, err
		}
		if result.Level != driverV2.RuleLevelNull {
			return result, nil
		}
		if !v.PgContext.IsExistColumn(ownerName, tableName, stmt.Subname) {
			level = driverV2.RuleLevelWarn
			message = fmt.Sprintf("表[%s.%s]对应的列[%s]不存在", ownerName, tableName, stmt.Subname)
		} else {
			if v.PgContext.IsExistColumn(ownerName, tableName, stmt.Newname) {
				level = driverV2.RuleLevelWarn
				message = fmt.Sprintf("表[%s.%s]对应的列[%s]已存在", ownerName, tableName, stmt.Newname)
			}
		}
	}
	return newAuditResult(level, message), nil
}

// ValidateInsertColumns :校验insert列
func (v *ValidateBasicObject) ValidateInsertColumns(stmt *parser.Node_InsertStmt) (*driverV2.AuditResult, error) {
	level, message := driverV2.RuleLevelNull, ""
	ownerName, tableName := GetOwnerNameAndTableName(stmt.InsertStmt.Relation, v.PgContext)
	result, err := validateSchemaExist(ownerName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	result, err = validateTableExist(ownerName, tableName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	columns := make([]string, 0)
	cols := stmt.InsertStmt.Cols
	for _, col := range cols {
		columnName := col.GetResTarget().GetName()
		if !v.PgContext.IsExistColumn(ownerName, tableName, columnName) {
			columns = append(columns, columnName)
		}
	}

	if len(columns) > 0 {
		level = driverV2.RuleLevelWarn
		message = fmt.Sprintf("insert表[%s.%s]的列[%s]不存在",
			ownerName, tableName, strings.Join(columns, ","))
	}

	return newAuditResult(level, message), nil
}

// ValidateUpdateColumns :校验update列
func (v *ValidateBasicObject) ValidateUpdateColumns(stmt *parser.Node_UpdateStmt) (*driverV2.AuditResult, error) {
	msg := make([]string, 0)
	level, message := driverV2.RuleLevelNull, ""
	ownerName, tableName := GetOwnerNameAndTableName(stmt.UpdateStmt.Relation, v.PgContext)
	result, err := validateSchemaExist(ownerName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	result, err = validateTableExist(ownerName, tableName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	columns := make([]string, 0)
	targetList := stmt.UpdateStmt.TargetList
	for _, col := range targetList {
		columnName := col.GetResTarget().GetName()
		if !v.PgContext.IsExistColumn(ownerName, tableName, columnName) {
			columns = append(columns, columnName)
		}
	}

	if len(columns) > 0 {
		level = driverV2.RuleLevelWarn
		message = fmt.Sprintf("update表[%s.%s]set的列[%s]不存在", ownerName, tableName, strings.Join(columns, ","))
		return newAuditResult(level, message), nil
	}

	// 后面校验使用
	updateTables := make([]*OperatedTable, 0)
	// 表别名
	tableAliasName := ""
	if stmt.UpdateStmt.Relation.Alias != nil {
		tableAliasName = stmt.UpdateStmt.Relation.Alias.Aliasname
	}
	updateTables = append(updateTables, parseOperatedTableSlice(v, ownerName, tableName, tableAliasName)...)

	fromClauses := stmt.UpdateStmt.FromClause
	for _, fromClause := range fromClauses {
		if fromClause.GetRangeVar() == nil {
			continue
		}
		schemaName := fromClause.GetRangeVar().Schemaname
		realName := fromClause.GetRangeVar().Relname
		aliasName := ""
		if fromClause.GetRangeVar().GetAlias() != nil {
			aliasName = fromClause.GetRangeVar().GetAlias().GetAliasname()
		}
		updateTables = append(updateTables, parseOperatedTableSlice(v, schemaName, realName, aliasName)...)
	}

	// where条件列存在校验
	if stmt.UpdateStmt.WhereClause != nil && stmt.UpdateStmt.WhereClause.GetAExpr() != nil {
		aExpr := stmt.UpdateStmt.WhereClause.GetAExpr()
		lExpr, rExpr := aExpr.GetLexpr(), aExpr.GetRexpr()
		if lExpr.GetColumnRef() != nil {
			msg = append(msg, validateWhereFieldExist(lExpr, ownerName, tableName, updateTables)...)
		}

		if rExpr.GetColumnRef() != nil {
			msg = append(msg, validateWhereFieldExist(rExpr, ownerName, tableName, updateTables)...)
		}
	}

	// 递归校验列是否存在
	if stmt.UpdateStmt.WhereClause != nil {
		msg = append(msg, recursiveValidateColumn(stmt.UpdateStmt.GetWhereClause().GetBoolExpr(), msg, ownerName, tableName, updateTables)...)
	}

	if len(msg) > 0 {
		level = driverV2.RuleLevelWarn
		msg = RemoveDuplicates(msg)
		message += fmt.Sprintf("update表[%s.%s]where条件中:%s", ownerName, tableName, strings.Join(msg, ";"))
	}

	return newAuditResult(level, message), nil
}

// ValidateDeleteColumns :校验delete列
func (v *ValidateBasicObject) ValidateDeleteColumns(stmt *parser.Node_DeleteStmt) (*driverV2.AuditResult, error) {
	msg := make([]string, 0)
	level, message := driverV2.RuleLevelNull, ""
	ownerName, tableName := GetOwnerNameAndTableName(stmt.DeleteStmt.Relation, v.PgContext)
	result, err := validateSchemaExist(ownerName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}
	result, err = validateTableExist(ownerName, tableName, v)
	if err != nil {
		return nil, err
	}
	if result.Level != driverV2.RuleLevelNull {
		return result, nil
	}

	// 后面校验使用
	deleteTables := make([]*OperatedTable, 0)
	// 表别名
	tableAliasName := ""
	if stmt.DeleteStmt.Relation.Alias != nil {
		tableAliasName = stmt.DeleteStmt.Relation.Alias.Aliasname
	}
	deleteTables = append(deleteTables, parseOperatedTableSlice(v, ownerName, tableName, tableAliasName)...)

	usingClauses := stmt.DeleteStmt.GetUsingClause()
	for _, usingClause := range usingClauses {
		schemaName := usingClause.GetRangeVar().Schemaname
		realName := usingClause.GetRangeVar().Relname
		aliasName := ""
		if usingClause.GetRangeVar().GetAlias() != nil {
			aliasName = usingClause.GetRangeVar().GetAlias().GetAliasname()
		}
		deleteTables = append(deleteTables, parseOperatedTableSlice(v, schemaName, realName, aliasName)...)
	}

	if stmt.DeleteStmt.WhereClause != nil && stmt.DeleteStmt.WhereClause.GetAExpr() != nil {
		aExpr := stmt.DeleteStmt.WhereClause.GetAExpr()
		lExpr, rExpr := aExpr.GetLexpr(), aExpr.GetRexpr()
		if lExpr.GetColumnRef() != nil {
			msg = append(msg, validateWhereFieldExist(lExpr, ownerName, tableName, deleteTables)...)
		}

		if rExpr.GetColumnRef() != nil {
			msg = append(msg, validateWhereFieldExist(rExpr, ownerName, tableName, deleteTables)...)
		}
	}

	// 递归校验列是否存在
	if stmt.DeleteStmt.WhereClause != nil {
		msg = append(msg, recursiveValidateColumn(stmt.DeleteStmt.GetWhereClause().GetBoolExpr(), msg, ownerName, tableName, deleteTables)...)
	}

	if len(msg) > 0 {
		level = driverV2.RuleLevelWarn
		msg = RemoveDuplicates(msg)
		message += fmt.Sprintf("delete表[%s.%s]where条件中:%s", ownerName, tableName, strings.Join(msg, ";"))
	}

	return newAuditResult(level, message), nil
}

func RemoveDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}
	for v := range elements {
		if encountered[elements[v]] == true {
			// 已经遇到重复元素，继续下一个循环
			continue
		}
		encountered[elements[v]] = true
		result = append(result, elements[v])
	}
	return result
}

func recursiveValidateColumn(boolExpr *parser.BoolExpr, msg []string, ownerName string, tableName string, operatedTables []*OperatedTable) []string {
	if boolExpr != nil {
		args := boolExpr.GetArgs()
		for _, arg := range args {
			if arg.GetBoolExpr() != nil {
				return recursiveValidateColumn(arg.GetBoolExpr(), msg, ownerName, tableName, operatedTables)
			}
			lExpr, rExpr := arg.GetAExpr().GetLexpr(), arg.GetAExpr().GetRexpr()
			if lExpr.GetColumnRef() != nil {
				msg = append(msg, validateWhereFieldExist(lExpr, ownerName, tableName, operatedTables)...)
			}

			if rExpr.GetColumnRef() != nil {
				msg = append(msg, validateWhereFieldExist(rExpr, ownerName, tableName, operatedTables)...)
			}
		}
	}
	return msg
}

func parseOperatedTableSlice(v *ValidateBasicObject, ownerName, tableName, tableAliasName string) []*OperatedTable {
	operatedTables := make([]*OperatedTable, 0)
	if schemaInfo, ok := v.PgContext.DatabaseInfo.SchemaInfoMap[ownerName]; ok {
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName != tableName {
				continue
			}
			// 表列名数组
			columns := make([]string, 0)
			columnInfoList := tableInfo.ColumnInfoList
			for _, columnInfo := range columnInfoList {
				columns = append(columns, columnInfo.ColumnName)
			}
			operatedTables = append(operatedTables, &OperatedTable{
				schema:     ownerName,
				tableName:  tableName,
				tableAlias: tableAliasName,
				columns:    columns,
			})
		}
	}
	return operatedTables
}

func validateWhereFieldExist(expr *parser.Node, ownerName, tableName string, operatedTables []*OperatedTable) []string {
	msg := make([]string, 0)
	if len(operatedTables) == 0 {
		return msg
	}
	schema := ownerName
	table := tableName
	columnName := ""
	fields := expr.GetColumnRef().GetFields()
	if len(fields) == 1 {
		columnName = fields[0].GetString_().GetStr()
	} else if len(fields) == 2 {
		table = fields[0].GetString_().GetStr()
		columnName = fields[1].GetString_().GetStr()
	} else if len(fields) == 3 {
		schema = fields[0].GetString_().GetStr()
		table = fields[1].GetString_().GetStr()
		columnName = fields[2].GetString_().GetStr()
	} else {
		columnName = fields[len(fields)-1].GetString_().GetStr()
	}

	for _, operatedTable := range operatedTables {
		if operatedTable.schema != schema || (operatedTable.tableName != table && operatedTable.tableAlias != table) {
			continue
		}
		isExistColumn := false
		columns := operatedTable.columns
		for _, column := range columns {
			if column == columnName {
				isExistColumn = true
				break
			}
		}

		if !isExistColumn {
			msg = append(msg, fmt.Sprintf("表[%s.%s]列[%s]不存在", schema, operatedTable.tableName, columnName))
		}
	}
	return msg
}

func validateDmlSchemaAndTableExist(subQueries []*parser.SelectStmt, v *ValidateBasicObject) (*driverV2.AuditResult, error) {
	auditResult := newAuditResult(driverV2.RuleLevelNull, "")
	message := make([]string, 0)
	schemaMap := make(map[string]string)
	tableNameMap := make(map[string]string)
	withTablesMap := make(map[string]string)
	for _, subQuery := range subQueries {
		resultTablesMap := parseWithTables(subQuery.WithClause)
		for key, val := range resultTablesMap {
			if _, ok := withTablesMap[key]; !ok {
				withTablesMap[key] = val
			}
		}
		fromClause := subQuery.FromClause
		for _, from := range fromClause {
			// 校验from后的表名和对应的schema
			if from.GetRangeVar() != nil {
				result, err := validateSchemaAndTableNameForDmlFromClause(from.GetRangeVar(), v, schemaMap, withTablesMap, tableNameMap)
				if err != nil {
					return result, err
				}
			}

			// 校验join连接的表名和对应的schema
			if from.GetJoinExpr() != nil {
				larg, rarg := from.GetJoinExpr().GetLarg(), from.GetJoinExpr().GetRarg()
				if larg != nil && larg.GetRangeVar() != nil {
					result, err := validateSchemaAndTableNameForDmlFromClause(larg.GetRangeVar(), v, schemaMap, withTablesMap, tableNameMap)
					if err != nil {
						return result, err
					}
				}
				if rarg != nil && rarg.GetRangeVar() != nil {
					result, err := validateSchemaAndTableNameForDmlFromClause(rarg.GetRangeVar(), v, schemaMap, withTablesMap, tableNameMap)
					if err != nil {
						return result, err
					}
				}
			}
		}
	}

	if len(schemaMap) > 0 {
		var keys []string
		for k := range schemaMap {
			keys = append(keys, k)
		}
		message = append(message, fmt.Sprintf(SchemaNotExist, strings.Join(keys, ",")))
	}

	if len(tableNameMap) > 0 {
		var keys []string
		for k := range tableNameMap {
			keys = append(keys, k)
		}
		message = append(message, fmt.Sprintf("表[%s]不存在", strings.Join(keys, ",")))
	}

	if len(message) > 0 {
		auditResult = newAuditResult(driverV2.RuleLevelWarn, strings.Join(message, ";"))
	}
	return auditResult, nil
}

func validateSchemaAndTableNameForDmlFromClause(rangeVar *parser.RangeVar, v *ValidateBasicObject, schemaMap map[string]string, withTablesMap map[string]string, tableNameMap map[string]string) (*driverV2.AuditResult, error) {
	auditResult := newAuditResult(driverV2.RuleLevelNull, "")
	if rangeVar != nil {
		schemaName := rangeVar.Schemaname
		if len(schemaName) == 0 {
			schemaName = v.PgContext.DatabaseInfo.CurrentSchema
		}
		var err error
		var isSchema, isTable bool
		tableName := rangeVar.Relname
		isSchema, err = v.PgContext.IsExistSchema(v.PgContext.CurrentDatabase, schemaName)
		if err != nil {
			return auditResult, err
		}
		if !isSchema {
			_, ok := schemaMap[schemaName]
			if !ok {
				schemaMap[schemaName] = schemaName
			}
			return auditResult, fmt.Errorf(SchemaNotExist, schemaName)
		}
		// with 表名存在，跳过后面的校验
		if _, ok := withTablesMap[tableName]; ok {
			return auditResult, nil
		}
		isTable, err = v.PgContext.IsExistTable(schemaName, tableName)
		if err != nil {
			return auditResult, err
		}
		if !isTable {
			tableNameKey := fmt.Sprintf("%s.%s", schemaName, tableName)
			_, ok := tableNameMap[tableNameKey]
			if !ok {
				tableNameMap[tableNameKey] = tableNameKey
			}
		}
	}
	return auditResult, nil
}

func parseWithTables(withClause *parser.WithClause) map[string]string {
	resultMap := make(map[string]string)
	if withClause == nil {
		return resultMap
	}
	ctes := withClause.Ctes
	for _, cte := range ctes {
		commonTableExpr := cte.GetCommonTableExpr()
		if commonTableExpr == nil {
			continue
		}
		cteName := commonTableExpr.GetCtename()
		if len(cteName) == 0 {
			continue
		}
		if _, ok := resultMap[cteName]; !ok {
			resultMap[cteName] = cteName
		}
	}
	return resultMap
}

// 校验dml schema和table名是否存在 :validateDmlSchemaNameAndTableNameExist
func validateDmlSchemaNameAndTableNameExist(relation *parser.RangeVar, v *ValidateBasicObject) (*driverV2.AuditResult, error) {
	var err error
	var isSchema, isTable bool
	auditResult := newAuditResult(driverV2.RuleLevelNull, "")
	schemaName := relation.Schemaname
	tableName := relation.Relname
	if len(schemaName) == 0 {
		schemaName = v.PgContext.DatabaseInfo.CurrentSchema
	}
	isSchema, err = v.PgContext.IsExistSchema(v.PgContext.CurrentDatabase, schemaName)
	if err != nil {
		return auditResult, err
	}
	if !isSchema {
		auditResult = newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(SchemaNotExist, schemaName))
		return auditResult, nil
	}
	isTable, err = v.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return auditResult, err
	}
	if !isTable {
		auditResult = newAuditResult(driverV2.RuleLevelWarn, fmt.Sprintf(TableNotExist, schemaName, tableName))
		return auditResult, nil
	}
	return auditResult, nil
}

func validateSchemaExist(schemaName string, v *ValidateBasicObject) (*driverV2.AuditResult, error) {
	result := newAuditResult(driverV2.RuleLevelNull, "")
	isSchema, err := v.PgContext.IsExistSchema(v.PgContext.CurrentDatabase, schemaName)
	if err != nil {
		return nil, err
	}
	if !isSchema {
		result.Level = driverV2.RuleLevelWarn
		result.I18nAuditResultInfo[i18nPkg.DefaultLang] = driverV2.AuditResultInfo{Message: fmt.Sprintf(SchemaNotExist, schemaName)}
		return result, nil
	}
	return result, nil
}

func validateTableExist(schemaName, tableName string, v *ValidateBasicObject) (*driverV2.AuditResult, error) {
	result := newAuditResult(driverV2.RuleLevelNull, "")
	isTable, err := v.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	if !isTable {
		result.Level = driverV2.RuleLevelWarn
		result.I18nAuditResultInfo[i18nPkg.DefaultLang] = driverV2.AuditResultInfo{Message: fmt.Sprintf(TableNotExist, schemaName, tableName)}
		return result, nil
	}
	return result, nil
}
