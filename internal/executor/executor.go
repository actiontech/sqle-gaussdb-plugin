package executor

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/errors"
	hclog "github.com/hashicorp/go-hclog"
	_ "github.com/jackc/pgx/v4/stdlib"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

const (
	ConnectTimeOut = 5
	DialTimeOut    = 5 * time.Second
)

type Db interface {
	Close()
	Ping() error
	Exec(query string) (driver.Result, error)
	Transact(qs ...string) ([]driver.Result, error)
	RollbackQuery(query string, args ...interface{}) ([]map[string]sql.NullString, error)
	Query(query string, args ...interface{}) ([]map[string]sql.NullString, error)
	Logger() hclog.Logger
}

// *logrus.Entry
type BaseConn struct {
	log        hclog.Logger
	host       string
	port       string
	user       string
	db         *sql.DB
	conn       *sql.Conn
	MockOutput sql.NullString
}

func (c *BaseConn) Close() {
	_ = c.conn.Close()
	_ = c.db.Close()
}

func (c *BaseConn) Ping() error {
	c.Logger().Info("ping", "host", c.host, "port", c.port)
	ctx, cancel := context.WithTimeout(context.Background(), DialTimeOut)
	defer cancel()
	err := c.conn.PingContext(ctx)
	if err != nil {
		c.Logger().Info("ping failed", "host", c.host, "port", c.port, "err", err)
	} else {
		c.Logger().Info("ping success", "host", c.host, "port", c.port)
	}
	return errors.New(errors.ConnectRemoteDatabaseError, err)
}

func (c *BaseConn) Exec(query string) (driver.Result, error) {
	result, err := c.conn.ExecContext(context.Background(), query)
	if err != nil {
		c.Logger().Error("exec sql failed", "host", c.host,
			"port", c.port,
			"user", c.user,
			"query", query,
			"err", err.Error())
	} else {
		c.Logger().Info("exec sql success", "host", c.host,
			"port", c.port,
			"user", c.user,
			"query", query)
	}
	c.Logger()
	return result, errors.New(errors.ConnectRemoteDatabaseError, err)
}

func (c *BaseConn) Transact(qs ...string) ([]driver.Result, error) {
	var err error
	var tx *sql.Tx
	var results []driver.Result
	c.Logger().Info("doing sql transact", "host", c.host, "port", c.port, "user", c.user)
	tx, err = c.conn.BeginTx(context.Background(), nil)
	if err != nil {
		return results, err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			c.Logger().Error("rollback sql transact")
			panic(p)
		}
		if err != nil {
			tx.Rollback()
			c.Logger().Error("rollback sql transact")
			return
		}
		err = tx.Commit()
		if err != nil {
			c.Logger().Error("transact commit failed")
		} else {
			c.Logger().Info("done sql transact")
		}
	}()
	for _, query := range qs {
		var txResult driver.Result
		txResult, err = tx.Exec(query)
		if err != nil {
			c.Logger().Error("exec sql failed", "err", err, "query", query)
			return results, err
		} else {
			results = append(results, txResult)
			c.Logger().Info("exec sql success", "query", query)
		}
	}
	return results, nil
}

func (c *BaseConn) RollbackQuery(query string, args ...interface{}) (result []map[string]sql.NullString, err error) {
	var tx *sql.Tx
	c.Logger().Info("doing rollback query", "host", c.host, "port", c.port, "user", c.user)
	tx, err = c.conn.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		tx.Rollback()
		c.Logger().Error("rollback query done")
	}()
	result, err = c.Query(query, args...)
	return result, err
}

func (c *BaseConn) Query(query string, args ...interface{}) ([]map[string]sql.NullString, error) {
	rows, err := c.conn.QueryContext(context.Background(), query, args...)
	if err != nil {
		c.Logger().Error("query sql failed;", "host", c.host,
			"port", c.port,
			"user", c.user,
			"query", query,
			"err", err.Error(),
		)
		return nil, errors.New(errors.ConnectRemoteDatabaseError, err)
	} else {
		c.Logger().Info("query sql success;", "host", c.host,
			"port", c.port,
			"user", c.user,
			"query", query,
		)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		// unknown error
		c.Logger().Error(err.Error())
		return nil, err
	}
	result := make([]map[string]sql.NullString, 0)
	for rows.Next() {
		buf := make([]interface{}, len(columns))
		data := make([]sql.NullString, len(columns))
		for i := range buf {
			buf[i] = &data[i]
		}
		if err := rows.Scan(buf...); err != nil {
			c.Logger().Error(err.Error())
			return nil, err
		}
		value := make(map[string]sql.NullString, len(columns))
		for i := 0; i < len(columns); i++ {
			k := columns[i]
			v := data[i]
			value[k] = v
		}
		result = append(result, value)
	}
	return result, nil
}

func (c *BaseConn) Logger() hclog.Logger {
	return c.log
}

type Executor struct {
	Db Db
}

func NewExecutor(entry hclog.Logger, db *sql.DB, conn *sql.Conn, dns *driverV2.DSN) (*Executor, error) {
	var executor = &Executor{}
	executor.Db = &BaseConn{
		log:  entry,
		host: dns.Host,
		port: dns.Port,
		user: dns.User,
		db:   db,
		conn: conn,
	}

	return executor, nil
}

// NewMockExecutor returns a new mock executor.
func NewMockExecutor(entry hclog.Logger) (*Executor, sqlmock.Sqlmock, error) {
	mockDB, handler, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		return nil, nil, err
	}
	mockConn, err := mockDB.Conn(context.TODO())
	if err != nil {
		return nil, nil, err
	}

	var executor = &Executor{}
	executor.Db = &BaseConn{
		log:  entry,
		host: "mockhost",
		port: "mockport",
		user: "mockuser",
		db:   mockDB,
		conn: mockConn,
	}
	return executor, handler, nil
}

// NewMockExecutorForUnitTest returns a new mock executor.
func NewMockExecutorForUnitTest() (*Executor, sqlmock.Sqlmock, error) {
	mockDB, handler, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		return nil, nil, err
	}
	mockConn, err := mockDB.Conn(context.TODO())
	if err != nil {
		return nil, nil, err
	}

	var executor = &Executor{}
	executor.Db = &BaseConn{
		log: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Trace,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
		host: "mockhost",
		port: "mockport",
		user: "mockuser",
		db:   mockDB,
		conn: mockConn,
	}
	for i := 0; i < 3; i++ {
		columnsIndex := []string{"indexname", "indexdef"}
		rowsIndex := sqlmock.NewRows(columnsIndex)
		handler.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").WillReturnRows(rowsIndex)
		handler.ExpectQuery(`SELECT
		kcu.column_name,
		kcu.constraint_name,
		tc.constraint_type
	FROM 
		information_schema.table_constraints AS tc 
	JOIN information_schema.key_column_usage AS kcu
	ON tc.constraint_name = kcu.constraint_name
	AND tc.table_schema = kcu.table_schema
	WHERE tc.table_name = $1 AND tc.table_schema = $2;
`).WillReturnRows(sqlmock.NewRows([]string{"column_name", "constraint_name", "constraint_type"}))

		handler.ExpectQuery("SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)").
			WillReturnRows(handler.NewRows([]string{"typname"}).AddRow("int4").AddRow("varchar").AddRow("date"))
		handler.ExpectQuery("SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'").
			WillReturnRows(handler.NewRows([]string{"table_name"}).AddRow("exist_tb_1").AddRow("exist_tb_2").AddRow("exist_tb_3").AddRow("exist_tb_9"))
		handler.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").
			WillReturnRows(sqlmock.NewRows([]string{"schema_name"}).AddRow("test"))

	}
	handler.MatchExpectationsInOrder(false)
	return executor, handler, nil
}

//func Ping(entry hclog.Logger, instance *mdriver.DSN) error {
//	conn, err := NewExecutor(entry, instance, "")
//	if err != nil {
//		return err
//	}
//	defer conn.Db.Close()
//	return conn.Db.Ping()
//}

func (c *Executor) ShowDatabases(ignoreSysDatabase bool) ([]string, error) {
	var query string
	if ignoreSysDatabase {
		query = "SELECT datname FROM pg_database WHERE datname NOT IN ('template1', 'template0');"
	} else {
		query = "SELECT datname FROM pg_database;"
	}
	result, err := c.Db.Query(query)
	if err != nil {
		return nil, err
	}
	dbs := make([]string, len(result))
	for n, v := range result {
		if len(v) != 1 {
			err := fmt.Errorf("show databases error, result not match")
			c.Db.Logger().Error(err.Error())
			return dbs, errors.New(errors.ConnectRemoteDatabaseError, err)
		}
		for _, db := range v {
			dbs[n] = db.String
			break
		}
	}
	return dbs, nil
}

func (c *Executor) GetSchemas(database string, ignoreSysSchema bool) ([]string, error) {
	var query string
	var result []map[string]sql.NullString
	var err error
	if ignoreSysSchema {
		query = "select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;"
		result, err = c.Db.Query(query, database, "pg_%", "information_schema")
	} else {
		query = "select schema_name from information_schema.schemata where catalog_name = $1;"
		result, err = c.Db.Query(query, database)
	}
	if err != nil {
		return nil, err
	}
	schemas := make([]string, len(result))
	for n, v := range result {
		if len(v) != 1 {
			err := fmt.Errorf("get schemas error, result not match")
			c.Db.Logger().Error(err.Error())
			return schemas, fmt.Errorf("get schemas error, result not match")
		}
		for _, db := range v {
			schemas[n] = db.String
			break
		}
	}
	return schemas, nil
}

func (c *Executor) GetExecutionPlan(sql string) ([]string, error) {
	aliveCheck := c.Db.Ping()
	if aliveCheck != nil {
		return nil, aliveCheck
	}

	// structure sql query plan
	query := "EXPLAIN (FORMAT JSON) " + sql
	results, err := c.Db.Query(query)
	if err != nil {
		return nil, err
	}
	var epo = make([]string, len(results))
	for n, result := range results {
		if len(result) != 1 {
			err := fmt.Errorf("execute query plan error, result not match")
			c.Db.Logger().Error(err.Error())
			return nil, errors.New(errors.ConnectRemoteDatabaseError, err)
		}

		for _, val := range result {
			epo[n] = val.String
		}
	}
	return epo, nil
}

func (c *Executor) GetExecutionPlanNoJson(sql string) ([]string, error) {
	aliveCheck := c.Db.Ping()
	if aliveCheck != nil {
		return nil, aliveCheck
	}

	// structure sql query plan
	query := "EXPLAIN " + sql
	results, err := c.Db.Query(query)
	if err != nil {
		return nil, err
	}
	var epo = make([]string, len(results))
	for n, result := range results {
		if len(result) != 1 {
			err := fmt.Errorf("execute query plan no json error, result not match")
			c.Db.Logger().Error(err.Error())
			return nil, errors.New(errors.ConnectRemoteDatabaseError, err)
		}

		for _, val := range result {
			epo[n] = val.String
		}
	}
	return epo, nil
}

func (c *Executor) GetExecutionAnalyzePlan(sql string) ([]string, error) {
	aliveCheck := c.Db.Ping()
	if aliveCheck != nil {
		return nil, aliveCheck
	}

	// structure sql query plan
	query := "EXPLAIN (ANALYZE, FORMAT JSON) " + sql
	results, err := c.Db.Query(query)
	if err != nil {
		return nil, err
	}
	var epo = make([]string, len(results))
	for n, result := range results {
		if len(result) != 1 {
			err := fmt.Errorf("execute query plan error, result not match")
			c.Db.Logger().Error(err.Error())
			return nil, errors.New(errors.ConnectRemoteDatabaseError, err)
		}

		for _, val := range result {
			epo[n] = val.String
		}
	}
	return epo, nil
}

func (c *Executor) GetTableNamesBySchemaName(schemaName string) ([]string, error) {
	var query string
	query = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	result, err := c.Db.Query(query, schemaName)
	if err != nil {
		return nil, err
	}
	tableNames := make([]string, len(result))
	for n, v := range result {
		if len(v) != 1 {
			continue
		}
		for _, tableName := range v {
			tableNames[n] = tableName.String
			break
		}
	}
	return tableNames, nil
}

func (c *Executor) GetViewNamesBySchemaName(schemaName string) ([]string, error) {
	var query string
	query = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'VIEW'"
	result, err := c.Db.Query(query, schemaName)
	if err != nil {
		return nil, err
	}
	tableNames := make([]string, len(result))
	for n, v := range result {
		if len(v) != 1 {
			continue
		}
		for _, tableName := range v {
			tableNames[n] = tableName.String
			break
		}
	}
	return tableNames, nil
}

type TableColumnsInfo struct {
	ColumnName             string
	ColumnType             string
	ColumnTypType          string // 列类型类型，用于指示特定数据类型的类别,如 'b', 'c', 'd', 'e', 'p', 'r', 'm'
	CharacterSetName       string
	IsNullable             string
	ColumnDefault          string
	NumericPrecision       string
	NumericScale           string
	CharacterMaximumLength string
}

const (
	Column_DataType_USER_DEFINED = "USER-DEFINED"
)
const (
	Column_Typtype_Enum = "e"
)

func (c *Executor) GetTableColumnsInfo(schema, tableName string) ([]*TableColumnsInfo, error) {
	query := `
	SELECT c.column_name, c.data_type, c.character_set_name, c.is_nullable, c.column_default, c.numeric_precision, 
	c.numeric_scale, c.character_maximum_length, t.typtype
	FROM information_schema.columns AS c
	JOIN pg_catalog.pg_type AS t ON t.oid = c.udt_name::regtype
	WHERE c.table_schema = $1 AND c.table_name = $2 
`
	records, err := c.Db.Query(query, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("get table columns info error, %s", err.Error())
	}

	ret := make([]*TableColumnsInfo, len(records))
	for i, record := range records {
		ret[i] = &TableColumnsInfo{
			ColumnName:             record["column_name"].String,
			ColumnType:             record["data_type"].String,
			CharacterSetName:       record["character_set_name"].String,
			IsNullable:             record["is_nullable"].String,
			ColumnDefault:          record["column_default"].String,
			NumericPrecision:       record["numeric_precision"].String,
			NumericScale:           record["numeric_scale"].String,
			CharacterMaximumLength: record["character_maximum_length"].String,
			ColumnTypType:          record["typtype"].String,
		}
	}

	return ret, nil
}

func (c *Executor) GetTableColumnsInfoBatch(schema string) (map[string][]*TableColumnsInfo, error) {
	querySql := `
	SELECT c.table_name, c.column_name, c.data_type, c.character_set_name, c.is_nullable, c.column_default, c.numeric_precision, 
	c.numeric_scale, c.character_maximum_length, t.typtype
	FROM information_schema.columns AS c
	JOIN pg_catalog.pg_type AS t ON t.oid = c.udt_name::regtype
	WHERE c.table_schema = $1 
`
	records, err := c.Db.Query(querySql, schema)
	if err != nil {
		return nil, fmt.Errorf("get schema=%s table columns info error, %s", schema, err.Error())
	}

	ret := make(map[string][]*TableColumnsInfo)
	for _, record := range records {
		tableName := record["table_name"].String
		tableColumnsInfo := &TableColumnsInfo{
			ColumnName:             record["column_name"].String,
			ColumnType:             record["data_type"].String,
			CharacterSetName:       record["character_set_name"].String,
			IsNullable:             record["is_nullable"].String,
			ColumnDefault:          record["column_default"].String,
			NumericPrecision:       record["numeric_precision"].String,
			NumericScale:           record["numeric_scale"].String,
			CharacterMaximumLength: record["character_maximum_length"].String,
			ColumnTypType:          record["typtype"].String,
		}
		// 检查map中是否已存在该table_name的key，若不存在则初始化一个新的map存储该表的列值
		if _, ok := ret[tableName]; !ok {
			ret[tableName] = make([]*TableColumnsInfo, 0)
		}
		ret[tableName] = append(ret[tableName], tableColumnsInfo)
	}

	return ret, nil
}

type TableConstraintsInfo struct {
	ConstraintName    string
	ConstraintType    string
	Columns           string
	ReferencedTable   string
	ReferencedColumns string
	CheckCondition    string
}

func (c *Executor) GetTableConstraintsInfo(schema, tableName string) ([]*TableConstraintsInfo, error) {
	query := `
        SELECT 
			c.conname AS constraint_name, 
			c.contype AS constraint_type,  
			CASE 
				WHEN c.contype IN ('p', 'u') THEN string_agg(a.attname, ',') 
				WHEN c.contype = 'f' THEN string_agg(a.attname, ',')  
				ELSE null 
			END as column_names, 
			CASE 
				WHEN c.contype = 'f' THEN c.confrelid::regclass 
				ELSE null 
			END as referenced_table, 
			CASE 
				WHEN c.contype = 'f' THEN string_agg(ac.attname, ',') 
				ELSE null 
			END as referenced_columns, 
			CASE 
				WHEN c.contype = 'c' THEN pg_get_constraintdef(c.oid) 
				ELSE null 
			END as check_condition   
		FROM pg_constraint c 
		JOIN pg_class t ON c.conrelid = t.oid 
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey) 
		JOIN pg_namespace n ON t.relnamespace = n.oid   
		LEFT JOIN pg_class rc ON c.confrelid = rc.oid 
		LEFT JOIN pg_attribute ac ON ac.attrelid = rc.oid AND ac.attnum = ANY (c.confkey) 
		WHERE n.nspname = $1 AND t.relname = $2   
		GROUP BY c.conname, c.contype, c.confrelid, c.oid
    `
	records, err := c.Db.Query(query, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("get table constraints info error, %s", err.Error())
	}

	ret := make([]*TableConstraintsInfo, len(records))
	for i, record := range records {
		ret[i] = &TableConstraintsInfo{
			ConstraintName:    record["constraint_name"].String,
			ConstraintType:    record["constraint_type"].String,
			Columns:           record["column_names"].String,
			ReferencedTable:   record["referenced_table"].String,
			ReferencedColumns: record["referenced_columns"].String,
			CheckCondition:    record["check_condition"].String,
		}
	}
	return ret, nil
}

func (c *Executor) GetTableConstraintsInfoBatch(schema string) (map[string][]*TableConstraintsInfo, error) {
	query := `
        SELECT 
			t.relname AS table_name, 
			c.conname AS constraint_name, 
			c.contype AS constraint_type,  
			CASE 
				WHEN c.contype IN ('p', 'u') THEN string_agg(a.attname, ',') 
				WHEN c.contype = 'f' THEN string_agg(a.attname, ',')  
				ELSE null 
			END as column_names, 
			CASE 
				WHEN c.contype = 'f' THEN c.confrelid::regclass 
				ELSE null 
			END as referenced_table, 
			CASE 
				WHEN c.contype = 'f' THEN string_agg(ac.attname, ',') 
				ELSE null 
			END as referenced_columns, 
			CASE 
				WHEN c.contype = 'c' THEN pg_get_constraintdef(c.oid) 
				ELSE null 
			END as check_condition   
		FROM pg_constraint c 
		JOIN pg_class t ON c.conrelid = t.oid 
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey) 
		JOIN pg_namespace n ON t.relnamespace = n.oid   
		LEFT JOIN pg_class rc ON c.confrelid = rc.oid 
		LEFT JOIN pg_attribute ac ON ac.attrelid = rc.oid AND ac.attnum = ANY (c.confkey) 
		WHERE n.nspname = $1   
		GROUP BY t.relname, c.conname, c.contype, c.confrelid, c.oid
		HAVING c.contype IN('p', 'u', 'f', 'c')
    `
	records, err := c.Db.Query(query, schema)
	if err != nil {
		return nil, fmt.Errorf("get table constraints info error, %s", err.Error())
	}

	ret := make(map[string][]*TableConstraintsInfo)
	for _, record := range records {
		tableName := record["table_name"].String
		tableConstraintsInfo := &TableConstraintsInfo{
			ConstraintName:    record["constraint_name"].String,
			ConstraintType:    record["constraint_type"].String,
			Columns:           record["column_names"].String,
			ReferencedTable:   record["referenced_table"].String,
			ReferencedColumns: record["referenced_columns"].String,
			CheckCondition:    record["check_condition"].String,
		}
		// 检查map中是否已存在该table_name的key，若不存在则初始化一个新的map存储该表约束值
		if _, ok := ret[tableName]; !ok {
			ret[tableName] = make([]*TableConstraintsInfo, 0)
		}
		ret[tableName] = append(ret[tableName], tableConstraintsInfo)
	}

	return ret, nil
}

type TableIndexesInfo struct {
	IndexName string
	IndexDDL  string
}

func (c *Executor) GetTableIndexesInfo(schema, tableName string) ([]*TableIndexesInfo, error) {
	query := `SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2`
	records, err := c.Db.Query(query, schema, tableName)
	if err != nil {
		return nil, err
	}

	ret := make([]*TableIndexesInfo, len(records))
	for i, record := range records {
		ret[i] = &TableIndexesInfo{
			IndexName: record["indexname"].String,
			IndexDDL:  record["indexdef"].String,
		}
	}
	return ret, nil
}

func (c *Executor) GetTableIndexesInfoBatch(schema string) (map[string][]*TableIndexesInfo, error) {
	sprintf := "SELECT tablename,indexname,indexdef FROM pg_indexes where schemaname = $1"
	records, err := c.Db.Query(sprintf, schema)
	if err != nil {
		return nil, err
	}

	ret := make(map[string][]*TableIndexesInfo)
	for _, record := range records {
		tableName := record["tablename"].String
		tableIndexesInfo := &TableIndexesInfo{
			IndexName: record["indexname"].String,
			IndexDDL:  record["indexdef"].String,
		}
		// 检查map中是否已存在该table_name的key，若不存在则初始化一个新的map存储该表索引值
		if _, ok := ret[tableName]; !ok {
			ret[tableName] = make([]*TableIndexesInfo, 0)
		}
		ret[tableName] = append(ret[tableName], tableIndexesInfo)
	}
	return ret, nil
}

type TableDistributionInfo struct {
	TableName              string
	DistributionColumnName string
}

func (c *Executor) GetTableDistributionInfo(schema string) ([]*TableDistributionInfo, error) {
	// http://10.186.18.21/sqle/sqle-tbase-plugin/-/issues/21: 不同版本的Tbase中pgxc_class中的字段不一样
	// 版本1：PG 10 Tbase_v5.06.1.1   pgxc_class 中的字段是pcattnum
	// 版本2：PG 10 Tbase_v5.21.8.0  pgxc_class 中的字段是DISCOLNUMS
	isTbaseV5_21 := false
	query := `SELECT EXISTS (
    SELECT 1
    FROM pg_attribute
    WHERE attrelid = 'pgxc_class'::regclass
    AND attname = 'discolnums'
);`
	records, err := c.Db.Query(query)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, fmt.Errorf("get table distribution info error, result is %v", records)
	}
	if query, ok := records[0]["exists"]; !ok {
		return nil, fmt.Errorf("get table distribution info error, column \"exists\" not found")
	} else if query.String == "true" {
		isTbaseV5_21 = true
	}

	if isTbaseV5_21 {
		query = `SELECT
	c.relname AS table_name,
	a.attname AS distribution_key
FROM
	pgxc_class pc
JOIN
	pg_class c ON pc.pcrelid = c.oid
JOIN
	pg_attribute a ON c.oid = a.attrelid
JOIN
	pg_namespace n ON c.relnamespace = n.oid
WHERE
	a.attnum =  ANY(pc.discolnums)
	AND n.nspname = $1;`
	} else {
		query = `SELECT
	c.relname AS table_name,
	a.attname AS distribution_key
FROM
	pgxc_class pc
JOIN
	pg_class c ON pc.pcrelid = c.oid
JOIN
	pg_attribute a ON c.oid = a.attrelid
JOIN
	pg_namespace n ON c.relnamespace = n.oid
WHERE
	a.attnum = pc.pcattnum
	AND n.nspname = $1;`
	}

	records, err = c.Db.Query(query, schema)
	if err != nil {
		return nil, err
	}

	ret := make([]*TableDistributionInfo, len(records))
	for i, record := range records {
		ret[i] = &TableDistributionInfo{
			TableName:              record["table_name"].String,
			DistributionColumnName: record["distribution_key"].String,
		}
	}
	return ret, nil
}

func (c *Executor) ComputeIndexCellDivision(schemaName, tableName, indexColumn string) (int, error) {
	sql := fmt.Sprintf(`
        select 
            CASE 
				WHEN COUNT(DISTINCT t.cell_division_number) = 0 OR COUNT(t.*) = 0 THEN 0
				ELSE COUNT(DISTINCT t.cell_division_number) * 100 / NULLIF(COUNT(t.*), 0)
			END AS index_cell_division
	     from 
		(
		  select %s as cell_division_number from %s limit 50000
		) t
    `, indexColumn, fmt.Sprintf("%s.%s", schemaName, tableName))
	result, err := c.Db.Query(sql)
	if err != nil {
		return 0, err
	}
	if len(result) != 1 {
		err := fmt.Errorf("ComputeIndexCellDivision error, result is %v", result)
		c.Db.Logger().Error(err.Error())
		return 0, errors.New(errors.ConnectRemoteDatabaseError, err)
	}
	if query, ok := result[0]["index_cell_division"]; !ok {
		err := fmt.Errorf("ComputeIndexCellDivision error, \"index_cell_division\" value is 0")
		c.Db.Logger().Error(err.Error())
		return 0, errors.New(errors.ConnectRemoteDatabaseError, err)
	} else {
		indexCellDivision, _ := strconv.Atoi(query.String)
		return indexCellDivision, nil
	}
}

func (c *Executor) ShowCreateTable(schema, tableName string) (string, error) {
	query := fmt.Sprintf(`SELECT 'CREATE TABLE ' || relname || E'\n(\n' ||
       array_to_string(
               array_agg(
                       '    ' || column_name || ' ' || type || ' ' || not_null
                   )
           , E',\n'
           ) || E'\n);\n' as ddl
from (SELECT c.relname,
             a.attname                                       AS column_name,
             pg_catalog.format_type(a.atttypid, a.atttypmod) as type,
             case
                 when a.attnotnull
                     then 'NOT NULL'
                 else 'NULL'
                 END                                         as not_null
      FROM pg_class c,
           pg_attribute a,
           pg_type t
      WHERE c.relname = $1
        AND a.attnum > 0
        AND a.attrelid = c.oid
        AND a.atttypid = t.oid
      ORDER BY a.attnum) as tabledefinition
group by relname;`)

	result, err := c.Db.Query(query, tableName)
	if err != nil {
		return "", err
	}
	if len(result) != 1 {
		err := fmt.Errorf("show create table error, result is %v", result)
		c.Db.Logger().Error(err.Error())
		return "", errors.New(errors.ConnectRemoteDatabaseError, err)
	}
	if query, ok := result[0]["ddl"]; !ok {
		err := fmt.Errorf("show create table error, column \"ddl\" not found")
		c.Db.Logger().Error(err.Error())
		return "", errors.New(errors.ConnectRemoteDatabaseError, err)
	} else {
		return query.String, nil
	}
}

func (c *Executor) GetTableFormExecutionPlan(sql string) ([]string, error) {
	sprintf := fmt.Sprintf("EXPLAIN %s", sql)
	records, err := c.Db.Query(sprintf)
	if err != nil {
		return nil, fmt.Errorf("get table columns info error, %s", err.Error())
	}

	ret := make([]string, len(records))
	for i, record := range records {
		ret[i] = record["QUERY PLAN"].String
	}

	return ret, nil
}

func (e *Executor) GetCurrentSchema() (string, error) {
	records, err := e.Db.Query("SHOW search_path")
	if err != nil {
		return "", err
	}
	if len(records) <= 0 {
		return "", fmt.Errorf("cannot get search_path")
	}

	currentPath := records[0]["search_path"].String
	schemas := strings.Split(currentPath, ",")

	// search_path中第一个存在的schema会被pg视为当前默认的schema
	for _, schema := range schemas {
		// 默认的 search_path 第一个值会是"$user"
		if schema == `"$user"` {
			records, err = e.Db.Query(`SELECT quote_literal(user)`)
			if err != nil {
				return "", err
			}
			return strings.Trim(records[0]["quote_literal"].String, `'`), nil
		} else {
			records, err = e.Db.Query(fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = %v)", schema))
			if err != nil {
				return "", err
			}
			if "true" == records[0]["exist"].String {
				return schema, nil
			}
		}

	}

	return "", fmt.Errorf("can not get current schema")
}

// selectCountSql must be like "select count(*) from tb ..."
func (e Executor) EstimateAffectedRows(selectCountSql string) (count int, err error) {
	rows, err := e.Db.Query(selectCountSql)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("got more than one count")
	}
	countStr := rows[0]["count"]
	if count, err = strconv.Atoi(countStr.String); err != nil {
		return 0, fmt.Errorf("got unexpected count, error: %v", err)
	}

	return count, nil
}

func (e *Executor) GetIndexDef(ctx context.Context, schema string, indexName string) (string, error) {
	rowList, err := e.Db.Query(`
SELECT indexdef
FROM pg_indexes
WHERE schemaname = $1
  AND indexname = $2;
`, schema, indexName)
	if err != nil {
		return "", err
	}
	if len(rowList) != 1 {
		return "", fmt.Errorf("got more than %d index", len(rowList))
	}

	row := rowList[0]
	indexDef := row["indexdef"].String
	return indexDef, nil
}

func (e *Executor) GetPkList(_ context.Context, schema string, tableName string) ([]string, error) {
	var pkList []string
	rowList, err := e.Db.Query(`
SELECT a.attname
FROM pg_index i
         JOIN pg_attribute a ON a.attrelid = i.indrelid
WHERE i.indrelid = $1 ::regclass
  AND a.attnum = ANY (i.indkey)
  AND i.indisprimary;`, schema+"."+tableName)
	if err != nil {
		return pkList, err
	}

	for _, row := range rowList {
		pk := row["attname"].String
		pkList = append(pkList, pk)
	}

	return pkList, nil
}

func (e *Executor) GetColDef(schema, table, colName string) (string, error) {
	rowList, err := e.Db.Query(`
SELECT column_name, udt_name, character_maximum_length, is_nullable, column_default
FROM information_schema.columns
WHERE table_name = $1
  AND table_schema = $2
  AND column_name = $3;
`, table, schema, colName)
	if err != nil {
		return "", err
	}

	if len(rowList) != 1 {
		return "", err
	}

	row := rowList[0]
	columnDef := getColumnDefByRow(row)

	return columnDef, nil
}

func getColumnDefByRow(row map[string]sql.NullString) string {
	b := new(bytes.Buffer)
	columnName := row["column_name"].String
	b.Write([]byte(columnName))
	b.Write([]byte(" "))
	colType := row["udt_name"].String
	b.Write([]byte(colType))
	characterMaximumLength := row["character_maximum_length"].String
	if characterMaximumLength != "" {
		fieldLength := fmt.Sprintf("(%s)", characterMaximumLength)
		b.Write([]byte(fieldLength))
	}

	isNullable := row["is_nullable"].String
	if isNullable == "NO" {
		b.Write([]byte(" "))
		b.Write([]byte("NOT NULL"))
	}

	columnDefault := row["column_default"].String
	if columnDefault != "" {
		b.Write([]byte(" "))
		b.Write([]byte("DEFAULT "))
		b.Write([]byte(columnDefault))
	}

	return b.String()
}

func (e *Executor) GetRecordList(ctx context.Context, sql string) ([]map[string]string, error) {
	rows, err := e.Db.Query(sql)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]string, len(rows))
	for i, row := range rows {
		result[i] = make(map[string]string)
		for k, v := range row {
			result[i][k] = v.String
		}
	}

	return result, nil
}

func GetRecordListQuerySQL(schema string, table string, whereClause *parser.Node) (string, error) {
	inputSql := fmt.Sprintf("SELECT * FROM %s.%s", schema, table)
	tree, err := parser.Parse(inputSql)
	if err != nil {
		return "", err
	}

	stmt := tree.Stmts[0].GetStmt().GetSelectStmt()
	if whereClause != nil {
		stmt.WhereClause = whereClause
	}

	query, err := parser.Deparse(&parser.ParseResult{Stmts: []*parser.RawStmt{tree.Stmts[0]}})
	if err != nil {
		return "", err
	}

	return query, err
}

func (e *Executor) GetRecordCountQuerySQL(schema string, table string, whereClause *parser.Node) (count int, err error) {
	inputSql := fmt.Sprintf("SELECT count(*) AS count FROM %s.%s", schema, table)
	tree, err := parser.Parse(inputSql)
	if err != nil {
		return -1, err
	}

	stmt := tree.Stmts[0].GetStmt().GetSelectStmt()
	if whereClause != nil {
		stmt.WhereClause = whereClause
	}

	query, err := parser.Deparse(&parser.ParseResult{Stmts: []*parser.RawStmt{tree.Stmts[0]}})
	if err != nil {
		return -1, err
	}
	rows, err := e.Db.Query(query)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("got more than one count")
	}
	countStr := rows[0]["count"]
	if count, err = strconv.Atoi(countStr.String); err != nil {
		return 0, fmt.Errorf("got unexpected count, error: %v", err)
	}

	return count, nil
}

// GetTableIndexDef get all index definitions for a table
func (e *Executor) GetTableIndexDef(ctx context.Context, schemaName, tableName string) ([]string, error) {
	rowList, err := e.Db.Query(`
SELECT PG_GET_INDEXDEF(indexrelid) AS index_def
FROM pg_index
WHERE indrelid = $1 ::regclass`, fmt.Sprintf("%s.%s", schemaName, tableName))
	if err != nil {
		return nil, fmt.Errorf("get table: %q index def err: %w", fmt.Sprintf("%s.%s", schemaName, tableName), err)
	}

	var indexDefList []string
	for _, row := range rowList {
		indexDef := row["index_def"].String
		indexDefList = append(indexDefList, indexDef)
	}

	return indexDefList, nil
}

func (e *Executor) GetTableColumnDefList(ctx context.Context, schemaName string, tableName string) ([]string, error) {
	rowList, err := e.Db.Query(`
SELECT column_name, udt_name, character_maximum_length, is_nullable, column_default
FROM information_schema.columns
WHERE table_name = $1
  AND table_schema = $2`, tableName, schemaName)
	if err != nil {
		return nil, err
	}

	var columnDefList []string
	for _, row := range rowList {
		columnDef := getColumnDefByRow(row)
		columnDefList = append(columnDefList, columnDef)
	}

	return columnDefList, nil
}

func (e *Executor) ValidateFunctionExistInDb(databaseName, schemaName, functionName string) (bool, error) {
	sql := `
        SELECT 1 FROM information_schema.routines r
		LEFT JOIN information_schema.parameters p 
		ON r.specific_catalog = p.specific_catalog
		AND r.specific_schema = p.specific_schema
		AND r.specific_name = p.specific_name
		WHERE r.routine_type = 'FUNCTION' 
		AND r.routine_catalog = $1 
		AND r.routine_schema = $2 
		AND r.routine_name = $3 
		AND p.specific_name IS NULL;
    `
	data, err := e.Db.Query(sql, databaseName, schemaName, functionName)
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	return true, nil
}

func (e *Executor) GetFunctionInfoList(databaseName, schemaName, functionName string) ([]map[string]sql.NullString, error) {
	query := `
        SELECT r.routine_schema as specific_schema,
			   r.routine_name as specific_name,
			   string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters
		FROM information_schema.routines r
		JOIN information_schema.parameters p
		ON r.specific_catalog = p.specific_catalog
		AND r.specific_schema = p.specific_schema
		AND r.specific_name = p.specific_name
		WHERE r.routine_type = 'FUNCTION'
		AND r.routine_catalog = $1
		AND r.routine_schema = $2
		AND r.routine_name = $3
		AND p.specific_schema NOT IN ('pg_catalog', 'information_schema')
		AND p.parameter_mode = 'IN'
		GROUP BY r.routine_schema, r.routine_name
    `
	data, err := e.Db.Query(query, databaseName, schemaName, functionName)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (e *Executor) ValidateProcedureExistInDb(databaseName, schemaName, procedureName string) (bool, error) {
	sql := `
        SELECT 1 FROM information_schema.routines r LEFT JOIN information_schema.parameters p
		ON r.specific_catalog = p.specific_catalog
		AND r.specific_schema = p.specific_schema
		AND r.specific_name = p.specific_name
		WHERE r.routine_type = 'PROCEDURE' 
		AND r.routine_catalog = $1 
		AND r.routine_schema = $2 
		AND r.routine_name = $3 
		AND p.specific_name IS NULL;
    `
	data, err := e.Db.Query(sql, databaseName, schemaName, procedureName)
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	return true, nil
}

func (e *Executor) GetProcedureInfoList(databaseName, schemaName, procedureName string) ([]map[string]sql.NullString, error) {
	query := `
       SELECT r.routine_schema as specific_schema,
			  r.routine_name as specific_name,
			  string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters, 
			  string_agg(p.parameter_mode, ', ' ORDER BY p.ordinal_position) AS parameters_mode
		FROM information_schema.routines r
		JOIN information_schema.parameters p
		ON r.specific_catalog = p.specific_catalog
		AND r.specific_schema = p.specific_schema
		AND r.specific_name = p.specific_name
		WHERE r.routine_type = 'PROCEDURE' 
		AND r.routine_catalog = $1 
		AND r.routine_schema = $2 
		AND r.routine_name = $3
		AND p.specific_schema NOT IN ('pg_catalog', 'information_schema')
		GROUP BY r.routine_schema, r.routine_name
    `
	data, err := e.Db.Query(query, databaseName, schemaName, procedureName)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// 获取指定schema下的所有视图
func (e *Executor) GetViewListForSchema(schemaName string) ([]string, error) {
	views := make([]string, 0)
	sql := fmt.Sprintf("SELECT viewname FROM pg_catalog.pg_views where schemaname = $1")
	data, err := e.Db.Query(sql, schemaName)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	for _, item := range data {
		view, ok := item["viewname"]
		if ok {
			views = append(views, view.String)
		}
	}
	return views, nil
}

// GetDataTypeNameMap 获取类型map
func (e *Executor) GetDataTypeNameMap(_ context.Context, scheme string) (map[string]string, error) {
	typeNameMap := make(map[string]string)
	if len(scheme) == 0 {
		return typeNameMap, fmt.Errorf("scheme=%s can not be empty", scheme)
	}
	rowList, err := e.Db.Query(`SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n 
		ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)`, scheme)
	if err != nil {
		return typeNameMap, err
	}
	for _, row := range rowList {
		typeName := row["typname"].String
		if _, ok := typeNameMap[typeName]; !ok {
			typeNameMap[typeName] = typeName
		}
	}
	pgDataTypes := []string{"bigserial", "decimal", "mood", "serial", "serial2", "serial4", "serial8", "smallserial", "int", "text", "timestamp", "int8", "int4"}
	for _, dataType := range pgDataTypes {
		typeNameMap[dataType] = dataType
	}
	return typeNameMap, nil
}

type TableColumnConstraintInfo struct {
	ColumnName     string
	ConstraintName string
	ConstraintType ColumnConstraintType
}

type ColumnConstraintType string

const (
	ColumnConstraintTypeCHECK       = "CHECK"
	ColumnConstraintTypeFOREIGN_KEY = "FOREIGN KEY"
	ColumnConstraintTypePRIMARY_KEY = "PRIMARY KEY"
	ColumnConstraintTypeUNIQUE      = "UNIQUE"
)

// TODO: AI生成，需要测试
// TODO: 增加缓存
func (e *Executor) GetTableColumnConstraintInfo(schemaName string, tableName string) ([]*TableColumnConstraintInfo, error) {
	sql := `SELECT
	kcu.column_name,
	kcu.constraint_name,
	tc.constraint_type
FROM 
	information_schema.table_constraints AS tc 
JOIN information_schema.key_column_usage AS kcu
ON tc.constraint_name = kcu.constraint_name
AND tc.table_schema = kcu.table_schema
WHERE tc.table_name = $1 AND tc.table_schema = $2;
`
	records, err := e.Db.Query(sql, tableName, schemaName)
	if err != nil {
		return nil, fmt.Errorf("get table column constraint error, %s", err.Error())
	}

	ret := make([]*TableColumnConstraintInfo, len(records))
	for i, record := range records {
		var typ ColumnConstraintType
		switch record["constraint_type"].String {
		case "CHECK":
			typ = ColumnConstraintTypeCHECK
		case "FOREIGN KEY":
			typ = ColumnConstraintTypeFOREIGN_KEY
		case "PRIMARY KEY":
			typ = ColumnConstraintTypePRIMARY_KEY
		case "UNIQUE":
			typ = ColumnConstraintTypeUNIQUE
		default:
			return nil, fmt.Errorf("parser table column constraint error, unknown type %s", record["constraint_type"].String)
		}
		ret[i] = &TableColumnConstraintInfo{
			ColumnName:     record["column_name"].String,
			ConstraintName: record["constraint_name"].String,
			ConstraintType: typ,
		}
	}

	return ret, nil
}

func (c *Executor) GetTableColumnConstraintInfoBatch(schema string) (map[string][]*TableColumnConstraintInfo, error) {
	sql := `SELECT
	kcu.column_name,
	kcu.constraint_name,
	tc.constraint_type,
	tc.table_name
FROM 
	information_schema.table_constraints AS tc 
JOIN information_schema.key_column_usage AS kcu
ON tc.constraint_name = kcu.constraint_name
AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = $1;
	`
	records, err := c.Db.Query(sql, schema)
	if err != nil {
		return nil, fmt.Errorf("get table column constraint error, %s", err.Error())
	}

	ret := make(map[string][]*TableColumnConstraintInfo)
	for _, record := range records {
		var typ ColumnConstraintType
		switch record["constraint_type"].String {
		case "CHECK":
			typ = ColumnConstraintTypeCHECK
		case "FOREIGN KEY":
			typ = ColumnConstraintTypeFOREIGN_KEY
		case "PRIMARY KEY":
			typ = ColumnConstraintTypePRIMARY_KEY
		case "UNIQUE":
			typ = ColumnConstraintTypeUNIQUE
		default:
			return nil, fmt.Errorf("parser table column constraint error, unknown type %s", record["constraint_type"].String)
		}
		tableName := record["table_name"].String
		info := &TableColumnConstraintInfo{
			ColumnName:     record["column_name"].String,
			ConstraintName: record["constraint_name"].String,
			ConstraintType: typ,
		}

		ret[tableName] = append(ret[tableName], info)
	}

	return ret, nil
}

type TableType string

const (
	RelationTableType      TableType = "relation"
	OriginPartTableType    TableType = "origin_part"
	SelfStudyPartTableType TableType = "self_study_part"
)

// RelKind is the kind of relation
// note: https://github.com/Tencent/TBase/blob/v2.5.0/src/include/catalog/pg_class.h
type RelKind string

const (
	RelKindRelation         RelKind = "r"
	RelKindPartitionedTable RelKind = "p"
)

// RelPartKind is the kind of relation part
// note: https://github.com/Tencent/TBase/blob/v2.5.0/src/include/catalog/pg_class.h
type RelPartKind string

const (
	RelPartKindParent RelPartKind = "p"
	RelPartKindChild  RelPartKind = "c"
	RelPartKindNone   RelPartKind = "n"
)

// GetTableType get table type
func (e *Executor) GetTableType(schemaName, tableName string) (TableType, error) {
	sql := `SELECT c.relkind, c.relpartkind
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = $1
  AND c.relname = $2;`
	rows, err := e.Db.Query(sql, schemaName, tableName)
	if err != nil {
		return "", err
	}
	if len(rows) != 1 {
		return "", fmt.Errorf("got more than one count")
	}

	relKind := rows[0]["relkind"].String
	relPartKind := rows[0]["relpartkind"].String

	if RelKind(relKind) == RelKindRelation && RelPartKind(relPartKind) == RelPartKindNone {
		return RelationTableType, nil
	}

	if RelKind(relKind) == RelKindPartitionedTable && RelPartKind(relPartKind) == RelPartKindNone {
		return OriginPartTableType, nil
	}

	if RelKind(relKind) == RelKindRelation && RelPartKind(relPartKind) == RelPartKindParent {
		return SelfStudyPartTableType, nil
	}

	return "", fmt.Errorf("unknow table type")
}

// TODO: AI生成，需要测试
// TODO: 增加缓存
func (e *Executor) GetTableSizeMB(schemaName, tableName string) (int, error) {
	sql := fmt.Sprintf("SELECT pg_relation_size('%s.%s')/1024/1024 AS table_size", schemaName, tableName)
	rows, err := e.Db.Query(sql)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("got more than one count")
	}
	sizeStr := rows[0]["table_size"]
	size, err := strconv.Atoi(sizeStr.String)
	if err != nil {
		return 0, fmt.Errorf("got unexpected size, error: %v", err)
	}
	return size, nil
}

// TODO: AI生成，需要测试，注意测试空表情况
// TODO: 增加缓存
// getColumnSelectivity calculates the selectivity of a specified column.
func (e *Executor) GetColumnSelectivity(schemaName, tableName string, columnNames []string) (map[string]float64, error) {
	// get total count
	query := fmt.Sprintf(`SELECT COUNT(1) total FROM (SELECT * FROM %s.%s LIMIT 50000)`, pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(tableName))
	rows, err := e.Db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error executing total count query: %v", err)
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("got more than one count")
	}
	totalCount, err := strconv.ParseFloat(rows[0]["total"].String, 64)
	if err != nil {
		return nil, fmt.Errorf("got unexpected count, error: %v", err)
	}

	if totalCount == 0 {
		return nil, fmt.Errorf("total count is zero")
	}

	// Use a parameterized query to avoid SQL injection
	query = `
		SELECT COUNT(*) AS record_count 
		FROM (SELECT %s FROM %s.%s LIMIT 50000) AS limited 
		GROUP BY %s ORDER BY record_count DESC LIMIT 1`
	columnSelectivityMap := make(map[string]float64, len(columnNames))
	for _, col := range columnNames {
		rows, err = e.Db.Query(fmt.Sprintf(query, pq.QuoteIdentifier(col), pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(tableName), pq.QuoteIdentifier(col)))
		if err != nil {
			return nil, fmt.Errorf("error executing max count query: %v", err)
		}
		if len(rows) != 1 {
			return nil, fmt.Errorf("got more than one count")
		}
		maxCount, err := strconv.ParseFloat(rows[0]["record_count"].String, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing max count: %v", err)
		}

		columnSelectivityMap[col] = 1 - maxCount/totalCount
	}

	return columnSelectivityMap, nil
}

// GetTableAutoIncrementColumnDefaultValue 获取表自增列默认值
func (e *Executor) GetTableAutoIncrementColumnDefaultValue(_ context.Context, scheme, tableName string) (map[string]string, error) {
	resultMap := make(map[string]string)
	rowList, err := e.Db.Query(`SELECT column_name,column_default FROM information_schema.columns WHERE table_schema = $1
    AND table_name = $2 AND column_default LIKE 'nextval%'`, scheme, tableName)
	if err != nil {
		return nil, err
	}
	for _, row := range rowList {
		columnName := row["column_name"].String
		columnDefaultValue := row["column_default"].String
		if _, ok := resultMap[columnName]; !ok {
			resultMap[columnName] = columnDefaultValue
		}
	}
	return resultMap, nil
}

func (e *Executor) GetDatabaseDefaultCollate(currentDatabaseName string) (defaultCollate string, err error) {

	// 查询数据库的默认排序规则
	query := fmt.Sprintf("SELECT datcollate FROM pg_database WHERE datname = '%s'", currentDatabaseName)
	rowList, err := e.Db.Query(query)
	if err != nil {
		return "", err
	}
	for _, row := range rowList {
		defaultCollate = row["datcollate"].String
	}
	return defaultCollate, nil
}

// 获取分区表的所有子表
func (e *Executor) GetChildPartitionTableList(schemaName, tableName string) ([]string, error) {
	sql := `SELECT c.relname FROM pg_class c JOIN pg_inherits i ON c.oid = i.inhparent JOIN pg_namespace n ON c.relnamespace = n.oid WHERE c.relname = $1 AND n.nspname = $2`

	rows, err := e.Db.Query(sql, tableName, schemaName)
	if err != nil {
		return nil, err
	}
	var childTableList []string
	for _, row := range rows {
		childTableList = append(childTableList, row["relname"].String)
	}
	return childTableList, nil
}

type SubTable struct {
	SchemaName string
	TableName  string
}

func (c *Executor) GetOriginPartSubTables(schemaName string, tableName string) ([]*SubTable, error) {
	sql := `
 SELECT nsp.nspname AS schema_name,c.relname AS child_name
                          FROM pg_class p1
                          JOIN pg_inherits i1 ON p1.oid = i1.inhparent
                          JOIN pg_class c ON i1.inhrelid = c.oid
                          JOIN pg_namespace nsp on p1.relnamespace = nsp.oid
                          WHERE nsp.nspname = $1 and p1.relname = $2;`
	rows, err := c.Db.Query(sql, schemaName, tableName)
	if err != nil {
		return nil, err
	}

	var subTables []*SubTable
	for _, row := range rows {
		subTables = append(subTables, &SubTable{
			SchemaName: row["schema_name"].String,
			TableName:  row["child_name"].String,
		})
	}

	return subTables, nil
}

func (c *Executor) GetSelfStudyPartSubTables(schemaName string, tableName string) ([]*SubTable, error) {
	sql := `
	SELECT relname AS child_name
FROM pg_class
WHERE relparent = (SELECT c.oid
                   FROM pg_class c
                            JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
                   WHERE nsp.nspname = $1
                     AND c.relname = $2
                     AND c.relpartkind = 'p'
                   LIMIT 1);`

	rows, err := c.Db.Query(sql, schemaName, tableName)
	if err != nil {
		return nil, err
	}

	var subTables []*SubTable
	for _, row := range rows {
		subTables = append(subTables, &SubTable{
			SchemaName: schemaName,
			TableName:  row["child_name"].String,
		})
	}

	return subTables, nil
}

type TableSize struct {
	DataNode   string
	SchemaName string
	TableName  string
	Size       int
}

func (c *Executor) GetTableSizeGBByNode(dataNode string, schemaName string, tableNameList []string) ([]*TableSize, error) {
	sql := `EXECUTE DIRECT ON (%s) 'SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = ''%s''
AND   c.relname IN (%s);'`

	var ss []string
	for _, tableName := range tableNameList {
		ss = append(ss, fmt.Sprintf("''%s''", tableName))
	}

	rows, err := c.Db.Query(fmt.Sprintf(sql, dataNode, schemaName, strings.Join(ss, ",")))
	if err != nil {
		return nil, err
	}

	var tableSizeList []*TableSize
	for _, row := range rows {
		size, err := strconv.Atoi(row["pg_relation_size"].String)
		if err != nil {
			return nil, fmt.Errorf("got unexpected size, error: %v", err)
		}

		tableSizeList = append(tableSizeList, &TableSize{
			DataNode:   dataNode,
			SchemaName: row["nspname"].String,
			TableName:  row["relname"].String,
			Size:       size,
		})
	}

	return tableSizeList, nil
}

func (c *Executor) GetTableSizeGBBySingleNode(schemaName string, tableNameList []string) ([]*TableSize, error) {
	sql := `SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
FROM pg_class c JOIN pg_namespace nsp ON c.relnamespace = nsp.oid WHERE nsp.nspname = '%s' AND c.relname IN (%s)`

	var ss []string
	for _, tableName := range tableNameList {
		ss = append(ss, fmt.Sprintf("'%s'", tableName))
	}

	rows, err := c.Db.Query(fmt.Sprintf(sql, schemaName, strings.Join(ss, ",")))
	if err != nil {
		return nil, err
	}

	var tableSizeList []*TableSize
	for _, row := range rows {
		size, err := strconv.Atoi(row["pg_relation_size"].String)
		if err != nil {
			return nil, fmt.Errorf("got unexpected size, error: %v", err)
		}

		tableSizeList = append(tableSizeList, &TableSize{
			DataNode:   "-",
			SchemaName: row["nspname"].String,
			TableName:  row["relname"].String,
			Size:       size,
		})
	}

	return tableSizeList, nil
}
