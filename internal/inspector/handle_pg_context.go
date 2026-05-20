package inspector

import (
	"fmt"
	"github.com/actiontech/sqle-pg-plugin/internal/utils"
	parser "github.com/pganalyze/pg_query_go/v2"
	"strings"
)

type HandlePgContext struct {
	PgContext *PgContext
	RawStmt   *parser.RawStmt
}

// Handle :处理上下文方法
func (h *HandlePgContext) Handle() error {
	switch stmt := h.RawStmt.GetStmt().GetNode().(type) {
	case *parser.Node_CreateSchemaStmt:
		err := h.handleCreateSchema(h.PgContext.CurrentDatabase, stmt.CreateSchemaStmt.Schemaname)
		if err != nil {
			return err
		}
	case *parser.Node_DropStmt:
		err := dropStatement(stmt.DropStmt, h)
		if err != nil {
			return err
		}
	case *parser.Node_CreateStmt:
		relation := stmt.CreateStmt.Relation
		ownerName, tableName := GetOwnerNameAndTableName(relation, h.PgContext)
		tableInfo := &TableInfo{OwnerName: ownerName, TableName: tableName}
		err := h.handleCreateTable(tableInfo, &stmt.CreateStmt.TableElts)
		if err != nil {
			return err
		}
	case *parser.Node_AlterTableStmt:
		err := alterTableStatement(stmt.AlterTableStmt, h)
		if err != nil {
			return err
		}
	case *parser.Node_IndexStmt:
		err := indexStatement(stmt.IndexStmt, h)
		if err != nil {
			return err
		}
	case *parser.Node_RenameStmt:
		err := renameStatement(stmt.RenameStmt, h)
		if err != nil {
			return err
		}
	}
	return nil
}

// handleCreateSchema :处理创建schema
func (h *HandlePgContext) handleCreateSchema(database, schemaName string) error {
	isExist, err := h.PgContext.IsExistSchema(database, schemaName)
	if err != nil {
		return err
	}
	if !isExist {
		h.PgContext.AddSchema(schemaName)
		if _, ok := h.PgContext.DeletedSchemaMap[schemaName]; ok {
			delete(h.PgContext.DeletedSchemaMap, schemaName)
		}
	}
	return nil
}

// handleDropSchema :处理删除schema
func (h *HandlePgContext) handleDropSchema(database, schemaName string) error {
	isExist, err := h.PgContext.IsExistSchema(database, schemaName)
	if err != nil {
		return err
	}
	if isExist {
		h.PgContext.RemoveSchema(schemaName)
		if _, ok := h.PgContext.DeletedSchemaMap[schemaName]; !ok {
			h.PgContext.DeletedSchemaMap[schemaName] = schemaName
		}
	}
	return nil
}

// handleCreateTable :处理创建表
func (h *HandlePgContext) handleCreateTable(tableInfo *TableInfo, tableElts *[]*parser.Node) error {
	columnInfoList := make([]*ColumnInfo, 0)
	constraintList := make([]*ConstraintInfo, 0)
	indexInfoList := make([]*IndexInfo, 0)
	hasPrimaryKey := false
	uniqueIndexColumnList := make([]string, 0)
	for _, tableElt := range *tableElts {
		isPrimaryColumn, isUnique, isNullable := false, false, true
		switch def := tableElt.GetNode().(type) {
		case *parser.Node_ColumnDef:
			for _, constraint := range def.ColumnDef.Constraints {
				c := constraint.GetConstraint()
				if c == nil {
					continue
				}
				switch c.Contype {
				case parser.ConstrType_CONSTR_PRIMARY:
					isPrimaryColumn = true
					hasPrimaryKey = true
					uniqueIndexColumnList = append(uniqueIndexColumnList, def.ColumnDef.Colname)
				case parser.ConstrType_CONSTR_UNIQUE:
					isUnique = true
				case parser.ConstrType_CONSTR_NOTNULL:
					isNullable = false
				}
			}
		case *parser.Node_Constraint:
			// 含有主键约束
			if tableElt.GetConstraint().Contype == parser.ConstrType_CONSTR_PRIMARY {
				hasPrimaryKey = true
				keys := tableElt.GetConstraint().GetKeys()
				for _, key := range keys {
					uniqueIndexColumnList = append(uniqueIndexColumnList, key.GetString_().GetStr())
				}
			}
			// 获取唯一索引
			if tableElt.GetConstraint().Contype == parser.ConstrType_CONSTR_UNIQUE {
				columns := make([]string, 0)
				keys := tableElt.GetConstraint().GetKeys()
				for _, key := range keys {
					columns = append(columns, key.GetString_().GetStr())
				}
				indexInfoList = append(indexInfoList, &IndexInfo{
					IndexName:  tableElt.GetConstraint().Conname,
					OwnerName:  tableInfo.OwnerName,
					TableName:  tableInfo.TableName,
					ColumnList: columns,
					IsUnique:   true,
				})
			}
			// 获取建表时的约束
			constraints, err := createTableConstraints(tableElt, h)
			if err != nil {
				return err
			}
			constraintList = append(constraintList, constraints)
		}
		columnLength, columnPrecision := 0, 0
		typMods := tableElt.GetColumnDef().GetTypeName().GetTypmods()
		if typMods != nil {
			columnLength = int(typMods[0].GetAConst().GetVal().GetInteger().GetIval())
			if len(typMods) == 2 {
				columnPrecision = int(typMods[1].GetAConst().GetVal().GetInteger().GetIval())
			}
		}
		if tableElt.GetColumnDef() != nil {
			columnType := ""
			typeNames := tableElt.GetColumnDef().GetTypeName().GetNames()
			if len(typeNames) == 1 {
				columnType = typeNames[0].GetString_().GetStr()
			} else if len(typeNames) == 2 {
				columnType = typeNames[1].GetString_().GetStr()
			}
			columnInfoList = append(columnInfoList, &ColumnInfo{
				ColumnName:      tableElt.GetColumnDef().Colname,
				OwnerName:       tableInfo.OwnerName,
				TableName:       tableInfo.TableName,
				ColumnType:      columnType,
				ColumnLength:    columnLength,
				ColumnPrecision: columnPrecision,
				IsNullable:      isNullable,
				IsPrimaryColumn: isPrimaryColumn,
				IsUnique:        isUnique,
			})
			// 将列数据类型加到上下文PgDbTypeNameMap中
			if _, ok := h.PgContext.PgDbTypeNameMap[columnType]; !ok {
				h.PgContext.PgDbTypeNameMap[columnType] = columnType
			}
		}
	}
	tableInfo.ColumnInfoList = columnInfoList
	tableInfo.ConstraintList = constraintList
	err := h.PgContext.AddTable(tableInfo)
	if err != nil {
		return err
	}
	// 清除已删除的表信息
	deletedTableName := fmt.Sprintf(TwoFormatString, tableInfo.OwnerName, tableInfo.TableName)
	if _, ok := h.PgContext.DeletedTableMap[deletedTableName]; ok {
		delete(h.PgContext.DeletedTableMap, deletedTableName)
	}
	// 清除已删除的列信息
	for _, columnInfo := range tableInfo.ColumnInfoList {
		deletedColumn := fmt.Sprintf(ThreeFormatString, columnInfo.OwnerName, columnInfo.TableName, columnInfo.ColumnName)
		if _, ok := h.PgContext.DeletedColumnMap[deletedColumn]; ok {
			delete(h.PgContext.DeletedColumnMap, deletedColumn)
		}
	}
	// 清除已删除的约束信息
	for _, constraintInfo := range tableInfo.ConstraintList {
		deletedConstraint := fmt.Sprintf(TwoFormatString, constraintInfo.OwnerName, constraintInfo.ConstraintName)
		if _, ok := h.PgContext.DeletedConstraintMap[deletedConstraint]; ok {
			delete(h.PgContext.DeletedConstraintMap, deletedConstraint)
		}
	}
	// 添加索引
	for _, indexInfo := range indexInfoList {
		err := h.PgContext.AddIndex(indexInfo)
		if err != nil {
			return err
		}
		deletedIndexName := fmt.Sprintf(TwoFormatString, indexInfo.OwnerName, indexInfo.IndexName)
		if _, ok := h.PgContext.DeletedIndexMap[deletedIndexName]; ok {
			delete(h.PgContext.DeletedIndexMap, deletedIndexName)
		}
	}

	// 如果含有主键，那么生成一个索引名为“表名_pkey”的唯一索引
	if hasPrimaryKey {
		indexInfo := &IndexInfo{
			IndexName:  fmt.Sprintf("%s_pkey", tableInfo.TableName),
			OwnerName:  tableInfo.OwnerName,
			TableName:  tableInfo.TableName,
			IsUnique:   true,
			ColumnList: uniqueIndexColumnList,
		}
		err := h.PgContext.AddIndex(indexInfo)
		if err != nil {
			return err
		}
		deletedIndexName := fmt.Sprintf(TwoFormatString, indexInfo.OwnerName, indexInfo.IndexName)
		if _, ok := h.PgContext.DeletedIndexMap[deletedIndexName]; ok {
			delete(h.PgContext.DeletedIndexMap, deletedIndexName)
		}
	}
	return nil
}

// handleRenameTable :处理重命名表
func (h *HandlePgContext) handleRenameTable(schemaName, tableName, newTableName string) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	newTable, err := h.PgContext.IsExistTable(schemaName, newTableName)
	if err != nil {
		return err
	}
	if table && !newTable {
		h.PgContext.RenameTable(schemaName, tableName, newTableName)
		deletedTableName := fmt.Sprintf(TwoFormatString, schemaName, tableName)
		if _, ok := h.PgContext.DeletedTableMap[deletedTableName]; !ok {
			h.PgContext.DeletedTableMap[deletedTableName] = deletedTableName
		}
	}
	return nil
}

// handleDropTable :处理删除表
func (h *HandlePgContext) handleDropTable(schemaName, tableName string) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		h.PgContext.DeleteTable(schemaName, tableName)
		deletedTableName := fmt.Sprintf(TwoFormatString, schemaName, tableName)
		if _, ok := h.PgContext.DeletedTableMap[deletedTableName]; !ok {
			h.PgContext.DeletedTableMap[deletedTableName] = deletedTableName
		}
	}
	return err
}

// handleAddColumn :处理添加列
func (h *HandlePgContext) handleAddColumn(schemaName, tableName string, columnInfo *ColumnInfo) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		columnInfoList := make([]*ColumnInfo, 0)
		columnInfoList = append(columnInfoList, columnInfo)
		err := h.PgContext.AddColumns(schemaName, tableName, &columnInfoList)
		if err != nil {
			return err
		}
		for _, column := range columnInfoList {
			// 将列数据类型加到上下文PgDbTypeNameMap中
			if _, ok := h.PgContext.PgDbTypeNameMap[column.ColumnType]; !ok {
				h.PgContext.PgDbTypeNameMap[column.ColumnType] = column.ColumnType
			}
			deletedColumn := fmt.Sprintf(ThreeFormatString, schemaName, tableName, column.ColumnName)
			if _, ok := h.PgContext.DeletedColumnMap[deletedColumn]; ok {
				delete(h.PgContext.DeletedColumnMap, deletedColumn)
			}
		}
	}
	return nil
}

// handleAlterColumnType :处理修改列类型
func (h *HandlePgContext) handleAlterColumnType(schemaName, tableName string, columnInfo *ColumnInfo) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		err := h.PgContext.AlterColumnType(schemaName, tableName, columnInfo)
		if err != nil {
			return err
		}
		// 将列数据类型加到上下文PgDbTypeNameMap中
		if _, ok := h.PgContext.PgDbTypeNameMap[columnInfo.ColumnType]; !ok {
			h.PgContext.PgDbTypeNameMap[columnInfo.ColumnType] = columnInfo.ColumnType
		}
	}
	return nil
}

// handleAlterColumnSetNotNull :处理修改列为not null
func (h *HandlePgContext) handleAlterColumnSetNotNull(schemaName, tableName, columnName string, isSetNotNull bool) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		err := h.PgContext.AlterColumnNotNull(schemaName, tableName, columnName, isSetNotNull)
		if err != nil {
			return err
		}
	}
	return nil
}

// handleRenameColumn :处理重命名列
func (h *HandlePgContext) handleRenameColumn(schemaName, tableName, columnName, newColumnName string) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		isSuccess := h.PgContext.RenameColumn(schemaName, tableName, columnName, newColumnName)
		if !isSuccess {
			return nil
		}
		deletedColumn := fmt.Sprintf(ThreeFormatString, schemaName, tableName, columnName)
		if _, ok := h.PgContext.DeletedColumnMap[deletedColumn]; !ok {
			h.PgContext.DeletedColumnMap[deletedColumn] = deletedColumn
		}
	}
	return nil
}

// handleRenameConstraint :处理重命名约束
func (h *HandlePgContext) handleRenameConstraint(schemaName, tableName, constraintName, newConstraintName string) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		isSuccess := h.PgContext.RenameConstraint(schemaName, tableName, constraintName, newConstraintName)
		if !isSuccess {
			return nil
		}
		deletedConstraint := fmt.Sprintf(TwoFormatString, schemaName, constraintName)
		if _, ok := h.PgContext.DeletedConstraintMap[deletedConstraint]; !ok {
			h.PgContext.DeletedConstraintMap[deletedConstraint] = deletedConstraint
		}
	}
	return nil
}

// handleDropColumn :处理删除列
func (h *HandlePgContext) handleDropColumn(schemaName, tableName, columnName string) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		err := h.PgContext.RemoveColumns(schemaName, tableName, &[]string{columnName})
		if err != nil {
			return err
		}
		deletedColumn := fmt.Sprintf(ThreeFormatString, schemaName, tableName, columnName)
		if _, ok := h.PgContext.DeletedColumnMap[deletedColumn]; !ok {
			h.PgContext.DeletedColumnMap[deletedColumn] = deletedColumn
		}
	}
	return nil
}

// handleAddConstraint :处理添加约束
func (h *HandlePgContext) handleAddConstraint(schemaName, tableName string, constraintList *[]*ConstraintInfo) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		err := h.PgContext.AddConstraints(schemaName, tableName, constraintList)
		if err != nil {
			return err
		}
		for _, constraint := range *constraintList {
			deletedConstraint := fmt.Sprintf(TwoFormatString, schemaName, constraint)
			if _, ok := h.PgContext.DeletedConstraintMap[deletedConstraint]; ok {
				delete(h.PgContext.DeletedConstraintMap, deletedConstraint)
			}
		}
	}
	return nil
}

// handleDropConstraint :处理删除约束
func (h *HandlePgContext) handleDropConstraint(schemaName, tableName, constraintName string) error {
	table, err := h.PgContext.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		err := h.PgContext.RemoveConstraints(schemaName, tableName, &[]string{constraintName})
		if err != nil {
			return err
		}
		deletedConstraint := fmt.Sprintf(TwoFormatString, schemaName, constraintName)
		if _, ok := h.PgContext.DeletedConstraintMap[deletedConstraint]; !ok {
			h.PgContext.DeletedConstraintMap[deletedConstraint] = deletedConstraint
		}
	}
	return nil
}

// handleCreateIndex :处理创建索引
func (h *HandlePgContext) handleCreateIndex(indexInfo *IndexInfo) error {
	index, err := h.PgContext.IsExistIndex(indexInfo.OwnerName, indexInfo.TableName, indexInfo.IndexName)
	if err != nil {
		return err
	}
	if !index {
		err := h.PgContext.AddIndex(indexInfo)
		if err != nil {
			return err
		}
		deletedIndex := fmt.Sprintf(TwoFormatString, indexInfo.OwnerName, indexInfo.IndexName)
		if _, ok := h.PgContext.DeletedIndexMap[deletedIndex]; ok {
			delete(h.PgContext.DeletedIndexMap, deletedIndex)
		}
	}
	return nil
}

// handleRenameIndex :处理重命名索引
func (h *HandlePgContext) handleRenameIndex(schemaName, indexName, newIndexName string) error {
	index, err := h.PgContext.IsExistIndex(schemaName, "", indexName)
	if err != nil {
		return err
	}
	newIndex, err := h.PgContext.IsExistIndex(schemaName, "", newIndexName)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "index not exist, index name=") {
			return err
		}
		newIndex = false
	}
	if index && !newIndex {
		isSuccess := h.PgContext.RenameIndex(schemaName, indexName, newIndexName)
		if !isSuccess {
			return nil
		}
		deletedIndex := fmt.Sprintf(TwoFormatString, schemaName, indexName)
		if _, ok := h.PgContext.DeletedIndexMap[deletedIndex]; !ok {
			h.PgContext.DeletedIndexMap[deletedIndex] = deletedIndex
		}
	}
	return nil
}

// handleDropIndex :处理删除索引
func (h *HandlePgContext) handleDropIndex(schemaName, indexName string) error {
	index, err := h.PgContext.IsExistIndex(schemaName, "", indexName)
	if err != nil {
		return err
	}
	if index {
		h.PgContext.DeleteIndex(schemaName, indexName)
		deletedIndex := fmt.Sprintf(TwoFormatString, schemaName, indexName)
		if _, ok := h.PgContext.DeletedIndexMap[deletedIndex]; !ok {
			h.PgContext.DeletedIndexMap[deletedIndex] = deletedIndex
		}
	}
	return nil
}

// createTableConstraints :创建表约束
func createTableConstraints(def *parser.Node, h *HandlePgContext) (*ConstraintInfo, error) {
	if def == nil || def.GetConstraint() == nil {
		return nil, fmt.Errorf("onstraint not exists")
	}
	constraintType, referencedTable, checkCondition, constraintContent := "", "", "", ""
	columns, referencedColumnList := make([]string, 0), make([]string, 0)
	constraintName := def.GetConstraint().Conname
	if def.GetConstraint().Contype == parser.ConstrType_CONSTR_PRIMARY ||
		def.GetConstraint().Contype == parser.ConstrType_CONSTR_UNIQUE {
		keys := def.GetConstraint().GetKeys()
		for _, key := range keys {
			columns = append(columns, key.GetString_().GetStr())
		}
	}
	fkColumns, pkColumns := make([]string, 0), make([]string, 0)
	ownerName, pkTableName := "", ""
	if def.GetConstraint().Contype == parser.ConstrType_CONSTR_FOREIGN {
		fkAttrs := def.GetConstraint().GetFkAttrs()
		for _, fkAttr := range fkAttrs {
			fkColumns = append(fkColumns, fkAttr.GetString_().GetStr())
		}
		pkAttrs := def.GetConstraint().GetPkAttrs()
		for _, pkAttr := range pkAttrs {
			pkColumns = append(pkColumns, pkAttr.GetString_().GetStr())
		}
		ownerName = def.GetConstraint().GetPktable().GetSchemaname()
		if len(ownerName) == 0 {
			ownerName = h.PgContext.DatabaseInfo.CurrentSchema
		}
		pkTableName = def.GetConstraint().GetPktable().GetRelname()
	}
	switch def.GetConstraint().Contype {
	case parser.ConstrType_CONSTR_PRIMARY:
		constraintType = ConstraintTypeP
		constraintContent = fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)",
			constraintName, strings.Join(columns, ","))
	case parser.ConstrType_CONSTR_UNIQUE:
		constraintType = ConstraintTypeU
		constraintContent = fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)",
			constraintName, strings.Join(columns, ","))
	case parser.ConstrType_CONSTR_FOREIGN:
		constraintType = ConstraintTypeF
		referencedTable = fmt.Sprintf("%s.%s", ownerName, pkTableName)
		referencedColumnList = pkColumns
		constraintContent = fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s (%s)",
			constraintName, strings.Join(fkColumns, ","),
			ownerName, pkTableName, strings.Join(pkColumns, ","))
	case parser.ConstrType_CONSTR_CHECK:
		constraintType = ConstraintTypeC
		condition := utils.ParseCondition(def.GetConstraint().RawExpr)
		checkCondition = condition
		constraintContent = fmt.Sprintf("CONSTRAINT %s CHECK (%s)", constraintName, condition)
	}
	return &ConstraintInfo{
		ConstraintName:    constraintName,
		ConstraintType:    constraintType,
		OwnerName:         ownerName,
		ColumnList:        columns,
		ReferencedTable:   referencedTable,
		ReferencedColumns: referencedColumnList,
		CheckCondition:    checkCondition,
		ConstraintContent: constraintContent,
	}, nil
}

func dropStatement(stmt *parser.DropStmt, h *HandlePgContext) error {
	switch stmt.RemoveType {
	case parser.ObjectType_OBJECT_SCHEMA:
		objects := stmt.Objects
		for _, object := range objects {
			schemaName := object.GetString_().GetStr()
			err := h.handleDropSchema(h.PgContext.CurrentDatabase, schemaName)
			if err != nil {
				return err
			}
			if _, ok := h.PgContext.DeletedSchemaMap[schemaName]; !ok {
				h.PgContext.DeletedSchemaMap[schemaName] = schemaName
			}
		}
	case parser.ObjectType_OBJECT_TABLE:
		objects := stmt.Objects
		for _, object := range objects {
			schemaName, tableName := "", ""
			items := object.GetList().GetItems()
			if len(items) == 1 {
				schemaName = h.PgContext.DatabaseInfo.CurrentSchema
				tableName = items[0].GetString_().GetStr()
			} else if len(items) == 2 {
				schemaName = items[0].GetString_().GetStr()
				tableName = items[1].GetString_().GetStr()
			}
			err := h.handleDropTable(schemaName, tableName)
			if err != nil {
				return err
			}
			deletedTableName := fmt.Sprintf(TwoFormatString, schemaName, tableName)
			if _, ok := h.PgContext.DeletedTableMap[deletedTableName]; !ok {
				h.PgContext.DeletedTableMap[deletedTableName] = deletedTableName
			}
		}
	case parser.ObjectType_OBJECT_INDEX:
		objects := stmt.Objects
		for _, object := range objects {
			schemaName, indexName := "", ""
			items := object.GetList().GetItems()
			if len(items) == 1 {
				schemaName = h.PgContext.DatabaseInfo.CurrentSchema
				indexName = items[0].GetString_().GetStr()
			} else if len(items) == 2 {
				schemaName = items[0].GetString_().GetStr()
				indexName = items[1].GetString_().GetStr()
			}
			err := h.handleDropIndex(schemaName, indexName)
			if err != nil {
				return err
			}
			deletedIndexName := fmt.Sprintf(TwoFormatString, schemaName, indexName)
			if _, ok := h.PgContext.DeletedIndexMap[deletedIndexName]; !ok {
				h.PgContext.DeletedIndexMap[deletedIndexName] = deletedIndexName
			}
		}
	}
	return nil
}

func alterTableStatement(stmt *parser.AlterTableStmt, h *HandlePgContext) error {
	ownerName, tableName := GetOwnerNameAndTableName(stmt.Relation, h.PgContext)
	for _, cmd := range stmt.GetCmds() {
		switch cmd.GetAlterTableCmd().GetSubtype() {
		case parser.AlterTableType_AT_AddColumn:
			columnInfo := GetColumnInfo(ownerName, tableName, cmd.GetAlterTableCmd().GetDef().GetColumnDef())
			err := h.handleAddColumn(ownerName, tableName, columnInfo)
			if err != nil {
				return err
			}
			deletedColumn := fmt.Sprintf(ThreeFormatString, ownerName, tableName, columnInfo.ColumnName)
			if _, ok := h.PgContext.DeletedColumnMap[deletedColumn]; ok {
				delete(h.PgContext.DeletedColumnMap, deletedColumn)
			}
		case parser.AlterTableType_AT_AddConstraint:
			constraintList := make([]*ConstraintInfo, 0)
			// 获取约束
			constraint, err := createTableConstraints(cmd.GetAlterTableCmd().Def, h)
			if err != nil {
				return err
			}
			constraintList = append(constraintList, constraint)
			err = h.handleAddConstraint(ownerName, tableName, &constraintList)
			if err != nil {
				return err
			}
			// 处理主键约束和唯一索引生存索引存入pg上下文中
			if constraint != nil {
				switch constraint.ConstraintType {
				case ConstraintTypeP:
					// 生成索引名为“表名_pkey”的唯一索引
					indexInfo := &IndexInfo{
						IndexName:  fmt.Sprintf("%s_pkey", tableName),
						OwnerName:  ownerName,
						TableName:  tableName,
						IsUnique:   true,
						ColumnList: constraint.ColumnList,
					}
					err := h.PgContext.AddIndex(indexInfo)
					if err != nil {
						return err
					}
					deletedIndex := fmt.Sprintf(TwoFormatString, indexInfo.OwnerName, indexInfo.IndexName)
					if _, ok := h.PgContext.DeletedIndexMap[deletedIndex]; ok {
						delete(h.PgContext.DeletedIndexMap, deletedIndex)
					}
				case ConstraintTypeU:
					// 生成索引名为“表名_列名1,列名2..._key”的唯一索引
					columns := strings.Join(constraint.ColumnList, "_")
					indexInfo := &IndexInfo{
						IndexName:  fmt.Sprintf("%s_%s_key", tableName, columns),
						OwnerName:  ownerName,
						TableName:  tableName,
						IsUnique:   true,
						ColumnList: constraint.ColumnList,
					}
					err := h.PgContext.AddIndex(indexInfo)
					if err != nil {
						return err
					}
					deletedIndex := fmt.Sprintf(TwoFormatString, indexInfo.OwnerName, indexInfo.IndexName)
					if _, ok := h.PgContext.DeletedIndexMap[deletedIndex]; ok {
						delete(h.PgContext.DeletedIndexMap, deletedIndex)
					}
				}
			}
		case parser.AlterTableType_AT_AlterColumnType:
			alterColumnName := cmd.GetAlterTableCmd().GetName()
			columnInfo := GetColumnInfo(ownerName, tableName, cmd.GetAlterTableCmd().GetDef().GetColumnDef())
			columnInfo.ColumnName = alterColumnName
			err := h.handleAlterColumnType(ownerName, tableName, columnInfo)
			if err != nil {
				return err
			}
		case parser.AlterTableType_AT_SetNotNull:
			alterColumnName := cmd.GetAlterTableCmd().GetName()
			err := h.handleAlterColumnSetNotNull(ownerName, tableName, alterColumnName, true)
			if err != nil {
				return err
			}
		case parser.AlterTableType_AT_DropColumn:
			err := h.handleDropColumn(ownerName, tableName, cmd.GetAlterTableCmd().GetName())
			if err != nil {
				return err
			}
			deletedColumn := fmt.Sprintf(ThreeFormatString, ownerName, tableName, cmd.GetAlterTableCmd().GetName())
			if _, ok := h.PgContext.DeletedColumnMap[deletedColumn]; !ok {
				h.PgContext.DeletedColumnMap[deletedColumn] = deletedColumn
			}
		case parser.AlterTableType_AT_DropConstraint:
			err := h.handleDropConstraint(ownerName, tableName, cmd.GetAlterTableCmd().GetName())
			if err != nil {
				return err
			}
			deletedConstraint := fmt.Sprintf(TwoFormatString, ownerName, cmd.GetAlterTableCmd().GetName())
			if _, ok := h.PgContext.DeletedConstraintMap[deletedConstraint]; !ok {
				h.PgContext.DeletedConstraintMap[deletedConstraint] = deletedConstraint
			}
		case parser.AlterTableType_AT_DropNotNull:
			alterColumnName := cmd.GetAlterTableCmd().GetName()
			err := h.handleAlterColumnSetNotNull(ownerName, tableName, alterColumnName, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func indexStatement(stmt *parser.IndexStmt, h *HandlePgContext) error {
	indexName := stmt.GetIdxname()
	schemaName, tableName := GetOwnerNameAndTableName(stmt.Relation, h.PgContext)
	columns := make([]string, 0)
	indexParams := stmt.IndexParams
	for _, indexParam := range indexParams {
		columns = append(columns, indexParam.GetIndexElem().GetName())
	}
	indexInfo := &IndexInfo{IndexName: indexName, OwnerName: schemaName, TableName: tableName, ColumnList: columns}
	if stmt.GetUnique() {
		indexInfo.IsUnique = true
	}
	if len(columns) > 0 {
		err := h.handleCreateIndex(indexInfo)
		if err != nil {
			return err
		}
		deletedIndex := fmt.Sprintf(TwoFormatString, indexInfo.OwnerName, indexInfo.IndexName)
		if _, ok := h.PgContext.DeletedIndexMap[deletedIndex]; ok {
			delete(h.PgContext.DeletedIndexMap, deletedIndex)
		}
	}
	return nil
}

func renameStatement(stmt *parser.RenameStmt, h *HandlePgContext) error {
	switch stmt.RenameType {
	case parser.ObjectType_OBJECT_INDEX:
		ownerName, indexName := GetOwnerNameAndIndexName(stmt.Relation, h.PgContext)
		err := h.handleRenameIndex(ownerName, indexName, stmt.Newname)
		if err != nil {
			return err
		}
		deletedIndex := fmt.Sprintf(TwoFormatString, ownerName, indexName)
		if _, ok := h.PgContext.DeletedIndexMap[deletedIndex]; !ok {
			h.PgContext.DeletedIndexMap[deletedIndex] = deletedIndex
		}
		deletedNewIndex := fmt.Sprintf(TwoFormatString, ownerName, stmt.Newname)
		if _, ok := h.PgContext.DeletedIndexMap[deletedNewIndex]; ok {
			delete(h.PgContext.DeletedIndexMap, deletedNewIndex)
		}
	case parser.ObjectType_OBJECT_TABLE:
		ownerName, tableName := GetOwnerNameAndTableName(stmt.Relation, h.PgContext)
		err := h.handleRenameTable(ownerName, tableName, stmt.Newname)
		if err != nil {
			return err
		}
		deletedTable := fmt.Sprintf(TwoFormatString, ownerName, tableName)
		if _, ok := h.PgContext.DeletedTableMap[deletedTable]; !ok {
			h.PgContext.DeletedTableMap[deletedTable] = deletedTable
		}
		deletedNewTable := fmt.Sprintf(TwoFormatString, ownerName, stmt.Newname)
		if _, ok := h.PgContext.DeletedTableMap[deletedNewTable]; ok {
			delete(h.PgContext.DeletedTableMap, deletedNewTable)
		}
	case parser.ObjectType_OBJECT_COLUMN:
		ownerName, tableName := GetOwnerNameAndTableName(stmt.Relation, h.PgContext)
		err := h.handleRenameColumn(ownerName, tableName, stmt.Subname, stmt.Newname)
		if err != nil {
			return err
		}
		deletedColumn := fmt.Sprintf(ThreeFormatString, ownerName, tableName, stmt.Subname)
		if _, ok := h.PgContext.DeletedColumnMap[deletedColumn]; !ok {
			h.PgContext.DeletedColumnMap[deletedColumn] = deletedColumn
		}
		deletedNewColumn := fmt.Sprintf(ThreeFormatString, ownerName, tableName, stmt.Newname)
		if _, ok := h.PgContext.DeletedColumnMap[deletedNewColumn]; ok {
			delete(h.PgContext.DeletedColumnMap, deletedNewColumn)
		}
	case parser.ObjectType_OBJECT_TABCONSTRAINT:
		ownerName, tableName := GetOwnerNameAndTableName(stmt.Relation, h.PgContext)
		err := h.handleRenameConstraint(ownerName, tableName, stmt.Subname, stmt.Newname)
		if err != nil {
			return err
		}
		deletedConstraint := fmt.Sprintf(TwoFormatString, ownerName, stmt.Subname)
		if _, ok := h.PgContext.DeletedConstraintMap[deletedConstraint]; !ok {
			h.PgContext.DeletedConstraintMap[deletedConstraint] = deletedConstraint
		}
		deletedNewConstraint := fmt.Sprintf(TwoFormatString, ownerName, stmt.Newname)
		if _, ok := h.PgContext.DeletedConstraintMap[deletedNewConstraint]; ok {
			delete(h.PgContext.DeletedConstraintMap, deletedNewConstraint)
		}
	}
	return nil
}

func GetColumnInfo(ownerName, tableName string, columnDef *parser.ColumnDef) *ColumnInfo {
	columnName := columnDef.Colname
	columnType := ""
	typeNames := columnDef.TypeName.Names
	if len(typeNames) == 1 {
		columnType = typeNames[0].GetString_().GetStr()
	} else if len(typeNames) == 2 {
		columnType = typeNames[1].GetString_().GetStr()
	}
	columnLength, columnPrecision := 0, 0
	typMods := columnDef.TypeName.Typmods
	if typMods != nil {
		columnLength = int(typMods[0].GetAConst().GetVal().GetInteger().GetIval())
		if len(typMods) == 2 {
			columnPrecision = int(typMods[1].GetAConst().GetVal().GetInteger().GetIval())
		}
	}
	isNullable, isPrimaryColumn, isUnique := true, false, false
	constraints := columnDef.Constraints
	for _, constraint := range constraints {
		c := constraint.GetConstraint()
		if c == nil {
			continue
		}
		switch c.Contype {
		case parser.ConstrType_CONSTR_PRIMARY:
			isPrimaryColumn = true
		case parser.ConstrType_CONSTR_UNIQUE:
			isUnique = true
		case parser.ConstrType_CONSTR_NOTNULL:
			isNullable = false
		}
	}
	columnInfo := &ColumnInfo{
		ColumnName:      columnName,
		OwnerName:       ownerName,
		TableName:       tableName,
		ColumnType:      columnType,
		ColumnLength:    columnLength,
		ColumnPrecision: columnPrecision,
		IsNullable:      isNullable,
		IsPrimaryColumn: isPrimaryColumn,
		IsUnique:        isUnique,
	}
	return columnInfo
}

// GetOwnerNameAndTableName :获取schemaName和tableName
func GetOwnerNameAndTableName(relation *parser.RangeVar, c *PgContext) (string, string) {
	ownerName, tableName := "", ""
	if relation != nil {
		ownerName = relation.Schemaname
		tableName = relation.Relname
		if len(ownerName) == 0 {
			ownerName = c.DatabaseInfo.CurrentSchema
		}
	}
	return ownerName, tableName
}

// GetOwnerNameAndIndexName :获取schemaName和indexName
func GetOwnerNameAndIndexName(relation *parser.RangeVar, c *PgContext) (string, string) {
	ownerName, indexName := "", ""
	if relation != nil {
		ownerName = relation.Schemaname
		indexName = relation.Relname
		if len(ownerName) == 0 {
			ownerName = c.DatabaseInfo.CurrentSchema
		}
	}
	return ownerName, indexName
}
