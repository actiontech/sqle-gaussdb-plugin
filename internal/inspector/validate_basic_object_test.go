package inspector

import (
	"log"
	"os"
	"testing"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

type testValidateBasicObject struct {
	Name         string
	SQL          string
	ExpectResult string
}

func TestValidateCreateSchema(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "create schema",
			SQL:          "create schema postgres",
			ExpectResult: "",
		},
		{
			Name:         "create schema",
			SQL:          "create schema test",
			ExpectResult: "schema exists",
		},
	}

	// mock query: select schema_name from information_schema.schemata;
	// 创建预期的查询结果 columns 和 rows
	columns := []string{"datname"}
	rows := sqlmock.NewRows(columns).AddRow("test")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = 'test';").WillReturnRows(rows)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "schema exists"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateDropSchema(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "drop schema",
			SQL:          "drop schema postgres",
			ExpectResult: "schema not exists",
		},
		{
			Name:         "drop schema",
			SQL:          "drop schema test",
			ExpectResult: "",
		},
	}

	// mock query: select schema_name from information_schema.schemata;
	// 创建预期的查询结果 columns 和 rows
	columns := []string{"schema_name"}
	rows := sqlmock.NewRows(columns).AddRow("test")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = 'test';").WillReturnRows(rows)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "schema not exists"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateCreateTable(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "create table",
			SQL:          "create table test(id int,name varchar(100))",
			ExpectResult: "",
		},
		{
			Name:         "create table",
			SQL:          "create table person(id int,name varchar(100),age int)",
			ExpectResult: "table exists",
		},
	}

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("person1")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo := &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := [3]*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
			{
				ColumnName:   "name",
				OwnerName:    "test",
				TableName:    "person",
				ColumnType:   "varchar",
				ColumnLength: 100,
			},
			{
				ColumnName: "age",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
		}
		tableInfo.ColumnInfoList = columnInfoList[:]
		tableInfoList = append(tableInfoList, tableInfo)
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "table exists"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateDropTable(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "drop table",
			SQL:          "drop table test",
			ExpectResult: "table not exists",
		},
		{
			Name:         "drop table",
			SQL:          "drop table person",
			ExpectResult: "",
		},
	}

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("person1")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo := &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := [3]*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
			{
				ColumnName:   "name",
				OwnerName:    "test",
				TableName:    "person",
				ColumnType:   "varchar",
				ColumnLength: 100,
			},
			{
				ColumnName: "age",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
		}
		tableInfo.ColumnInfoList = columnInfoList[:]
		tableInfoList = append(tableInfoList, tableInfo)
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "table not exists"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateAlterTable(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "alter table add column",
			SQL:          `alter table person add column newColumn int`,
			ExpectResult: "",
		},
		{
			Name:         "alter table add column",
			SQL:          `alter table person add column age int`,
			ExpectResult: "failed",
		},
		{
			Name:         "alter table drop column",
			SQL:          `alter table person drop column name`,
			ExpectResult: "",
		},
		{
			Name:         "alter table drop column",
			SQL:          `alter table person drop column name1`,
			ExpectResult: "failed",
		},
		{
			Name:         "alter table alter column type",
			SQL:          `alter table person ALTER COLUMN name type varchar(200) using name::varchar(100)`,
			ExpectResult: "",
		},
		{
			Name:         "alter table alter column type",
			SQL:          `alter table person ALTER COLUMN name1 type varchar(200) using name1::varchar(100)`,
			ExpectResult: "failed",
		},
		{
			Name:         "alter table add CONSTRAINT",
			SQL:          `alter table person ADD CONSTRAINT uni_person unique (name)`,
			ExpectResult: "",
		},
		{
			Name:         "alter table add CONSTRAINT",
			SQL:          `alter table person1 ADD CONSTRAINT uni_person unique (name)`,
			ExpectResult: "failed",
		},
		{
			Name:         "alter table add primary key",
			SQL:          `alter table person ADD CONSTRAINT pk_person PRIMARY KEY (id)`,
			ExpectResult: "",
		},
		{
			Name:         "alter table add primary key",
			SQL:          `alter table person1 ADD CONSTRAINT pk_person PRIMARY KEY (id)`,
			ExpectResult: "failed",
		},
	}

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("person")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo := &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := [3]*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
			{
				ColumnName:   "name",
				OwnerName:    "test",
				TableName:    "person",
				ColumnType:   "varchar",
				ColumnLength: 100,
			},
			{
				ColumnName: "age",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
		}
		tableInfo.ColumnInfoList = columnInfoList[:]
		tableInfoList = append(tableInfoList, tableInfo)
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "failed"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateRename(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "alter table rename index name",
			SQL:          `ALTER INDEX idx_name RENAME TO new_idx_name;`,
			ExpectResult: "",
		},
		{
			Name:         "alter table rename index name",
			SQL:          `ALTER INDEX idx_name1 RENAME TO new_idx_name;`,
			ExpectResult: "failed",
		},
		{
			Name:         "alter table rename table name",
			SQL:          `ALTER TABLE person RENAME TO person_ok;`,
			ExpectResult: "",
		},
		{
			Name:         "alter table rename table name",
			SQL:          `ALTER TABLE person1 RENAME TO person_ok;`,
			ExpectResult: "failed",
		},
		{
			Name:         "alter table rename column name",
			SQL:          `ALTER TABLE person RENAME COLUMN id TO id1;`,
			ExpectResult: "",
		},
		{
			Name:         "alter table rename column name",
			SQL:          `ALTER TABLE person RENAME COLUMN idx TO id1;`,
			ExpectResult: "failed",
		},
	}

	// 模拟查询数据库是否存在索引
	rows := sqlmock.NewRows([]string{"indexdef"}).AddRow("create index idx_name on test(name)")
	// 预期查询，指定预期的参数和结果
	mock.ExpectQuery("SELECT indexdef FROM pg_indexes WHERE schemaname = $1 AND indexname = $2;").
		WithArgs("test", "new_idx_name").WillReturnRows(rows)
	// 创建预期的查询结果 columns 和 rows
	columnsIndex := []string{"indexname", "indexdef"}
	rowsIndex := sqlmock.NewRows(columnsIndex).AddRow("idx_name", "create index idx_name on test(name)")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").
		WillReturnRows(rowsIndex)
	rows = sqlmock.NewRows([]string{"indexdef"})
	mock.ExpectQuery("SELECT indexdef FROM pg_indexes WHERE schemaname = $1 AND indexname = $2;").
		WithArgs("test", "idx_name1").WillReturnRows(rows)

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("person1")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo := &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := [3]*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
			{
				ColumnName:   "name",
				OwnerName:    "test",
				TableName:    "person",
				ColumnType:   "varchar",
				ColumnLength: 100,
			},
			{
				ColumnName: "age",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
		}
		tableInfo.ColumnInfoList = columnInfoList[:]
		tableInfoList = append(tableInfoList, tableInfo)
		indexInfoList := make([]*IndexInfo, 0)
		indexInfoList = append(indexInfoList, &IndexInfo{
			IndexName:  "idx_name",
			OwnerName:  "test",
			TableName:  "person",
			ColumnList: []string{"name"},
		})
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
			IndexInfoList: indexInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "failed"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateDml(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "select schema and table",
			SQL:          `select id,name,age from test.person where id = 1`,
			ExpectResult: "",
		},
		{
			Name:         "select schema",
			SQL:          `select id,name,age from xxx.person where id = 1`,
			ExpectResult: "failed",
		},
		{
			Name:         "select table",
			SQL:          `select id,name,age from person1 where id = 1`,
			ExpectResult: "failed",
		},
		{
			Name:         "insert schema and table",
			SQL:          `insert into test.person(id,name,age) values(1,'test',100)`,
			ExpectResult: "",
		},
		{
			Name:         "insert schema",
			SQL:          `insert into xxx.person(id,name,age) values(1,'test',100)`,
			ExpectResult: "failed",
		},
		{
			Name:         "insert table",
			SQL:          `insert into test.person1(id,name,age) values(1,'test',100)`,
			ExpectResult: "failed",
		},
		{
			Name:         "update schema and table",
			SQL:          `update test.person set name='' where id > 0`,
			ExpectResult: "",
		},
		{
			Name:         "update schema",
			SQL:          `update xxx.person set name='' where id > 0`,
			ExpectResult: "failed",
		},
		{
			Name:         "update table",
			SQL:          `update test.person1 set name='' where id > 0`,
			ExpectResult: "failed",
		},
		{
			Name:         "delete schema and table",
			SQL:          `delete from test.person where id = 1`,
			ExpectResult: "",
		},
		{
			Name:         "delete schema",
			SQL:          `delete from xxx.person where id = 1`,
			ExpectResult: "failed",
		},
		{
			Name:         "delete table",
			SQL:          `delete from test.person1 where id = 1`,
			ExpectResult: "failed",
		},
		{
			Name:         "insert columns",
			SQL:          `insert into person(id,name,age) values(1,'test',100)`,
			ExpectResult: "",
		},
		{
			Name:         "insert columns",
			SQL:          `insert into person(id1,name,age) values(1,'test',100)`,
			ExpectResult: "failed",
		},
		{
			Name:         "update columns",
			SQL:          `update person set name='' where id > 0`,
			ExpectResult: "",
		},
		{
			Name:         "update columns",
			SQL:          `update person set name1='' where id > 0`,
			ExpectResult: "failed",
		},
		{
			Name:         "update columns",
			SQL:          `update test.person set name='' where name = 'test'`,
			ExpectResult: "",
		},
		{
			Name:         "update columns",
			SQL:          `update person set name='' where name1 = 'test'`,
			ExpectResult: "failed",
		},
		{
			Name:         "delete columns",
			SQL:          `delete from person where id = 1`,
			ExpectResult: "",
		},
		{
			Name:         "delete columns",
			SQL:          `delete from person where id1 = 1`,
			ExpectResult: "failed",
		},
	}

	// mock query: select schema_name from information_schema.schemata;
	// 创建预期的查询结果 columns 和 rows
	columns := []string{"schema_name"}
	rows := sqlmock.NewRows(columns).AddRow("test")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("person")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)

	expectedQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult = sqlmock.NewRows([]string{"table_name"}).AddRow("person")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)

	expectedQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult = sqlmock.NewRows([]string{"table_name"}).AddRow("person")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)

	expectedQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult = sqlmock.NewRows([]string{"table_name"}).AddRow("person")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale from information_schema.columns where table_schema = $1 and table_name = 'person1'"
	expectedResult = sqlmock.NewRows([]string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"}).
		AddRow("id", "integer", "", "YES", "", "32", "0").
		AddRow("name", "character varying", "", "YES", "", "", "").
		AddRow("age", "integer", "", "YES", "", "32", "0")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	// 创建预期的查询结果 columns 和 rows
	columnsIndex := []string{"indexname", "indexdef"}
	rowsIndex := sqlmock.NewRows(columnsIndex)
	// 预期执行 SELECT 语句
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = 'test' and tablename = 'person1'").
		WillReturnRows(rowsIndex)

	expectedQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'xxx' AND table_type = 'BASE TABLE';"
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo := &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := [3]*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
			{
				ColumnName:   "name",
				OwnerName:    "test",
				TableName:    "person",
				ColumnType:   "varchar",
				ColumnLength: 100,
			},
			{
				ColumnName: "age",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
		}
		tableInfo.ColumnInfoList = columnInfoList[:]
		tableInfoList = append(tableInfoList, tableInfo)
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "failed"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateWith(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "with单表校验--通过",
			SQL:          `WITH cte AS (SELECT * FROM exist_db.exist_tb_1 WHERE id IN (501)) SELECT * FROM cte;`,
			ExpectResult: "",
		},
		{
			Name:         "with单表校验--不通过 引用的cte1表不存在",
			SQL:          `WITH cte AS (SELECT * FROM exist_db.exist_tb_1 WHERE id IN (501)) SELECT * FROM cte1;`,
			ExpectResult: "failed",
		},
		{
			Name:         "with单表校验--不通过 引用的源表exist_tb_11不存在",
			SQL:          `WITH cte AS (SELECT * FROM exist_db.exist_tb_11 WHERE id IN (501)) SELECT * FROM cte1;`,
			ExpectResult: "failed",
		},
		{
			Name:         "with多表校验--通过",
			SQL:          `WITH cte AS (SELECT * FROM exist_db.exist_tb_1 WHERE id IN (501)),cte2 AS (SELECT * FROM exist_db.exist_tb_2) SELECT * FROM cte JOIN cte2 ON cte.id = cte2.id;`,
			ExpectResult: "",
		},
		{
			Name:         "with多表校验--不通过 引用的cte1表不存在",
			SQL:          `WITH cte AS (SELECT * FROM exist_db.exist_tb_1 WHERE id IN (501)),cte2 AS (SELECT * FROM exist_db.exist_tb_2) SELECT * FROM cte1 JOIN cte2 ON cte1.id = cte2.id;`,
			ExpectResult: "failed",
		},
		{
			Name:         "with多表校验--不通过 引用的源表exist_tb_11不存在",
			SQL:          `WITH cte AS (SELECT * FROM exist_db.exist_tb_11 WHERE id IN (501)),cte2 AS (SELECT * FROM exist_db.exist_tb_2) SELECT * FROM cte JOIN cte2 ON cte.id = cte2.id;`,
			ExpectResult: "failed",
		},
	}

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("exist_tb_1").AddRow("exist_tb_2")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "select table_name, column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1"
	expectedResult = sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"}).
		AddRow("exist_tb_1", "id", "integer", "", "YES", "", "32", "0", "10").
		AddRow("exist_tb_2", "id", "integer", "", "YES", "", "32", "0", "10")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "SELECT t.relname AS table_name, c.conname AS constraint_name, c.contype AS constraint_type, CASE WHEN c.contype IN ('p', 'u') THEN string_agg(a.attname, ',') WHEN c.contype = 'f' THEN string_agg(a.attname, ',') ELSE null END as column_names, CASE WHEN c.contype = 'f' THEN c.confrelid::regclass ELSE null END as referenced_table, CASE WHEN c.contype = 'f' THEN string_agg(ac.attname, ',') ELSE null END as referenced_columns, CASE WHEN c.contype = 'c' THEN pg_get_constraintdef(c.oid) ELSE null END as check_condition FROM pg_constraint c JOIN pg_class t ON c.conrelid = t.oid JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey) JOIN pg_namespace n ON t.relnamespace = n.oid LEFT JOIN pg_class rc ON c.confrelid = rc.oid LEFT JOIN pg_attribute ac ON ac.attrelid = rc.oid AND ac.attnum = ANY (c.confkey) WHERE n.nspname = $1 GROUP BY t.relname, c.conname, c.contype, c.confrelid, c.oid HAVING c.contype IN('p', 'u', 'f', 'c')"
	expectedResult = sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"}).
		AddRow("exist_tb_1", "id", "integer", "", "YES", "", "32", "0").
		AddRow("exist_tb_2", "id", "integer", "", "YES", "", "32", "0")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "SELECT tablename,indexname,indexdef FROM pg_indexes where schemaname = $1"
	expectedResult = sqlmock.NewRows([]string{"tablename", "indexname", "indexdef"}).
		AddRow("exist_tb_1", "", "")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	// Define your expected SQL query and result
	expectedQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult = sqlmock.NewRows([]string{"table_name"}).AddRow("exist_tb_1").AddRow("exist_tb_2")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "exist_db", nil, db)
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := []*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "exist_db_1",
				ColumnType: "int4",
			},
		}
		tableInfoList = append(tableInfoList, &TableInfo{
			TableName:      "exist_tb_1",
			OwnerName:      "exist_db",
			ColumnInfoList: columnInfoList,
		})
		tableInfoList = append(tableInfoList, &TableInfo{
			TableName:      "exist_tb_2",
			OwnerName:      "exist_db",
			ColumnInfoList: columnInfoList,
		})
		i.pgContext.DatabaseInfo.SchemaInfoMap["exist_db"] = &SchemaInfo{
			SchemaName:    "exist_db",
			TableInfoList: tableInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "failed"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateUpdateWithFromClause(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "update校验--通过",
			SQL:          `UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' FROM exist_db.exist_tb_2 WHERE exist_tb_1.id_shd_key = exist_tb_2.id;`,
			ExpectResult: "",
		},
		{
			Name:         "update校验--不通过 更新表exist_tb_1中列id_shd_key1不存在",
			SQL:          `UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' FROM exist_db.exist_tb_2 WHERE exist_tb_1.id_shd_key1 = exist_tb_2.id;`,
			ExpectResult: "failed",
		},
	}

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("exist_tb_1").AddRow("exist_tb_2")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "select table_name, column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1"
	expectedResult = sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"}).
		AddRow("exist_tb_1", "id", "integer", "", "YES", "", "32", "0", "10").
		AddRow("exist_tb_1", "id_shd_key", "integer", "", "YES", "", "100", "0", "0").
		AddRow("exist_tb_1", "c1_with_idx", "varchar(100)", "", "YES", "", "100", "0", "0").
		AddRow("exist_tb_2", "id", "integer", "", "YES", "", "32", "0", "10")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "SELECT t.relname AS table_name, c.conname AS constraint_name, c.contype AS constraint_type, CASE WHEN c.contype IN ('p', 'u') THEN string_agg(a.attname, ',') WHEN c.contype = 'f' THEN string_agg(a.attname, ',') ELSE null END as column_names, CASE WHEN c.contype = 'f' THEN c.confrelid::regclass ELSE null END as referenced_table, CASE WHEN c.contype = 'f' THEN string_agg(ac.attname, ',') ELSE null END as referenced_columns, CASE WHEN c.contype = 'c' THEN pg_get_constraintdef(c.oid) ELSE null END as check_condition FROM pg_constraint c JOIN pg_class t ON c.conrelid = t.oid JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey) JOIN pg_namespace n ON t.relnamespace = n.oid LEFT JOIN pg_class rc ON c.confrelid = rc.oid LEFT JOIN pg_attribute ac ON ac.attrelid = rc.oid AND ac.attnum = ANY (c.confkey) WHERE n.nspname = $1 GROUP BY t.relname, c.conname, c.contype, c.confrelid, c.oid HAVING c.contype IN('p', 'u', 'f', 'c')"
	expectedResult = sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"}).
		AddRow("exist_tb_1", "id", "integer", "", "YES", "", "32", "0").
		AddRow("exist_tb_1", "id_shd_key", "integer", "", "YES", "", "32", "0").
		AddRow("exist_tb_1", "c1_with_idx", "varchar(100)", "", "YES", "", "100", "0").
		AddRow("exist_tb_2", "id", "integer", "", "YES", "", "32", "0")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "SELECT tablename,indexname,indexdef FROM pg_indexes where schemaname = $1"
	expectedResult = sqlmock.NewRows([]string{"tablename", "indexname", "indexdef"}).
		AddRow("exist_tb_1", "", "")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	// Define your expected SQL query and result
	expectedQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult = sqlmock.NewRows([]string{"table_name"}).AddRow("exist_tb_1").AddRow("exist_tb_2")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "exist_db", nil, db)
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := []*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "exist_db_1",
				ColumnType: "int4",
			},
			{
				ColumnName: "id_shd_key",
				OwnerName:  "test",
				TableName:  "exist_db_1",
				ColumnType: "int4",
			},
			{
				ColumnName: "c1_with_idx",
				OwnerName:  "test",
				TableName:  "exist_db_1",
				ColumnType: "varchar(100)",
			},
		}
		tableInfoList = append(tableInfoList, &TableInfo{
			TableName:      "exist_tb_1",
			OwnerName:      "exist_db",
			ColumnInfoList: columnInfoList,
		})
		tableInfoList = append(tableInfoList, &TableInfo{
			TableName:      "exist_tb_2",
			OwnerName:      "exist_db",
			ColumnInfoList: columnInfoList,
		})
		i.pgContext.DatabaseInfo.SchemaInfoMap["exist_db"] = &SchemaInfo{
			SchemaName:    "exist_db",
			TableInfoList: tableInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "failed"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateCreateIndex(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "create index",
			SQL:          "create index idx_name on person(name)",
			ExpectResult: "index exists",
		},
		{
			Name:         "create index",
			SQL:          "create index idx_name_age on person(name,age)",
			ExpectResult: "",
		},
	}

	// 创建预期的查询结果 columns 和 rows
	columnsIndex := []string{"indexname", "indexdef"}
	rowsIndex := sqlmock.NewRows(columnsIndex).AddRow("index1", "CREATE INDEX idx_name ON test.test1 USING btree (name)")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").
		WillReturnRows(rowsIndex)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo := &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := [3]*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
			{
				ColumnName:   "name",
				OwnerName:    "test",
				TableName:    "person",
				ColumnType:   "varchar",
				ColumnLength: 100,
			},
			{
				ColumnName: "age",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
		}
		tableInfo.ColumnInfoList = columnInfoList[:]
		tableInfoList = append(tableInfoList, tableInfo)
		indexInfoList := make([]*IndexInfo, 0)
		indexInfoList = append(indexInfoList, &IndexInfo{
			IndexName:  "idx_name",
			OwnerName:  "test",
			TableName:  "person",
			ColumnList: []string{"name"},
		})
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
			IndexInfoList: indexInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "index exists"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}

func TestValidateDropIndex(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	testCases := []testValidateBasicObject{
		{
			Name:         "drop index",
			SQL:          "drop index idx_name",
			ExpectResult: "",
		},
		{
			Name:         "drop index",
			SQL:          "drop index idx_name1",
			ExpectResult: "index not exists",
		},
	}

	// 模拟查询数据库是否存在索引
	rows := sqlmock.NewRows([]string{"indexdef"})
	// 预期查询，指定预期的参数和结果
	mock.ExpectQuery("SELECT indexdef FROM pg_indexes WHERE schemaname = $1 AND indexname = $2;").
		WithArgs("test", "idx_name1").WillReturnRows(rows)

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo := &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList := make([]*TableInfo, 0)
		columnInfoList := [3]*ColumnInfo{
			{
				ColumnName: "id",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
			{
				ColumnName:   "name",
				OwnerName:    "test",
				TableName:    "person",
				ColumnType:   "varchar",
				ColumnLength: 100,
			},
			{
				ColumnName: "age",
				OwnerName:  "test",
				TableName:  "person",
				ColumnType: "int4",
			},
		}
		tableInfo.ColumnInfoList = columnInfoList[:]
		tableInfoList = append(tableInfoList, tableInfo)
		indexInfoList := make([]*IndexInfo, 0)
		indexInfoList = append(indexInfoList, &IndexInfo{
			IndexName:  "idx_name",
			OwnerName:  "test",
			TableName:  "person",
			ColumnList: []string{"name"},
		})
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
			IndexInfoList: indexInfoList,
		}
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		validateBasicObject := ValidateBasicObject{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		validateResult, validateErr := validateBasicObject.Validate()
		actualResult := ""
		if validateErr != nil || (validateResult != nil && len(validateResult.Level) > 0) {
			actualResult = "index not exists"
		}
		assert.EqualValues(t, testCase.ExpectResult, actualResult)
	}
}
