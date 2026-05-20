package inspector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	pg_query "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
)

type UnSupportSqlType struct {
	msg string
}

func (e UnSupportSqlType) Error() string {
	return e.msg
}

func (i *driverImpl) getRollbackSQL(ctx context.Context, rawStmt *pg_query.RawStmt) (string, error) {
	node := rawStmt.GetStmt().GetNode()
	switch stmt := node.(type) {
	// DDL
	case *pg_query.Node_CreateStmt:
		return i.getRollbackSQLForCreateStmt(ctx, stmt)
	case *pg_query.Node_IndexStmt:
		return i.getRollbackSQLForCreateIndexStmt(ctx, stmt)
	case *pg_query.Node_DropStmt:
		return i.getRollbackSQLForDropStmt(ctx, stmt)
	case *pg_query.Node_AlterTableStmt:
		return i.getRollbackSQLForAlterTable(ctx, stmt)
	case *pg_query.Node_RenameStmt:
		return i.getRollbackSQLForRename(ctx, stmt)
	// DML
	case *pg_query.Node_InsertStmt:
		return i.getRollbackSQLForInsertStmt(ctx, stmt)
	case *pg_query.Node_UpdateStmt:
		return i.getRollbackSQLForUpdateStmt(ctx, stmt)
	case *pg_query.Node_DeleteStmt:
		return i.getRollbackSQLForDeleteStmt(ctx, stmt)
	default:
		return "", UnSupportSqlType{msg: "目前还不支持生成该类型的回滚语句"}
	}
}

func (i *driverImpl) getRollbackSQLForCreateIndexStmt(_ context.Context, stmt *pg_query.Node_IndexStmt) (string, error) {
	schema := stmt.IndexStmt.GetRelation().GetSchemaname()
	if schema == "" {
		schema = i.currentSchema
	}
	idxName := stmt.IndexStmt.GetIdxname()
	rollbackSQL := fmt.Sprintf("DROP INDEX IF EXISTS %s.%s;", schema, idxName)
	return rollbackSQL, nil
}

func (i *driverImpl) getRollbackSQLForRename(_ context.Context, stmt *pg_query.Node_RenameStmt) (string, error) {
	relation := stmt.RenameStmt.GetRelation()
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = i.currentSchema
	}

	switch stmt.RenameStmt.GetRenameType() {
	case pg_query.ObjectType_OBJECT_TABLE:
		oldTableName := relation.GetRelname()
		newTableName := stmt.RenameStmt.GetNewname()
		return fmt.Sprintf("ALTER TABLE %s.%s RENAME TO %s;", schema, newTableName, oldTableName), nil
	case pg_query.ObjectType_OBJECT_COLUMN:
		tableName := relation.GetRelname()
		oldColName := stmt.RenameStmt.GetSubname()
		newColName := stmt.RenameStmt.GetNewname()
		return fmt.Sprintf("ALTER TABLE %s.%s RENAME COLUMN %s TO %s;", schema, tableName, newColName, oldColName), nil
	default:
		return "", UnSupportSqlType{msg: "目前还不支持生成该RENAME类型的回滚语句"}
	}
}

func (i *driverImpl) getRollbackSQLForAlterTable(ctx context.Context, stmt *pg_query.Node_AlterTableStmt) (string, error) {
	relation := stmt.AlterTableStmt.GetRelation()
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = i.currentSchema
	}
	table := relation.GetRelname()

	cmdList := stmt.AlterTableStmt.GetCmds()
	for _, cmd := range cmdList {
		subType := cmd.GetAlterTableCmd().GetSubtype()
		switch subType {
		case pg_query.AlterTableType_AT_AddColumn:
			column := cmd.GetAlterTableCmd().GetDef().GetColumnDef().GetColname()
			return fmt.Sprintf("ALTER TABLE %s.%s DROP COLUMN %s;", schema, table, column), nil
		case pg_query.AlterTableType_AT_AddConstraint:
			indexName := cmd.GetAlterTableCmd().GetDef().GetConstraint().GetConname()
			return fmt.Sprintf("DROP INDEX %s.%s;", schema, indexName), nil
		case pg_query.AlterTableType_AT_DropColumn:
			colName := cmd.GetColumnDef().GetColname()
			colDef, err := i.executor.GetColDef(schema, table, colName)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("ALTER TABLE %s.%s ADD COLUMN %s;", schema, table, colDef), nil
		case pg_query.AlterTableType_AT_DropConstraint:
			constraintName := cmd.GetAlterTableCmd().GetName()
			indexDef, err := i.GetIndexDefByCache(ctx, schema, constraintName)
			if err != nil {
				return "", err
			}
			return indexDef, nil
		default:
			return "", UnSupportSqlType{msg: "目前还不支持生成该ALTER类型的回滚语句"}
		}
	}

	return "", nil
}

func (i *driverImpl) getRollbackSQLForCreateStmt(_ context.Context, stmt *pg_query.Node_CreateStmt) (string, error) {
	relation := stmt.CreateStmt.GetRelation()
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = i.currentSchema
	}
	table := relation.GetRelname()
	rollbackSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s;", schema, table)
	return rollbackSQL, nil
}

// drop statement rollback sql
func (i *driverImpl) getRollbackSQLForDropStmt(ctx context.Context, stmt *pg_query.Node_DropStmt) (string, error) {
	object := stmt.DropStmt.Objects[0]
	nodeList, ok := object.GetNode().(*pg_query.Node_List)
	if !ok {
		return "", errors.New("sql error")
	}
	itemList := nodeList.List.GetItems()

	rmType := stmt.DropStmt.RemoveType
	switch rmType {
	case pg_query.ObjectType_OBJECT_TABLE:
		schema, table, err := getSchemaAndTableNameForDropTable(itemList)
		if err != nil {
			return "", err
		}
		if schema == "" {
			schema = i.currentSchema
		}

		tableDDL, err := i.GetTableDDL(ctx, schema, table)
		if err != nil {
			return "", err
		}

		return tableDDL, nil
	case pg_query.ObjectType_OBJECT_INDEX:
		schema, index, err := getSchemaAndTableNameForDropIndex(itemList)
		if err != nil {
			return "", err
		}
		if schema == "" {
			schema = i.currentSchema
		}

		indexDef, err := i.GetIndexDefByCache(ctx, schema, index)
		if err != nil {
			return "", err
		}

		return indexDef, nil
	default:
		return "", UnSupportSqlType{msg: fmt.Sprintf("目前还不支持生成DROP %v类型的回滚语句", rmType)}
	}
}

func getSchemaAndTableNameForDropIndex(itemList []*pg_query.Node) (string, string, error) {
	var schema, index string
	if len(itemList) == 1 {
		index = itemList[0].GetNode().(*pg_query.Node_String_).String_.GetSval()
	} else if len(itemList) == 2 {
		schema = itemList[0].GetNode().(*pg_query.Node_String_).String_.GetSval()
		index = itemList[1].GetNode().(*pg_query.Node_String_).String_.GetSval()
	} else {
		return "", "", errors.New("sql error")
	}
	return schema, index, nil
}

func getSchemaAndTableNameForDropTable(itemList []*pg_query.Node) (string, string, error) {
	var schema, table string
	if len(itemList) == 1 {
		table = itemList[0].GetNode().(*pg_query.Node_String_).String_.GetSval()
	} else if len(itemList) == 2 {
		schema = itemList[0].GetNode().(*pg_query.Node_String_).String_.GetSval()
		table = itemList[1].GetNode().(*pg_query.Node_String_).String_.GetSval()
	} else {
		return "", "", errors.New("sql error")
	}
	return schema, table, nil
}

// insert statement rollback sql
func (i *driverImpl) getRollbackSQLForInsertStmt(ctx context.Context, stmt *pg_query.Node_InsertStmt) (string, error) {
	relation := stmt.InsertStmt.Relation
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = i.currentSchema
	}
	table := relation.GetRelname()

	pkList, err := i.GetPkListByCache(ctx, schema, table)
	if err != nil {
		return "", err
	}
	if len(pkList) == 0 {
		return "", UnSupportSqlType{msg: "在没有主键的表中，不支持生成回滚语句来撤销INSERT语句所做的更改"}
	}

	// pg_query 语法树中的 insert 语句的 values 语法树，虽然名字是 Node_SelectStmt ，但是实际上是 insert 语句的 values 语法树
	selectStmt, ok := stmt.InsertStmt.GetSelectStmt().GetNode().(*pg_query.Node_SelectStmt)
	if !ok {
		return "", errors.New("sql error")
	}
	valueListStmt := selectStmt.SelectStmt.GetValuesLists()

	// get insert value list , example:
	// input : insert into test values (1,2,3),(4,5,6); sql 的 values 语法树
	// output: insert value 值的二维数组 [[1,2,3],[4,5,6]]
	valueList, err := getInsertValueList(valueListStmt)
	if err != nil {
		return "", err
	}
	if i.maxRollbackRowNumConf != -1 && len(valueList) > i.maxRollbackRowNumConf {
		return "", UnSupportSqlType{msg: fmt.Sprintf("当受影响的行数超过了预设的最大值%d时,系统将不生成回滚语句", i.maxRollbackRowNumConf)}
	}

	var rollbackSqlList []string
	for _, value := range valueList {
		var whereConditionList []string
		for i, col := range stmt.InsertStmt.Cols {
			colName := col.GetNode().(*pg_query.Node_ResTarget).ResTarget.Name
			for _, pk := range pkList {
				if colName == pk {
					whereCondition := fmt.Sprintf("%s = '%s'", colName, value[i])
					whereConditionList = append(whereConditionList, whereCondition)
				}
			}
		}

		if len(whereConditionList) != len(pkList) {
			return "", UnSupportSqlType{msg: "在没有主键的表中，不支持生成回滚语句来撤销INSERT语句所做的更改"}
		}

		where := strings.Join(whereConditionList, " AND ")
		rollbackSQL := fmt.Sprintf("DELETE FROM %s.%s WHERE %s;", schema, table, where)
		rollbackSqlList = append(rollbackSqlList, rollbackSQL)
	}

	return strings.Join(rollbackSqlList, "\n"), nil
}

func getInsertValueList(valuesList []*pg_query.Node) ([][]string, error) {
	valueList := make([][]string, 0, len(valuesList))
	for _, value := range valuesList {
		nodeList, ok := value.GetNode().(*pg_query.Node_List)
		if !ok {
			return nil, errors.New("sql error")
		}
		itemList := nodeList.List.GetItems()

		var valList []string
		for _, item := range itemList {
			aConst, ok := item.GetNode().(*pg_query.Node_AConst)
			if !ok {
				return nil, errors.New("sql error")
			}

			constStr := getStrFromNodeAConst(aConst)
			valList = append(valList, constStr)
		}

		valueList = append(valueList, valList)
	}

	return valueList, nil
}

func getStrFromNodeAConst(aConst *pg_query.Node_AConst) string {
	var val string
	node := aConst.AConst.GetVal()
	switch stmt := node.(type) {
	case *pg_query.A_Const_Ival:
		val = strconv.Itoa(int(stmt.Ival.GetIval()))
	case *pg_query.A_Const_Fval:
		val = stmt.Fval.GetFval()
	case *pg_query.A_Const_Sval:
		val = stmt.Sval.GetSval()
	case *pg_query.A_Const_Bsval:
		val = stmt.Bsval.GetBsval()
	}

	if aConst.AConst.GetIsnull() {
		val = "NULL"
	}

	return val
}

// update statement rollback sql
func (i *driverImpl) getRollbackSQLForUpdateStmt(ctx context.Context, stmt *pg_query.Node_UpdateStmt) (string, error) {
	relation := stmt.UpdateStmt.GetRelation()
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = i.currentSchema
	}
	table := relation.GetRelname()

	pkList, err := i.GetPkListByCache(ctx, schema, table)
	if err != nil {
		return "", err
	}
	if len(pkList) == 0 {
		return "", UnSupportSqlType{msg: "在没有主键的表中，不支持生成回滚语句来撤销UPDATE语句所做的更改"}
	}

	var querySQL string
	// check max rollback row num
	{
		whereClause := stmt.UpdateStmt.GetWhereClause()
		querySQL, err = executor.GetRecordListQuerySQL(schema, table, whereClause)
		if err != nil {
			return "", err
		}
		affectRowNum, err := i.getAffectRowNum(querySQL)
		if err != nil {
			return "", err
		}
		if i.maxRollbackRowNumConf != -1 && affectRowNum > i.maxRollbackRowNumConf {
			return "", UnSupportSqlType{msg: fmt.Sprintf("当受影响的行数超过了预设的最大值%d时,系统将不生成回滚语句", i.maxRollbackRowNumConf)}
		}
	}

	oldSetColNameMap := make(map[string]string, 0)
	for _, target := range stmt.UpdateStmt.GetTargetList() {
		resTarget := target.GetNode().(*pg_query.Node_ResTarget)
		aConst := resTarget.ResTarget.GetVal().GetNode().(*pg_query.Node_AConst)
		constStr := getStrFromNodeAConst(aConst)

		oldColName := resTarget.ResTarget.GetName()
		oldSetColNameMap[oldColName] = constStr
	}

	rowList, err := i.executor.GetRecordList(ctx, querySQL)
	if err != nil {
		return "", err
	}

	var rollbackSqlList []string
	for _, row := range rowList {
		var setList []string
		var whereConditionList []string
		for colName, colValue := range row {
			var isPk bool
			for _, pk := range pkList {
				if colName == pk {
					isPk = true
				}
			}

			var isColChange bool
			var isPkChange bool
			for oldSetColName := range oldSetColNameMap {
				if colName == oldSetColName {
					isColChange = true
					for _, pk := range pkList {
						if oldSetColName == pk {
							isPkChange = true
						}
					}
				}
			}

			if isColChange {
				set := fmt.Sprintf("%s = '%s'", colName, colValue)
				setList = append(setList, set)
			}

			if isPk {
				if isPkChange {
					whereCondition := fmt.Sprintf("%s = '%s'", colName, oldSetColNameMap[colName])
					whereConditionList = append(whereConditionList, whereCondition)
				} else {
					whereCondition := fmt.Sprintf("%s = '%s'", colName, colValue)
					whereConditionList = append(whereConditionList, whereCondition)
				}
			}
		}

		if len(whereConditionList) != len(pkList) {
			return "", UnSupportSqlType{msg: "在没有主键的表中,不支持生成回滚语句来撤销UPDATE语句所做的更改"}
		}

		where := strings.Join(whereConditionList, " AND ")
		set := strings.Join(setList, ",")
		rollbackSQL := fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s;", schema, table, set, where)
		rollbackSqlList = append(rollbackSqlList, rollbackSQL)
	}

	return strings.Join(rollbackSqlList, "\n"), nil
}

// delete statement rollback sql
func (i *driverImpl) getRollbackSQLForDeleteStmt(ctx context.Context, stmt *pg_query.Node_DeleteStmt) (string, error) {
	relation := stmt.DeleteStmt.GetRelation()
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = i.currentSchema
	}
	table := relation.GetRelname()

	var querySQL string
	var err error
	{
		whereClause := stmt.DeleteStmt.GetWhereClause()
		querySQL, err = executor.GetRecordListQuerySQL(schema, table, whereClause)
		if err != nil {
			return "", err
		}

		affectRowNum, err := i.getAffectRowNum(querySQL)
		if err != nil {
			return "", err
		}

		if i.maxRollbackRowNumConf != -1 && affectRowNum > i.maxRollbackRowNumConf {
			return "", UnSupportSqlType{msg: fmt.Sprintf("当受影响的行数超过了预设的最大值%d时,系统将不生成回滚语句", i.maxRollbackRowNumConf)}
		}
	}

	rowList, err := i.executor.GetRecordList(ctx, querySQL)
	if err != nil {
		return "", err
	}

	var once sync.Once
	var columnNameList []string
	var newValueList []string
	for _, row := range rowList {
		once.Do(func() {
			for k := range row {
				columnNameList = append(columnNameList, k)
			}
		})

		valueList := make([]string, 0, len(row))
		for _, v := range row {
			var newValue string
			if v == "" {
				newValue = "NULL"
			} else {
				newValue = fmt.Sprintf("'%s'", v)
			}
			valueList = append(valueList, newValue)
		}
		valueListStr := strings.Join(valueList, ",")
		newValueList = append(newValueList, valueListStr)
	}

	insertColList := strings.Join(columnNameList, ",")
	newValueListStr := strings.Join(newValueList, "),(")
	rollbackSQL := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s);", schema, table, insertColList, newValueListStr)

	return rollbackSQL, nil
}
