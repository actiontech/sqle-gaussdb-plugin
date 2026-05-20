package inspector

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
)

// 约束类型
const (
	ConstraintTypeP   = "primary"
	ConstraintTypeU   = "unique"
	ConstraintTypeF   = "foreign"
	ConstraintTypeC   = "check"
	UsingTypeOnline   = "ONLINE"
	UsingTypeOffline  = "OFFLINE"
	TwoFormatString   = "%s:%s"
	ThreeFormatString = "%s:%s:%s"
)

type DatabaseInfo struct {
	DatabaseName           string                 // 数据库名
	DatabaseDefaultCollate string                 // 数据库默认排序规则
	CurrentSchema          string                 // 当前schema
	SchemaInfoMap          map[string]*SchemaInfo // 当前数据库下所有的schema map,key=schema,value=schemaInfo
}

type SchemaInfo struct {
	SchemaName    string       // schema名
	IsLoadFromDb  bool         // 是否从数据库装载
	TableInfoList []*TableInfo // schema下所有的表名
	IndexInfoList []*IndexInfo // schema下所有的索引
}

type TableInfo struct {
	TableName              string            // 表名
	OwnerName              string            // 属主名
	TableSizeMB            int               // 表大小
	IsView                 bool              // 是否是视图
	IsLoadFromDb           bool              // 是否从数据库装载
	ColumnInfoList         []*ColumnInfo     // table对应的column信息列表
	ConstraintList         []*ConstraintInfo // table对应的约束列表
	DistributionColumnName string            // 分片键
}

type ColumnInfo struct {
	ColumnName      string // 列名
	OwnerName       string // 属主名
	TableName       string // 表名
	ColumnType      string // 列类型
	ColumnTypType   string // 列类型类型, 如 'b', 'c', 'd', 'e', 'p', 'r', 'm'
	ColumnLength    int    // 列长度
	ColumnPrecision int    // 列精度
	IsNullable      bool   // 是否空值
	DefaultValue    string // 默认值
	IsPrimaryColumn bool   // 是否主键列，默认：false
	IsUnique        bool   // 是否唯一约束，默认：false
	IndexName       string // 列拥有的索引名
	ConstraintName  string // 列拥有的约束名
	IsLoadFromDb    bool   // 是否从数据库装载
}

type ConstraintInfo struct {
	ConstraintName    string   // 约束名
	ConstraintType    string   // 约束类型
	OwnerName         string   // 属主名
	ColumnList        []string // 列
	ReferencedTable   string   // 外键约束关联表
	ReferencedColumns []string // 外键约束关联列
	CheckCondition    string   // check约束条件
	ConstraintContent string   //约束内容
}

type IndexInfo struct {
	IndexName    string             // 索引名
	OwnerName    string             // 属主名
	TableName    string             // 表名
	ColumnList   []string           // 列名列表
	FuncCallList []*parser.FuncCall // 函数索引列表
	IsUnique     bool               // 是否唯一索引
	IsPrimaryKey bool               // 是否主键索引
	IsLoadFromDb bool               // 是否从数据库装载
}

type PgContext struct {
	CurrentDatabase      string                 // 当前数据库
	Executor             *executor.Executor     // 数据库执行器
	DatabaseInfo         *DatabaseInfo          // 数据库信息
	UsingType            string                 // 使用类型：在线(ONLINE)和离线(OFFLINE)
	PgDbTypeNameMap      map[string]string      // pg数据类型map
	ExecutionPlanCache   map[string]*[]PlanType // 执行计划缓存
	DeletedSchemaMap     map[string]string      // 已删除的schema map，key:schema；value:schema
	DeletedTableMap      map[string]string      // 已删除的table map, key:schema:tableName：value:schema:tableName
	DeletedIndexMap      map[string]string      // 已删除的index map, key:schema:indexName：value:schema:indexName
	DeletedConstraintMap map[string]string      // 已删除的constraint map，key:schema:constraintName：value:schema:constraintName(同一schema下，约束名不能重复)
	DeletedColumnMap     map[string]string      // 已删除的column map，key:schema:tableName:columnName：value:schema:tableName:columnName
	lock                 sync.Mutex
}

func NewPgContext(currentDatabase, currentSchema string, executor *executor.Executor) (*PgContext, error) {
	schemaInfoMap := make(map[string]*SchemaInfo)

	usingType := ""
	if executor != nil {
		usingType = UsingTypeOnline
	} else {
		usingType = UsingTypeOffline
	}

	defaultCollate := ""
	if usingType == UsingTypeOnline {
		var err error
		defaultCollate, err = executor.GetDatabaseDefaultCollate(currentDatabase)
		if err != nil {
			return nil, fmt.Errorf("get database default collate error:%s", err)
		}
		// 设置当前schema对应的信息
		schemaInfo, err := setSchemaInfoBySchemaName(currentSchema, executor)
		if err != nil {
			return nil, err
		}
		schemaInfoMap[currentSchema] = schemaInfo
		// 设置其他schema对应的信息(只设置schemaName，为了提高性能：其他的信息都不设置，后续使用时设置)
		schemas, err := executor.GetSchemas(currentDatabase, false)
		if err != nil {
			return nil, fmt.Errorf("get schemas eror:%s", err)
		}
		for _, schema := range schemas {
			if schema == currentSchema {
				continue
			}
			schemaInfoMap[schema] = &SchemaInfo{
				SchemaName:    schema,
				IsLoadFromDb:  true,
				TableInfoList: make([]*TableInfo, 0),
				IndexInfoList: make([]*IndexInfo, 0),
			}
		}
	} else {
		schemaInfoMap[currentSchema] = &SchemaInfo{
			SchemaName:    currentSchema,
			IsLoadFromDb:  false,
			TableInfoList: make([]*TableInfo, 0),
			IndexInfoList: make([]*IndexInfo, 0),
		}
	}

	databaseInfo := &DatabaseInfo{
		DatabaseName:           currentDatabase,
		DatabaseDefaultCollate: defaultCollate,
		CurrentSchema:          currentSchema,
		SchemaInfoMap:          schemaInfoMap,
	}
	pgContext := &PgContext{
		CurrentDatabase:      currentDatabase,
		Executor:             executor,
		DatabaseInfo:         databaseInfo,
		UsingType:            usingType,
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	return pgContext, nil
}

// IsExistSchema :schema是否存在
func (c *PgContext) IsExistSchema(database, schemaName string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, ok := c.DeletedSchemaMap[schemaName]; ok {
		return false, nil
	}
	_, ok := c.DatabaseInfo.SchemaInfoMap[schemaName]
	if ok {
		return true, nil
	}
	if c.UsingType == UsingTypeOffline {
		return false, nil
	}
	schemas, err := c.Executor.GetSchemas(database, true)
	if err != nil {
		return false, fmt.Errorf("get schemas eror:%s", err)
	}
	isExistSchema := false
	for _, schema := range schemas {
		if schema == schemaName {
			isExistSchema = true
		}
		c.DatabaseInfo.SchemaInfoMap[schema] = &SchemaInfo{
			SchemaName:    schema,
			IsLoadFromDb:  true,
			TableInfoList: make([]*TableInfo, 0),
			IndexInfoList: make([]*IndexInfo, 0),
		}
	}
	if isExistSchema {
		return true, nil
	}
	// 这里返回nil，表示schema没有找到，并不是程序报错
	return false, nil
}

// IsExistTable :表是否存在
func (c *PgContext) IsExistTable(schemaName, tableName string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	deletedTable := fmt.Sprintf(TwoFormatString, schemaName, tableName)
	if _, ok := c.DeletedTableMap[deletedTable]; ok {
		return false, nil
	}
	schemaInfo, tablePosition := getSchemaInfoAndTablePosition(c, schemaName, tableName)
	if tablePosition > -1 {
		return true, nil
	}
	if c.UsingType == UsingTypeOffline {
		return false, nil
	}
	// 从数据库中获取当前schema对应的所有表
	tableNames, err := c.Executor.GetTableNamesBySchemaName(schemaName)
	if err != nil {
		return false, fmt.Errorf("get tableNames by schema name error:%s", err)
	}
	isExist := false
	for _, item := range tableNames {
		if item == tableName {
			isExist = true
			break
		}
	}
	if !isExist {
		return false, nil
	}

	tableColumnsMap, err := getColumnInfoListFromDbBatch(schemaName, c.Executor)
	if err != nil {
		return false, fmt.Errorf("batch get column info list eror:%s", err)
	}
	tableConstraintMap, err := getConstraintInfoListFromDbBatch(schemaName, c.Executor)
	if err != nil {
		return false, fmt.Errorf("batch get constraint info list eror:%s", err)
	}
	tableIndexList, err := getIndexInfoListFromDbBatch(schemaName, c.Executor)
	if err != nil {
		return false, fmt.Errorf("batch get index info list eror:%s", err)
	}

	// 列信息
	columnsList, ok := tableColumnsMap[tableName]
	if !ok {
		return false, err
	}
	// 约束信息
	constraintInfos, ok := tableConstraintMap[tableName]
	if !ok {
		constraintInfos = make([]*ConstraintInfo, 0)
	}
	tableInfo := &TableInfo{
		TableName:      tableName,
		OwnerName:      schemaName,
		IsLoadFromDb:   true,
		ColumnInfoList: columnsList,
		ConstraintList: constraintInfos,
	}
	schemaInfo.TableInfoList = append(schemaInfo.TableInfoList, tableInfo)
	// 获取表对应的索引信息
	for _, indexInfo := range tableIndexList {
		if indexInfo.TableName == tableName {
			schemaInfo.IndexInfoList = append(schemaInfo.IndexInfoList, indexInfo)
		}
	}
	c.DatabaseInfo.SchemaInfoMap[schemaName] = schemaInfo
	return true, nil
}

// IsExistColumn :列是否存在
func (c *PgContext) IsExistColumn(schemaName, tableName, columnName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	deletedColumn := fmt.Sprintf(ThreeFormatString, schemaName, tableName, columnName)
	if _, ok := c.DeletedColumnMap[deletedColumn]; ok {
		return false
	}
	value, ok := c.DatabaseInfo.SchemaInfoMap[schemaName]
	if !ok {
		return false
	}
	list := value.TableInfoList
	if len(list) == 0 {
		return false
	}
	var tableObj *TableInfo
	for _, tableInfo := range list {
		if tableInfo.TableName == tableName {
			tableObj = tableInfo
			break
		}
	}
	if tableObj != nil {
		columnInfoList := tableObj.ColumnInfoList
		for _, columnInfo := range columnInfoList {
			if columnInfo.ColumnName == columnName {
				return true
			}
		}
	}
	return false
}

// IsExistIndex :索引是否存在
func (c *PgContext) IsExistIndex(schemaName, tableName, indexName string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	deletedIndex := fmt.Sprintf(TwoFormatString, schemaName, indexName)
	if _, ok := c.DeletedIndexMap[deletedIndex]; ok {
		return false, nil
	}
	value, ok := c.DatabaseInfo.SchemaInfoMap[schemaName]
	if !ok {
		return false, nil
	}
	indexInfoList := value.IndexInfoList
	if len(indexInfoList) == 0 {
		if c.UsingType == UsingTypeOffline {
			return false, nil
		}
		value.IndexInfoList = make([]*IndexInfo, 0)
	}
	for _, indexInfo := range indexInfoList {
		if indexInfo.IndexName == indexName {
			return true, nil
		}
	}
	// 离线上下文，这里没有找到索引就返回false，不继续进行了。
	if c.UsingType == UsingTypeOffline {
		return false, nil
	}
	// 获取索引所在的表名
	if len(tableName) == 0 {
		indexDef, err := c.Executor.GetIndexDef(nil, schemaName, indexName)
		if err != nil {
			return false, fmt.Errorf("index not exist, index name=%s", indexName)
		}
		ast, err := SqlParserFunc(indexDef)
		if err != nil {
			return false, fmt.Errorf("parse sql failed: %v", err)
		}
		switch stmt := ast.(*parser.RawStmt).GetStmt().GetNode().(type) {
		case *parser.Node_IndexStmt:
			schemaName, tableName = GetOwnerNameAndTableName(stmt.IndexStmt.Relation, c)
		}
	}
	if len(tableName) == 0 {
		return false, nil
	}
	// 获取表对应的索引信息
	indexInfos, err := getIndexInfoListFromDb(schemaName, tableName, c.Executor)
	if err != nil {
		return false, fmt.Errorf("pg_context IsExistIndex getIndexInfoListFromDb error %s", err)
	}
	isExistIndex := false
	for _, indexInfo := range indexInfos {
		if indexInfo.IndexName != indexName {
			continue
		}
		isExistIndex = true
	}
	if len(indexInfos) > 0 {
		value.IndexInfoList = append(value.IndexInfoList, indexInfos...)
		c.DatabaseInfo.SchemaInfoMap[schemaName] = value
	}
	return isExistIndex, nil
}

// AddSchema :添加schema
func (c *PgContext) AddSchema(schemaName string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	schemaInfo := &SchemaInfo{
		SchemaName:    schemaName,
		TableInfoList: make([]*TableInfo, 0),
		IndexInfoList: make([]*IndexInfo, 0),
	}
	c.DatabaseInfo.SchemaInfoMap[schemaName] = schemaInfo
}

// RemoveSchema :删除schema
func (c *PgContext) RemoveSchema(schemaName string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.DatabaseInfo.SchemaInfoMap, schemaName)
}

// AddTable :添加表
func (c *PgContext) AddTable(tableInfo *TableInfo) error {
	table, err := c.IsExistTable(tableInfo.OwnerName, tableInfo.TableName)
	if err != nil {
		return err
	}
	if !table {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[tableInfo.OwnerName]
		if schemaInfo == nil {
			schemaInfo = &SchemaInfo{TableInfoList: make([]*TableInfo, 0)}
		}
		if schemaInfo.SchemaName == "" {
			schemaInfo.SchemaName = tableInfo.OwnerName
		}
		schemaInfo.TableInfoList = append(schemaInfo.TableInfoList, tableInfo)
	}
	return nil
}

// RenameTable :重命名表
func (c *PgContext) RenameTable(schemaName, tableName, newTableName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	schemaInfo, tablePosition := getSchemaInfoAndTablePosition(c, schemaName, tableName)
	if tablePosition == -1 {
		return false
	}

	list := schemaInfo.TableInfoList
	isExistNewTable := false
	for _, tableInfo := range list {
		if tableInfo.TableName == newTableName {
			isExistNewTable = true
			break
		}
	}
	if !isExistNewTable {
		tableInfo := list[tablePosition]
		tableInfo.TableName = newTableName
		return true
	}
	return false
}

// DeleteTable :删除表
func (c *PgContext) DeleteTable(schemaName, tableName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	schemaInfo, tablePosition := getSchemaInfoAndTablePosition(c, schemaName, tableName)
	if tablePosition > -1 {
		list := schemaInfo.TableInfoList
		list = append(list[:tablePosition], list[tablePosition+1:]...)
		schemaInfo.TableInfoList = list
		// 删除表对应的索引
		indexInfoList := schemaInfo.IndexInfoList
		finalIndexInfoList := make([]*IndexInfo, 0)
		for _, indexInfo := range indexInfoList {
			if indexInfo.OwnerName == schemaName && indexInfo.TableName == tableName {
				continue
			}
			finalIndexInfoList = append(finalIndexInfoList, indexInfo)
		}
		schemaInfo.IndexInfoList = finalIndexInfoList
		return true
	}
	return false
}

// AddColumns :添加列
func (c *PgContext) AddColumns(schemaName, tableName string, columnInfoList *[]*ColumnInfo) error {
	table, err := c.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[schemaName]
		if schemaInfo == nil {
			return nil
		}
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName == tableName {
				tableInfo.ColumnInfoList = append(tableInfo.ColumnInfoList, *columnInfoList...)
				break
			}
		}
	}
	return nil
}

// AlterColumnType :修改列类型
func (c *PgContext) AlterColumnType(schemaName, tableName string, columnInfo *ColumnInfo) error {
	table, err := c.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[schemaName]
		if schemaInfo == nil {
			return nil
		}
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName == tableName {
				columnInfos := tableInfo.ColumnInfoList
				alterColumnPosition := -1
				for i, columnInfoSource := range columnInfos {
					if columnInfoSource.ColumnName == columnInfo.ColumnName {
						alterColumnPosition = i
					}
				}
				if alterColumnPosition == -1 {
					break
				}
				columnInfos[alterColumnPosition].ColumnType = columnInfo.ColumnType
				columnInfos[alterColumnPosition].ColumnLength = columnInfo.ColumnLength
				columnInfos[alterColumnPosition].ColumnPrecision = columnInfo.ColumnPrecision
				break
			}
		}
	}
	return nil
}

// AlterColumnNotNull :修改列not null
func (c *PgContext) AlterColumnNotNull(schemaName, tableName, columnName string, isSetNotNull bool) error {
	table, err := c.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[schemaName]
		if schemaInfo == nil {
			return nil
		}
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName == tableName {
				columnInfos := tableInfo.ColumnInfoList
				alterColumnPosition := -1
				for i, columnInfoSource := range columnInfos {
					if columnInfoSource.ColumnName == columnName {
						alterColumnPosition = i
					}
				}
				if alterColumnPosition == -1 {
					break
				}
				columnInfos[alterColumnPosition].IsNullable = !isSetNotNull
				break
			}
		}
	}
	return nil
}

// RenameColumn :重命名列
func (c *PgContext) RenameColumn(schemaName, tableName, columnName, newColumnName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	schemaInfo, tablePosition := getSchemaInfoAndTablePosition(c, schemaName, tableName)
	if tablePosition == -1 {
		return false
	}
	isExistColumn := false
	isExistNewColumn := false
	columnIndex := 0
	list := schemaInfo.TableInfoList
	tableInfo := list[tablePosition]
	columnInfoList := tableInfo.ColumnInfoList
	for i, columnInfo := range columnInfoList {
		if columnInfo.ColumnName == columnName {
			isExistColumn = true
			columnIndex = i
		}
		if columnInfo.ColumnName == newColumnName {
			isExistNewColumn = true
		}
	}

	if isExistColumn && !isExistNewColumn {
		columnInfo := columnInfoList[columnIndex]
		columnInfo.ColumnName = newColumnName
		return true
	}
	return false
}

// RenameConstraint :重命名约束
func (c *PgContext) RenameConstraint(schemaName, tableName, constraintName, newConstraintName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	schemaInfo, tablePosition := getSchemaInfoAndTablePosition(c, schemaName, tableName)
	if tablePosition == -1 {
		return false
	}

	isExistConstraint := false
	isExistNewConstraint := false
	constraintIndex := 0
	list := schemaInfo.TableInfoList
	tableInfo := list[tablePosition]
	constraintList := tableInfo.ConstraintList
	for i, constraintInfo := range constraintList {
		if constraintInfo.ConstraintName == constraintName {
			isExistConstraint = true
			constraintIndex = i
		}
		if constraintInfo.ConstraintName == newConstraintName {
			isExistNewConstraint = true
		}
	}

	if isExistConstraint && !isExistNewConstraint {
		constraintInfo := constraintList[constraintIndex]
		constraintInfo.ConstraintContent = strings.ReplaceAll(constraintInfo.ConstraintContent,
			constraintInfo.ConstraintName, newConstraintName)
		constraintInfo.ConstraintName = newConstraintName
		return true
	}
	return false
}

// RemoveColumns :删除列
func (c *PgContext) RemoveColumns(schemaName, tableName string, columnNames *[]string) error {
	table, err := c.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[schemaName]
		if schemaInfo == nil {
			return nil
		}
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName == tableName {
				columnInfos := tableInfo.ColumnInfoList
				finalColumnInfos := make([]*ColumnInfo, 0)
				for _, columnInfoSource := range columnInfos {
					isExistDeleteColumn := false
					for _, columnName := range *columnNames {
						if columnInfoSource.ColumnName == columnName {
							isExistDeleteColumn = true
							break
						}
					}
					if !isExistDeleteColumn {
						finalColumnInfos = append(finalColumnInfos, columnInfoSource)
					}
				}
				tableInfo.ColumnInfoList = tableInfo.ColumnInfoList[:0]
				tableInfo.ColumnInfoList = append(tableInfo.ColumnInfoList, finalColumnInfos...)
				break
			}
		}
	}
	return nil
}

// AddConstraints :添加约束
func (c *PgContext) AddConstraints(schemaName, tableName string, constraintList *[]*ConstraintInfo) error {
	table, err := c.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[schemaName]
		if schemaInfo == nil {
			return nil
		}
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName == tableName {
				// 表中现有约束
				constraints := tableInfo.ConstraintList
				finalAddConstraints := make([]*ConstraintInfo, 0)
				for _, constraintAdd := range *constraintList {
					isExistConstraint := false
					for _, constraintSource := range constraints {
						if constraintAdd.ConstraintName == constraintSource.ConstraintName {
							isExistConstraint = true
							break
						}
					}
					if !isExistConstraint {
						finalAddConstraints = append(finalAddConstraints, constraintAdd)
					}
				}
				tableInfo.ConstraintList = append(tableInfo.ConstraintList, finalAddConstraints...)
				break
			}
		}
	}
	return nil
}

// RemoveConstraints :删除约束
func (c *PgContext) RemoveConstraints(schemaName, tableName string, constraintList *[]string) error {
	table, err := c.IsExistTable(schemaName, tableName)
	if err != nil {
		return err
	}
	if table {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[schemaName]
		if schemaInfo == nil {
			return nil
		}
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName == tableName {
				constraints := tableInfo.ConstraintList
				finalConstraints := make([]*ConstraintInfo, 0)
				for _, constraintSource := range constraints {
					isExistDeleteConstraint := false
					for _, constraintNameRemove := range *constraintList {
						if constraintSource.ConstraintName == constraintNameRemove {
							isExistDeleteConstraint = true
							break
						}
					}
					if !isExistDeleteConstraint {
						finalConstraints = append(finalConstraints, constraintSource)
					}
				}
				tableInfo.ConstraintList = tableInfo.ConstraintList[:0]
				tableInfo.ConstraintList = append(tableInfo.ConstraintList, finalConstraints...)
				break
			}
		}
	}
	return nil
}

// AddIndex :添加索引
func (c *PgContext) AddIndex(indexInfo *IndexInfo) error {
	if indexInfo == nil {
		return nil
	}
	index, err := c.IsExistIndex(indexInfo.OwnerName, indexInfo.TableName, indexInfo.IndexName)
	if err != nil {
		return err
	}
	if !index {
		c.lock.Lock()
		defer c.lock.Unlock()
		schemaInfo := c.DatabaseInfo.SchemaInfoMap[indexInfo.OwnerName]
		if schemaInfo != nil {
			schemaInfo.IndexInfoList = append(schemaInfo.IndexInfoList, indexInfo)
		}
	}
	return nil
}

// RenameIndex :重命名索引
func (c *PgContext) RenameIndex(schemaName, indexName, newIndexName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	schemaInfo, indexPosition := getSchemaInfoAndIndexPosition(c, schemaName, indexName)
	if indexPosition == -1 {
		return false
	}

	indexInfoList := schemaInfo.IndexInfoList
	// 校验新的索引名
	isExistNewIndex := false
	for _, indexInfo := range indexInfoList {
		if indexInfo.IndexName == newIndexName {
			isExistNewIndex = true
			break
		}
	}

	if !isExistNewIndex {
		indexInfo := indexInfoList[indexPosition]
		indexInfo.IndexName = newIndexName
		// 先删除旧的索引
		indexInfoList = append(indexInfoList[:indexPosition], indexInfoList[indexPosition+1:]...)
		// 添加新的索引
		indexInfoList = append(indexInfoList, indexInfo)
		return true
	}
	return false
}

// DeleteIndex :删除索引
func (c *PgContext) DeleteIndex(schemaName, indexName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	schemaInfo, indexPosition := getSchemaInfoAndIndexPosition(c, schemaName, indexName)
	if indexPosition > -1 {
		indexInfoList := schemaInfo.IndexInfoList
		indexInfoList = append(indexInfoList[:indexPosition], indexInfoList[indexPosition+1:]...)
		schemaInfo.IndexInfoList = indexInfoList
		return true
	}
	return false
}

// GetTableIndexColumns 根据schema和表名获取索引列slice，返回结果格式：["column1,column2","column1,column2,column3"]
func (c *PgContext) GetTableIndexColumns(schemaName, tableName string) []string {
	indexColumns := make([]string, 0)
	schemaInfo, ok := c.DatabaseInfo.SchemaInfoMap[schemaName]
	if ok {
		indexInfoList := schemaInfo.IndexInfoList
		for _, indexInfo := range indexInfoList {
			if indexInfo.TableName == tableName {
				indexColumns = append(indexColumns, strings.Join(indexInfo.ColumnList, ","))
			}
		}
	}
	return indexColumns
}

// GetTableColumns 根据schema和表名获取表列slice，返回结果格式：["column1,column2"]
func (c *PgContext) GetTableColumns(schemaName, tableName string) []string {
	columns := make([]string, 0)
	if schemaInfo, ok := c.DatabaseInfo.SchemaInfoMap[schemaName]; ok {
		tableInfoList := schemaInfo.TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName != tableName {
				continue
			}
			columnInfoList := tableInfo.ColumnInfoList
			for _, columnInfo := range columnInfoList {
				columns = append(columns, columnInfo.ColumnName)
			}
		}
	}
	return columns
}

func (c *PgContext) GetDatabaseDefaultCollate() string {
	return c.DatabaseInfo.DatabaseDefaultCollate
}

// setSchemaInfoBySchemaName :设置schemaName对应的SchemaInfo信息
func setSchemaInfoBySchemaName(schemaName string, executor *executor.Executor) (*SchemaInfo, error) {
	tableInfoList := make([]*TableInfo, 0)
	indexInfoList := make([]*IndexInfo, 0)
	// 从数据库中获取当前schema对应的所有表及索引
	tableNames, err := executor.GetTableNamesBySchemaName(schemaName)
	if err != nil {
		return nil, fmt.Errorf("get tablenames by schema name eror:%s", err)
	}
	viewNames, err := executor.GetViewNamesBySchemaName(schemaName)
	if err != nil {
		return nil, fmt.Errorf("get viewnames by schema name error:%s", err)
	}
	tableColumnsMap, err := getColumnInfoListFromDbBatch(schemaName, executor)
	if err != nil {
		return nil, fmt.Errorf("batch get column list info from db eror:%s", err)
	}
	tableConstraintMap, err := getConstraintInfoListFromDbBatch(schemaName, executor)
	if err != nil {
		return nil, fmt.Errorf("batch get constraint list info from db eror:%s", err)
	}
	tableIndexList, err := getIndexInfoListFromDbBatch(schemaName, executor)
	if err != nil {
		return nil, fmt.Errorf("batch get index list info from db eror:%s", err)
	}
	if len(tableIndexList) > 0 {
		indexInfoList = append(indexInfoList, tableIndexList...)
	}

	tableDistributionMap, err := GetTableDistributionInfo(schemaName, executor)
	if err != nil {
		return nil, fmt.Errorf("get table distribution info error:%s", err)
	}

	for _, tableName := range tableNames {
		// 列信息
		columnsList, ok := tableColumnsMap[tableName]
		if !ok {
			continue
		}
		// 约束信息
		constraintList, ok := tableConstraintMap[tableName]
		if !ok {
			constraintList = make([]*ConstraintInfo, 0)
		}
		tableInfoList = append(tableInfoList, &TableInfo{
			TableName:              tableName,
			IsLoadFromDb:           true,
			ColumnInfoList:         columnsList,
			ConstraintList:         constraintList,
			DistributionColumnName: tableDistributionMap[tableName],
		})
	}
	for _, view := range viewNames {
		// 列信息
		columnsList, ok := tableColumnsMap[view]
		if !ok {
			continue
		}
		// 约束信息
		constraintList, ok := tableConstraintMap[view]
		if !ok {
			constraintList = make([]*ConstraintInfo, 0)
		}
		tableInfoList = append(tableInfoList, &TableInfo{
			TableName:      view,
			IsView:         true,
			IsLoadFromDb:   true,
			ColumnInfoList: columnsList,
			ConstraintList: constraintList,
		})
	}
	return &SchemaInfo{
		SchemaName:    schemaName,
		TableInfoList: tableInfoList,
		IndexInfoList: indexInfoList,
	}, nil
}

// getSchemaInfoAndTablePosition :获取schemaInfo和表在表列表中的位置
func getSchemaInfoAndTablePosition(p *PgContext, schemaName, tableName string) (*SchemaInfo, int) {
	schemaInfo, ok := p.DatabaseInfo.SchemaInfoMap[schemaName]
	if !ok {
		return &SchemaInfo{SchemaName: schemaName}, -1
	}
	list := schemaInfo.TableInfoList
	if len(list) == 0 {
		return &SchemaInfo{SchemaName: schemaName}, -1
	}
	tablePosition := -1
	for i, tableInfo := range list {
		if tableInfo.TableName == tableName {
			tablePosition = i
			break
		}
	}
	return schemaInfo, tablePosition
}

// getColumnInfoListFromDb :从数据库中获取列信息
func getColumnInfoListFromDb(schemaName, tableName string, executor *executor.Executor) ([]*ColumnInfo, error) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnsInfo, err := executor.GetTableColumnsInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	for _, columnInfo := range columnsInfo {
		columnLength := 0
		if columnInfo.ColumnType == "character varying" {
			columnLength, _ = strconv.Atoi(columnInfo.CharacterMaximumLength)
		} else {
			columnLength, _ = strconv.Atoi(columnInfo.NumericPrecision)
		}
		aToiNumericScale, _ := strconv.Atoi(columnInfo.NumericScale)
		columnInfoList = append(columnInfoList, &ColumnInfo{
			TableName:       tableName,
			OwnerName:       schemaName,
			ColumnName:      columnInfo.ColumnName,
			ColumnType:      columnInfo.ColumnType,
			ColumnTypType:   columnInfo.ColumnTypType,
			ColumnLength:    columnLength,
			ColumnPrecision: aToiNumericScale,
			IsNullable:      columnInfo.IsNullable == "YES",
			DefaultValue:    columnInfo.ColumnDefault,
		})
	}
	return columnInfoList, nil
}

// getColumnInfoListFromDbBatch :批量从数据库中获取列信息
func getColumnInfoListFromDbBatch(schemaName string, executor *executor.Executor) (map[string][]*ColumnInfo, error) {
	ret := make(map[string][]*ColumnInfo)
	tableColumnsInfoMap, err := executor.GetTableColumnsInfoBatch(schemaName)
	if err != nil {
		return nil, err
	}
	for tableName, tableColumnInfo := range tableColumnsInfoMap {
		columnInfoList := make([]*ColumnInfo, 0)
		for _, columnInfo := range tableColumnInfo {
			columnLength := 0
			if columnInfo.ColumnType == "character varying" {
				columnLength, _ = strconv.Atoi(columnInfo.CharacterMaximumLength)
			} else {
				columnLength, _ = strconv.Atoi(columnInfo.NumericPrecision)
			}
			aToiNumericScale, _ := strconv.Atoi(columnInfo.NumericScale)
			columnInfoList = append(columnInfoList, &ColumnInfo{
				TableName:       tableName,
				OwnerName:       schemaName,
				ColumnName:      columnInfo.ColumnName,
				ColumnType:      columnInfo.ColumnType,
				ColumnLength:    columnLength,
				ColumnPrecision: aToiNumericScale,
				IsNullable:      columnInfo.IsNullable == "YES",
				DefaultValue:    columnInfo.ColumnDefault,
			})
		}
		ret[tableName] = columnInfoList
	}
	return ret, nil
}

// getConstraintInfoListFromDb :从数据库中获取表约束信息
func getConstraintInfoListFromDb(schemaName, tableName string, executor *executor.Executor) ([]*ConstraintInfo, error) {
	constraintInfoList := make([]*ConstraintInfo, 0)
	tableConstraintsInfo, err := executor.GetTableConstraintsInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	for _, constraintInfo := range tableConstraintsInfo {
		constraintName := constraintInfo.ConstraintName
		constraintType, referencedTable, checkCondition, constraintContent := "", "", "", ""
		columnList, referencedColumnList := make([]string, 0), make([]string, 0)
		switch strings.ToLower(constraintInfo.ConstraintType) {
		case "p":
			constraintType = ConstraintTypeP
			columnList = strings.Split(constraintInfo.Columns, ",")
			constraintContent = fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)",
				constraintName, constraintInfo.Columns)
		case "u":
			constraintType = ConstraintTypeU
			columnList = strings.Split(constraintInfo.Columns, ",")
			constraintContent = fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)",
				constraintName, constraintInfo.Columns)
		case "f":
			constraintType = ConstraintTypeF
			columnList = strings.Split(constraintInfo.Columns, ",")
			referencedTable = constraintInfo.ReferencedTable
			referencedColumnList = strings.Split(constraintInfo.ReferencedColumns, ",")
			constraintContent = fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
				constraintName, constraintInfo.Columns, referencedTable, constraintInfo.ReferencedColumns)
		case "c":
			constraintType = ConstraintTypeC
			checkCondition = constraintInfo.CheckCondition
			constraintContent = fmt.Sprintf("CONSTRAINT %s CHECK (%s)", constraintName, checkCondition)
		}
		constraintInfoList = append(constraintInfoList, &ConstraintInfo{
			ConstraintName:    constraintName,
			ConstraintType:    constraintType,
			ColumnList:        columnList,
			ReferencedTable:   referencedTable,
			ReferencedColumns: referencedColumnList,
			CheckCondition:    checkCondition,
			ConstraintContent: constraintContent,
		})
	}
	return constraintInfoList, nil
}

// getConstraintInfoListFromDbBatch :从数据库中批量获取表约束信息
func getConstraintInfoListFromDbBatch(schemaName string, executor *executor.Executor) (map[string][]*ConstraintInfo, error) {
	ret := make(map[string][]*ConstraintInfo)
	tableConstraintsInfoMap, err := executor.GetTableConstraintsInfoBatch(schemaName)
	if err != nil {
		return nil, err
	}
	for tableName, tableConstraintInfo := range tableConstraintsInfoMap {
		constraintInfoList := make([]*ConstraintInfo, 0)
		for _, constraintInfo := range tableConstraintInfo {
			constraintName := constraintInfo.ConstraintName
			constraintType, referencedTable, checkCondition, constraintContent := "", "", "", ""
			columnList, referencedColumnList := make([]string, 0), make([]string, 0)
			switch strings.ToLower(constraintInfo.ConstraintType) {
			case "p":
				constraintType = ConstraintTypeP
				columnList = strings.Split(constraintInfo.Columns, ",")
				constraintContent = fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)",
					constraintName, constraintInfo.Columns)
			case "u":
				constraintType = ConstraintTypeU
				columnList = strings.Split(constraintInfo.Columns, ",")
				constraintContent = fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)",
					constraintName, constraintInfo.Columns)
			case "f":
				constraintType = ConstraintTypeF
				columnList = strings.Split(constraintInfo.Columns, ",")
				referencedTable = constraintInfo.ReferencedTable
				referencedColumnList = strings.Split(constraintInfo.ReferencedColumns, ",")
				constraintContent = fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
					constraintName, constraintInfo.Columns, referencedTable, constraintInfo.ReferencedColumns)
			case "c":
				constraintType = ConstraintTypeC
				checkCondition = constraintInfo.CheckCondition
				constraintContent = fmt.Sprintf("CONSTRAINT %s CHECK (%s)", constraintName, checkCondition)
			}
			constraintInfoList = append(constraintInfoList, &ConstraintInfo{
				ConstraintName:    constraintName,
				ConstraintType:    constraintType,
				ColumnList:        columnList,
				ReferencedTable:   referencedTable,
				ReferencedColumns: referencedColumnList,
				CheckCondition:    checkCondition,
				ConstraintContent: constraintContent,
			})
		}
		ret[tableName] = constraintInfoList
	}
	return ret, nil
}

// getIndexInfoListFromDb :从数据库中获取当前表的索引信息
func getIndexInfoListFromDb(schemaName, tableName string, e *executor.Executor) ([]*IndexInfo, error) {
	indexInfoList := make([]*IndexInfo, 0)
	indexesInfo, err := e.GetTableIndexesInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	constraintInfo, err := e.GetTableColumnConstraintInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	for _, indexInfo := range indexesInfo {
		indexDDL := indexInfo.IndexDDL
		if len(indexDDL) == 0 {
			continue
		}
		isUniqueIndex := false
		// 索引对应的列列表
		columnList := make([]string, 0)
		funcCallList := make([]*parser.FuncCall, 0)
		ast, err := SqlParserFunc(indexDDL)
		if err != nil {
			return nil, fmt.Errorf("parse sql failed: %v", err)
		}
		switch stmt := ast.(*parser.RawStmt).GetStmt().GetNode().(type) {
		case *parser.Node_IndexStmt:
			if stmt.IndexStmt.GetUnique() {
				isUniqueIndex = true
			}
			indexParams := stmt.IndexStmt.IndexParams
			for _, indexParam := range indexParams {
				columnList = append(columnList, indexParam.GetIndexElem().GetName())
				if indexParam.GetIndexElem().GetExpr() != nil {
					if f, ok := indexParam.GetIndexElem().GetExpr().GetNode().(*parser.Node_FuncCall); ok {
						funcCallList = append(funcCallList, f.FuncCall)
					}
				}
			}
		}
		isPrimaryKey := false
		for _, info := range constraintInfo {
			if info.ConstraintType == executor.ColumnConstraintTypePRIMARY_KEY && info.ConstraintName == indexInfo.IndexName {
				isPrimaryKey = true
				break
			}
		}
		indexInfoList = append(indexInfoList, &IndexInfo{
			IndexName:    indexInfo.IndexName,
			OwnerName:    schemaName,
			TableName:    tableName,
			ColumnList:   columnList,
			FuncCallList: funcCallList,
			IsUnique:     isUniqueIndex,
			IsPrimaryKey: isPrimaryKey,
			IsLoadFromDb: true,
		})
	}
	return indexInfoList, nil
}

// getIndexInfoListFromDbBatch :从数据库中批量获取当前表的索引信息
func getIndexInfoListFromDbBatch(schemaName string, e *executor.Executor) ([]*IndexInfo, error) {
	ret := make([]*IndexInfo, 0)
	indexesInfoMap, err := e.GetTableIndexesInfoBatch(schemaName)
	if err != nil {
		return nil, err
	}
	constraintInfo, err := e.GetTableColumnConstraintInfoBatch(schemaName)
	if err != nil {
		return nil, err
	}
	for tableName, tableIndexInfo := range indexesInfoMap {
		for _, indexInfo := range tableIndexInfo {
			indexDDL := indexInfo.IndexDDL
			if len(indexDDL) == 0 {
				continue
			}
			isUniqueIndex := false

			// TODO 解析器支持 GLOBAL 全局索引 eg: create global index if not exists index51 on gi_insert using btree (f13);
			// indexDDL 是从数据库查询的创建语句，比较规范，先临时处理
			words := strings.Split(indexDDL, " ")
			if len(words) > 1 && strings.ToLower(words[1]) == "global" {
				e.Db.Logger().Warn("ignore keyword GLOBAL when parse sql: %s", indexDDL)
				indexDDL = strings.Join(append(words[:1], words[2:]...), " ")
			}
			// 索引对应的列列表
			columnList := make([]string, 0)
			ast, err := SqlParserFunc(indexDDL)
			if err != nil {
				return nil, fmt.Errorf("parse sql: %s failed: %v", indexDDL, err)
			}
			switch stmt := ast.(*parser.RawStmt).GetStmt().GetNode().(type) {
			case *parser.Node_IndexStmt:
				if stmt.IndexStmt.GetUnique() {
					isUniqueIndex = true
				}
				indexParams := stmt.IndexStmt.IndexParams
				for _, indexParam := range indexParams {
					columnList = append(columnList, indexParam.GetIndexElem().GetName())
				}
			}
			isPrimaryKey := false
			for _, info := range constraintInfo[tableName] {
				if info.ConstraintType == executor.ColumnConstraintTypePRIMARY_KEY && info.ConstraintName == indexInfo.IndexName {
					isPrimaryKey = true
					break
				}
			}
			ret = append(ret, &IndexInfo{
				IndexName:    indexInfo.IndexName,
				OwnerName:    schemaName,
				TableName:    tableName,
				ColumnList:   columnList,
				IsUnique:     isUniqueIndex,
				IsPrimaryKey: isPrimaryKey,
				IsLoadFromDb: true,
			})
		}
	}
	return ret, nil
}

// getSchemaInfoAndIndexPosition :获取schemaInfo和索引在索引列表中的位置
func getSchemaInfoAndIndexPosition(p *PgContext, schemaName, indexName string) (*SchemaInfo, int) {
	schemaInfo, ok := p.DatabaseInfo.SchemaInfoMap[schemaName]
	if !ok {
		return &SchemaInfo{SchemaName: schemaName}, -1
	}
	indexInfoList := schemaInfo.IndexInfoList
	if len(indexInfoList) == 0 {
		return &SchemaInfo{SchemaName: schemaName}, -1
	}
	indexPosition := -1
	for i, indexInfo := range indexInfoList {
		if indexInfo.IndexName == indexName {
			indexPosition = i
			break
		}
	}
	return schemaInfo, indexPosition
}

func GetTableDistributionInfo(schemaName string, executor *executor.Executor) (ret map[string] /*table name*/ string, err error) {
	ret = make(map[string]string)
	infos, err := executor.GetTableDistributionInfo(schemaName)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		ret[info.TableName] = info.DistributionColumnName
	}
	return ret, nil
}

func (c *PgContext) GetTableColumnsInfo(schema, tableName string) ([]*executor.TableColumnsInfo, error) {
	if c.Executor != nil {
		return c.Executor.GetTableColumnsInfo(schema, tableName)
	}
	return make([]*executor.TableColumnsInfo, 0), nil
}

func (c *PgContext) GetTableIndexesInfo(schema, tableName string) ([]*executor.TableIndexesInfo, error) {
	if c.Executor != nil {
		return c.Executor.GetTableIndexesInfo(schema, tableName)
	}
	return make([]*executor.TableIndexesInfo, 0), nil
}

func (c *PgContext) GetTableAutoIncrementColumnDefaultValue(scheme, tableName string) (map[string]string, error) {
	if c.Executor != nil {
		return c.Executor.GetTableAutoIncrementColumnDefaultValue(nil, scheme, tableName)
	}
	return make(map[string]string), nil
}

func (c *PgContext) ComputeIndexCellDivision(schemaName, tableName, indexColumn string) (int, error) {
	if c.Executor != nil {
		return c.Executor.ComputeIndexCellDivision(schemaName, tableName, indexColumn)
	}
	return 0, nil
}

func (c *PgContext) ValidateFunctionExistInDb(databaseName, schemaName, functionName string) (bool, error) {
	if c.Executor != nil {
		return c.Executor.ValidateFunctionExistInDb(databaseName, schemaName, functionName)
	}
	return true, nil
}

func (c *PgContext) GetFunctionInfoList(databaseName, schemaName, functionName string) ([]map[string]sql.NullString, error) {
	if c.Executor != nil {
		return c.Executor.GetFunctionInfoList(databaseName, schemaName, functionName)
	}
	return make([]map[string]sql.NullString, 0), nil
}

func (c *PgContext) ValidateProcedureExistInDb(databaseName, schemaName, procedureName string) (bool, error) {
	if c.Executor != nil {
		return c.Executor.ValidateProcedureExistInDb(databaseName, schemaName, procedureName)
	}
	return true, nil
}

func (c *PgContext) GetProcedureInfoList(databaseName, schemaName, procedureName string) ([]map[string]sql.NullString, error) {
	if c.Executor != nil {
		return c.Executor.GetProcedureInfoList(databaseName, schemaName, procedureName)
	}
	return make([]map[string]sql.NullString, 0), nil
}

func (c *PgContext) GetViewListForSchema(schemaName string) ([]string, error) {
	if c.Executor != nil {
		return c.Executor.GetViewListForSchema(schemaName)
	}
	return make([]string, 0), nil
}

func (c *PgContext) GetIndexDef(schema string, indexName string) (string, error) {
	if c.Executor != nil {
		return c.Executor.GetIndexDef(nil, schema, indexName)
	}
	return "", nil
}
