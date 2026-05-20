package inspector

import (
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/hashicorp/go-hclog"
	parser "github.com/pganalyze/pg_query_go/v2"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

type testPgContext struct {
	Name             string
	SQL              string
	ExpectSchema     string
	ExpectSchemaInfo SchemaInfo
}

func TestCreateSchema(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	i := NewMockDriver(nil, "test", "test", nil, db)
	testCase := testPgContext{
		Name:         "create schema",
		SQL:          `create schema postgres`,
		ExpectSchema: "postgres",
		ExpectSchemaInfo: SchemaInfo{
			SchemaName:    "postgres",
			TableInfoList: make([]*TableInfo, 0),
			IndexInfoList: make([]*IndexInfo, 0),
		},
	}

	// mock query: select schema_name from information_schema.schemata;
	// 创建预期的查询结果 columns 和 rows
	columns := []string{"schema_name"}
	rows := sqlmock.NewRows(columns).AddRow("test")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1;").WillReturnRows(rows)

	runTest(t, testCase, i)
}

func TestDropSchema(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	i := NewMockDriver(nil, "test", "test", nil, db)
	testCase := testPgContext{
		Name:             "drop schema",
		SQL:              `drop schema postgres`,
		ExpectSchema:     "postgres",
		ExpectSchemaInfo: SchemaInfo{},
	}

	// mock query: select schema_name from information_schema.schemata;
	// 创建预期的查询结果 columns 和 rows
	columns := []string{"schema_name"}
	rows := sqlmock.NewRows(columns).AddRow("test")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;").WillReturnRows(rows)
	mock.ExpectQuery("select schema_name from information_schema.schemata where catalog_name = $1;").WillReturnRows(rows)

	runTest(t, testCase, i)
}

func TestCreateTable(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	i := NewMockDriver(nil, "", "test", nil, db)
	columnInfoList := make([]*ColumnInfo, 0)
	tableInfoList := make([]*TableInfo, 0)
	indexInfoList := make([]*IndexInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName:      "id",
		OwnerName:       "test",
		TableName:       "person",
		ColumnType:      "int4",
		IsNullable:      false,
		IsPrimaryColumn: true,
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName:   "name",
		OwnerName:    "test",
		TableName:    "person",
		ColumnType:   "varchar",
		ColumnLength: 100,
		IsUnique:     true,
		IsNullable:   true,
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "age",
		OwnerName:  "test",
		TableName:  "person",
		ColumnType: "int4",
		IsNullable: true,
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "person",
		OwnerName:      "test",
		ColumnInfoList: columnInfoList,
	})
	indexInfoList = append(indexInfoList, &IndexInfo{
		IndexName:  "person_pkey",
		TableName:  "person",
		OwnerName:  "test",
		IsUnique:   true,
		ColumnList: []string{"id"},
	})
	testCase := testPgContext{
		Name:             "create table",
		SQL:              `create table person(id int not null primary key ,name varchar(100) unique ,age int)`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoList, IndexInfoList: indexInfoList},
	}

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).AddRow("person1")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	// 创建预期的查询结果 columns 和 rows
	columnsIndex := []string{"indexname", "indexdef"}
	rowsIndex := sqlmock.NewRows(columnsIndex)
	// 预期执行 SELECT 语句
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").
		WillReturnRows(rowsIndex)

	runTest(t, testCase, i)
}

func TestDropTable(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	i := NewMockDriver(nil, "test", "test", nil, db)
	testCase := testPgContext{
		Name:             "drop table",
		SQL:              `drop table person`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test"},
	}

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).
		AddRow("person")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = "select table_name, column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1"
	expectedResult = sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"}).
		AddRow("person", "id", "integer", "", "YES", "", "32", "0").
		AddRow("person", "name", "character varying", "", "YES", "", "", "").
		AddRow("person", "age", "integer", "", "YES", "", "32", "0")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	expectedQuery = `SELECT t.relname AS table_name, c.conname AS constraint_name, c.contype AS constraint_type, CASE WHEN c.contype IN ('p', 'u') THEN string_agg(a.attname, ',') WHEN c.contype = 'f' THEN string_agg(a.attname, ',') ELSE null END as column_names, CASE WHEN c.contype = 'f' THEN c.confrelid::regclass ELSE null END as referenced_table, CASE WHEN c.contype = 'f' THEN string_agg(ac.attname, ',') ELSE null END as referenced_columns, CASE WHEN c.contype = 'c' THEN pg_get_constraintdef(c.oid) ELSE null END as check_condition FROM pg_constraint c JOIN pg_class t ON c.conrelid = t.oid JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey) JOIN pg_namespace n ON t.relnamespace = n.oid LEFT JOIN pg_class rc ON c.confrelid = rc.oid LEFT JOIN pg_attribute ac ON ac.attrelid = rc.oid AND ac.attnum = ANY (c.confkey) WHERE n.nspname = $1 GROUP BY t.relname, c.conname, c.contype, c.confrelid, c.oid HAVING c.contype IN('p', 'u', 'f', 'c')`
	expectedResult = sqlmock.NewRows([]string{"table_name", "constraint_name", "constraint_type", "column_names", "referenced_table", "referenced_columns", "check_condition"}).
		AddRow("test", "test", "P", "", "", "", "")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	// 创建预期的查询结果 columns 和 rows
	columnsIndex := []string{"tablename", "indexdef"}
	rowsIndex := sqlmock.NewRows(columnsIndex).AddRow("index1", "CREATE INDEX idx_name ON test.test1 USING btree (name)")
	// 预期执行 SELECT 语句
	mock.ExpectQuery("SELECT tablename,indexname,indexdef FROM pg_indexes where schemaname = $1").
		WillReturnRows(rowsIndex)

	runTest(t, testCase, i)
}

func TestAlterTable(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	indexInfoLists := [8][]*IndexInfo{}
	tableInfoLists := [8][]*TableInfo{}
	tableInfoList := make([]*TableInfo, 0)
	columnInfoListTemp := make([]*ColumnInfo, 0)
	columnInfoList := [4]*ColumnInfo{
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
		{
			ColumnName: "newcolumn",
			OwnerName:  "test",
			TableName:  "person",
			ColumnType: "int4",
			IsNullable: true,
		},
	}

	// add column
	tableInfo := &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	tableInfo.ColumnInfoList = columnInfoList[:]
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[0] = tableInfoList

	// drop column
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	columnInfoListTemp = append(columnInfoListTemp[:1], columnInfoListTemp[2:3]...)
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[1] = tableInfoList

	// alter column type
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	columnInfoListTemp[1].ColumnLength = 200
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[2] = tableInfoList

	// add primary constraint
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	constraintInfoList := make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "pk_person",
		ConstraintType:    ConstraintTypeP,
		ColumnList:        []string{"id"},
		ConstraintContent: "CONSTRAINT pk_person PRIMARY KEY (id)",
	})
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[3] = tableInfoList
	indexInfoList := make([]*IndexInfo, 0)
	indexInfo := &IndexInfo{
		IndexName:  "person_pkey",
		TableName:  "person",
		OwnerName:  "test",
		IsUnique:   true,
		ColumnList: []string{"id"},
	}
	indexInfoList = append(indexInfoList, indexInfo)
	indexInfoLists[3] = indexInfoList

	// add unique constraint
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	constraintInfoList = make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "uni_person",
		ConstraintType:    ConstraintTypeU,
		ColumnList:        []string{"name"},
		ConstraintContent: "CONSTRAINT uni_person UNIQUE (name)",
	})
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[4] = tableInfoList
	indexInfoList = make([]*IndexInfo, 0)
	indexInfo = &IndexInfo{
		IndexName:  "person_name_key",
		TableName:  "person",
		OwnerName:  "test",
		IsUnique:   true,
		ColumnList: []string{"name"},
	}
	indexInfoList = append(indexInfoList, indexInfo)
	indexInfoLists[4] = indexInfoList

	// drop constraint
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[5] = tableInfoList

	// set column not null
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ColumnInfoList[1].IsNullable = false
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[6] = tableInfoList

	// drop column not null
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ColumnInfoList[1].IsNullable = true
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[7] = tableInfoList

	testCases := []testPgContext{
		{
			Name:             "alter table add column",
			SQL:              `alter table person add column newColumn int`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[0]},
		},
		{
			Name:             "alter table drop column",
			SQL:              `alter table person drop column name`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[1]},
		},
		{
			Name:             "alter table alter column type",
			SQL:              `alter table person ALTER COLUMN name type varchar(200) using name::varchar(100)`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[2]},
		},
		{
			Name:             "alter table add primary Constraint",
			SQL:              `alter table person ADD CONSTRAINT pk_person PRIMARY KEY (id)`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[3], IndexInfoList: indexInfoLists[3]},
		},
		{
			Name:             "alter table add unique Constraint",
			SQL:              `alter table person ADD CONSTRAINT uni_person unique (name)`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[4], IndexInfoList: indexInfoLists[4]},
		},
		{
			Name:             "alter table drop constraint",
			SQL:              `alter table person DROP CONSTRAINT pk_test1`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[5]},
		},
		{
			Name:             "alter table set column not null",
			SQL:              `alter table person ALTER COLUMN name SET NOT NULL`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[6]},
		},
		{
			Name:             "alter table drop column not null",
			SQL:              `alter table person ALTER COLUMN name DROP NOT NULL;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[7]},
		},
	}

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo = &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList = make([]*TableInfo, 0)
		columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
		tableInfo.ColumnInfoList = columnInfoListTemp
		tableInfoList = append(tableInfoList, tableInfo)
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
			IndexInfoList: testCase.ExpectSchemaInfo.IndexInfoList,
		}
		runTest(t, testCase, i)
	}
}

func TestRename(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	tableInfoLists := [4][]*TableInfo{}
	tableInfoList := make([]*TableInfo, 0)
	columnInfoListTemp := make([]*ColumnInfo, 0)
	constraintInfoList := make([]*ConstraintInfo, 0)
	columnInfoList := [4]*ColumnInfo{
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
		{
			ColumnName: "newcolumn",
			OwnerName:  "test",
			TableName:  "person",
			ColumnType: "int4",
			IsNullable: true,
		},
	}

	// 当前schema下索引列表
	indexInfoList := make([]*IndexInfo, 0)
	indexInfoList = append(indexInfoList, &IndexInfo{
		IndexName:  "idx_name",
		OwnerName:  "test",
		TableName:  "person",
		ColumnList: []string{"name"},
	})

	// 主键约束
	constraintInfoList = make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "pk_person",
		ConstraintType:    ConstraintTypeP,
		ColumnList:        []string{"id"},
		ConstraintContent: "CONSTRAINT pk_person PRIMARY KEY (id)",
	})

	// rename index name
	tableInfo := &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[0] = tableInfoList
	indexInfoListModify := make([]*IndexInfo, 0)
	indexInfoListModify = append(indexInfoListModify, &IndexInfo{
		IndexName:  "new_idx_name",
		OwnerName:  "test",
		TableName:  "person",
		ColumnList: []string{"name"},
	})

	// rename table name
	tableInfo = &TableInfo{
		TableName: "person_ok",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[1] = tableInfoList

	// rename column name
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	columnInfoListTemp[0].ColumnName = "id1"
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[2] = tableInfoList

	// rename constraint name
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	constraintInfoList = make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "pk_person_id",
		ConstraintType:    ConstraintTypeP,
		ColumnList:        []string{"id"},
		ConstraintContent: "CONSTRAINT pk_person_id PRIMARY KEY (id)",
	})
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[3] = tableInfoList

	// 模拟查询数据库是否存在索引
	rows := sqlmock.NewRows([]string{"indexdef"}).AddRow("create index new_idx_name on test(name)")
	// 预期查询，指定预期的参数和结果
	mock.ExpectQuery("SELECT indexdef FROM pg_indexes WHERE schemaname = $1 AND indexname = $2;").
		WithArgs("test", "new_idx_name").WillReturnRows(rows)

	// Define your expected SQL query and result
	expectedQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult := sqlmock.NewRows([]string{"table_name"}).
		AddRow("person")

	rows = sqlmock.NewRows([]string{"indexname", "indexdef"})
	// 预期查询，指定预期的参数和结果
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").
		WillReturnRows(rows)

	// Define your expected SQL query and result
	expectedQuery = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	expectedResult = sqlmock.NewRows([]string{"table_name"}).
		AddRow("person")
	// Set up the expectations for the query
	mock.ExpectQuery(expectedQuery).WillReturnRows(expectedResult)

	testCases := []testPgContext{
		{
			Name:             "alter table rename index name",
			SQL:              `ALTER INDEX idx_name RENAME TO new_idx_name;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[0], IndexInfoList: indexInfoListModify},
		},
		{
			Name:             "alter table rename table name",
			SQL:              `ALTER TABLE person RENAME TO person_ok;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[1], IndexInfoList: indexInfoList},
		},
		{
			Name:             "alter table rename column name",
			SQL:              `ALTER TABLE person RENAME COLUMN id TO id1;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[2], IndexInfoList: indexInfoList},
		},
		{
			Name:             "alter table rename constraint name",
			SQL:              `ALTER TABLE person RENAME CONSTRAINT pk_person TO pk_person_id;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[3], IndexInfoList: indexInfoList},
		},
	}

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, db)
		tableInfo = &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList = make([]*TableInfo, 0)
		columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
		tableInfo.ColumnInfoList = columnInfoListTemp
		tableInfoList = make([]*TableInfo, 0)
		constraintInfoList = make([]*ConstraintInfo, 0)
		constraintInfoList = append(constraintInfoList, &ConstraintInfo{
			ConstraintName:    "pk_person",
			ConstraintType:    ConstraintTypeP,
			ColumnList:        []string{"id"},
			ConstraintContent: "CONSTRAINT pk_person PRIMARY KEY (id)",
		})
		tableInfo.ConstraintList = constraintInfoList
		tableInfoList = append(tableInfoList, tableInfo)
		indexInfoList = make([]*IndexInfo, 0)
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
		runTest(t, testCase, i)
	}
}

func TestCreateIndex(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	i := NewMockDriver(nil, "test", "test", nil, db)
	indexInfoList := make([]*IndexInfo, 0)
	indexInfoList = append(indexInfoList, &IndexInfo{
		IndexName:  "idx_name",
		OwnerName:  "test",
		TableName:  "person",
		ColumnList: []string{"name"},
	})
	testCase := testPgContext{
		Name:             "create index",
		SQL:              `create index idx_name on person(name)`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test", IndexInfoList: indexInfoList},
	}

	// 创建预期的查询结果 columns 和 rows
	columnsIndex := []string{"indexname", "indexdef"}
	rowsIndex := sqlmock.NewRows(columnsIndex)
	// 预期执行 SELECT 语句
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").
		WillReturnRows(rowsIndex)
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").
		WillReturnRows(rowsIndex)

	runTest(t, testCase, i)
}

func TestDropIndex(t *testing.T) {
	db, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	defer mock.ExpectClose()
	if err != nil {
		log.Printf("获取db和mock失败:%s\n", err)
	}

	i := NewMockDriver(nil, "test", "test", nil, db)
	testCase := testPgContext{
		Name:             "drop index",
		SQL:              `drop index idx_name`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test"},
	}

	// 模拟查询数据库是否存在索引
	rows := sqlmock.NewRows([]string{"indexdef"}).AddRow("create index idx_name on test(name)")
	// 预期查询，指定预期的参数和结果
	mock.ExpectQuery("SELECT indexdef FROM pg_indexes WHERE schemaname = $1 AND indexname = $2;").
		WithArgs("test", "idx_name").WillReturnRows(rows)

	rows = sqlmock.NewRows([]string{"indexname", "indexdef"}).AddRow("idx_name", "create index idx_name on test(name)")
	// 预期查询，指定预期的参数和结果
	mock.ExpectQuery("SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2").
		WillReturnRows(rows)

	runTest(t, testCase, i)
}

// **************************************************************************************************************
// ********************************下面是离线上下文测试用例**********************************************************
// **************************************************************************************************************
func TestOfflineCreateSchema(t *testing.T) {
	i := NewMockDriver(nil, "test", "test", nil, nil)
	testCase := testPgContext{
		Name:         "create schema",
		SQL:          `create schema postgres`,
		ExpectSchema: "postgres",
		ExpectSchemaInfo: SchemaInfo{
			SchemaName:    "postgres",
			TableInfoList: make([]*TableInfo, 0),
			IndexInfoList: make([]*IndexInfo, 0),
		},
	}

	i.pgContext.UsingType = UsingTypeOffline
	runTest(t, testCase, i)
}

func TestOfflineDropSchema(t *testing.T) {
	i := NewMockDriver(nil, "test", "test", nil, nil)
	testCase := testPgContext{
		Name:             "drop schema",
		SQL:              `drop schema postgres`,
		ExpectSchema:     "postgres",
		ExpectSchemaInfo: SchemaInfo{},
	}

	i.pgContext.UsingType = UsingTypeOffline
	runTest(t, testCase, i)
}

func TestOfflineCreateTable(t *testing.T) {
	i := NewMockDriver(nil, "", "test", nil, nil)
	columnInfoList := make([]*ColumnInfo, 0)
	tableInfoList := make([]*TableInfo, 0)
	indexInfoList := make([]*IndexInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName:      "id",
		OwnerName:       "test",
		TableName:       "person",
		ColumnType:      "int4",
		IsNullable:      false,
		IsPrimaryColumn: true,
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName:   "name",
		OwnerName:    "test",
		TableName:    "person",
		ColumnType:   "varchar",
		ColumnLength: 100,
		IsUnique:     true,
		IsNullable:   true,
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "age",
		OwnerName:  "test",
		TableName:  "person",
		ColumnType: "int4",
		IsNullable: true,
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "person",
		OwnerName:      "test",
		ColumnInfoList: columnInfoList,
	})
	indexInfoList = append(indexInfoList, &IndexInfo{
		IndexName:  "person_pkey",
		TableName:  "person",
		OwnerName:  "test",
		IsUnique:   true,
		ColumnList: []string{"id"},
	})
	testCase := testPgContext{
		Name:             "create table",
		SQL:              `create table person(id int not null primary key ,name varchar(100) unique ,age int)`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoList, IndexInfoList: indexInfoList},
	}

	i.pgContext.UsingType = UsingTypeOffline
	runTest(t, testCase, i)
}

func TestOfflineDropTable(t *testing.T) {
	i := NewMockDriver(nil, "test", "test", nil, nil)
	testCase := testPgContext{
		Name:             "drop table",
		SQL:              `drop table person`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test"},
	}

	i.pgContext.UsingType = UsingTypeOffline
	runTest(t, testCase, i)
}

func TestOfflineAlterTable(t *testing.T) {
	indexInfoLists := [8][]*IndexInfo{}
	tableInfoLists := [8][]*TableInfo{}
	tableInfoList := make([]*TableInfo, 0)
	columnInfoListTemp := make([]*ColumnInfo, 0)
	columnInfoList := [4]*ColumnInfo{
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
		{
			ColumnName: "newcolumn",
			OwnerName:  "test",
			TableName:  "person",
			ColumnType: "int4",
			IsNullable: true,
		},
	}

	// add column
	tableInfo := &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	tableInfo.ColumnInfoList = columnInfoList[:]
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[0] = tableInfoList

	// drop column
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	columnInfoListTemp = append(columnInfoListTemp[:1], columnInfoListTemp[2:3]...)
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[1] = tableInfoList

	// alter column type
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	columnInfoListTemp[1].ColumnLength = 200
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[2] = tableInfoList

	// add primary constraint
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	constraintInfoList := make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "pk_person",
		ConstraintType:    ConstraintTypeP,
		ColumnList:        []string{"id"},
		ConstraintContent: "CONSTRAINT pk_person PRIMARY KEY (id)",
	})
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[3] = tableInfoList
	indexInfoList := make([]*IndexInfo, 0)
	indexInfo := &IndexInfo{
		IndexName:  "person_pkey",
		TableName:  "person",
		OwnerName:  "test",
		IsUnique:   true,
		ColumnList: []string{"id"},
	}
	indexInfoList = append(indexInfoList, indexInfo)
	indexInfoLists[3] = indexInfoList

	// add unique constraint
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	constraintInfoList = make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "uni_person",
		ConstraintType:    ConstraintTypeU,
		ColumnList:        []string{"name"},
		ConstraintContent: "CONSTRAINT uni_person UNIQUE (name)",
	})
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[4] = tableInfoList
	indexInfoList = make([]*IndexInfo, 0)
	indexInfo = &IndexInfo{
		IndexName:  "person_name_key",
		TableName:  "person",
		OwnerName:  "test",
		IsUnique:   true,
		ColumnList: []string{"name"},
	}
	indexInfoList = append(indexInfoList, indexInfo)
	indexInfoLists[4] = indexInfoList

	// drop constraint
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[5] = tableInfoList

	// set column not null
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ColumnInfoList[1].IsNullable = false
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[6] = tableInfoList

	// drop column not null
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ColumnInfoList[1].IsNullable = true
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[7] = tableInfoList

	testCases := []testPgContext{
		{
			Name:             "alter table add column",
			SQL:              `alter table person add column newColumn int`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[0]},
		},
		{
			Name:             "alter table drop column",
			SQL:              `alter table person drop column name`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[1]},
		},
		{
			Name:             "alter table alter column type",
			SQL:              `alter table person ALTER COLUMN name type varchar(200) using name::varchar(100)`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[2]},
		},
		{
			Name:             "alter table add primary Constraint",
			SQL:              `alter table person ADD CONSTRAINT pk_person PRIMARY KEY (id)`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[3], IndexInfoList: indexInfoLists[3]},
		},
		{
			Name:             "alter table add unique Constraint",
			SQL:              `alter table person ADD CONSTRAINT uni_person unique (name)`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[4], IndexInfoList: indexInfoLists[4]},
		},
		{
			Name:             "alter table drop constraint",
			SQL:              `alter table person DROP CONSTRAINT pk_test1`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[5]},
		},
		{
			Name:             "alter table set column not null",
			SQL:              `alter table person ALTER COLUMN name SET NOT NULL`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[6]},
		},
		{
			Name:             "alter table drop column not null",
			SQL:              `alter table person ALTER COLUMN name DROP NOT NULL;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[7]},
		},
	}

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, nil)
		tableInfo = &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList = make([]*TableInfo, 0)
		columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
		tableInfo.ColumnInfoList = columnInfoListTemp
		tableInfoList = append(tableInfoList, tableInfo)
		i.pgContext.DatabaseInfo.SchemaInfoMap["test"] = &SchemaInfo{
			SchemaName:    "test",
			TableInfoList: tableInfoList,
			IndexInfoList: testCase.ExpectSchemaInfo.IndexInfoList,
		}

		i.pgContext.UsingType = UsingTypeOffline
		runTest(t, testCase, i)
	}
}

func TestOfflineRename(t *testing.T) {
	tableInfoLists := [4][]*TableInfo{}
	tableInfoList := make([]*TableInfo, 0)
	columnInfoListTemp := make([]*ColumnInfo, 0)
	constraintInfoList := make([]*ConstraintInfo, 0)
	columnInfoList := [4]*ColumnInfo{
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
		{
			ColumnName: "newcolumn",
			OwnerName:  "test",
			TableName:  "person",
			ColumnType: "int4",
			IsNullable: true,
		},
	}

	// 当前schema下索引列表
	indexInfoList := make([]*IndexInfo, 0)
	indexInfoList = append(indexInfoList, &IndexInfo{
		IndexName:  "idx_name",
		OwnerName:  "test",
		TableName:  "person",
		ColumnList: []string{"name"},
	})

	// 主键约束
	constraintInfoList = make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "pk_person",
		ConstraintType:    ConstraintTypeP,
		ColumnList:        []string{"id"},
		ConstraintContent: "CONSTRAINT pk_person PRIMARY KEY (id)",
	})

	// rename index name
	tableInfo := &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[0] = tableInfoList
	indexInfoListModify := make([]*IndexInfo, 0)
	indexInfoListModify = append(indexInfoListModify, &IndexInfo{
		IndexName:  "new_idx_name",
		OwnerName:  "test",
		TableName:  "person",
		ColumnList: []string{"name"},
	})

	// rename table name
	tableInfo = &TableInfo{
		TableName: "person_ok",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[1] = tableInfoList

	// rename column name
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	columnInfoListTemp[0].ColumnName = "id1"
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[2] = tableInfoList

	// rename constraint name
	tableInfo = &TableInfo{
		TableName: "person",
		OwnerName: "test",
	}
	columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
	tableInfo.ColumnInfoList = columnInfoListTemp
	tableInfoList = make([]*TableInfo, 0)
	constraintInfoList = make([]*ConstraintInfo, 0)
	constraintInfoList = append(constraintInfoList, &ConstraintInfo{
		ConstraintName:    "pk_person_id",
		ConstraintType:    ConstraintTypeP,
		ColumnList:        []string{"id"},
		ConstraintContent: "CONSTRAINT pk_person_id PRIMARY KEY (id)",
	})
	tableInfo.ConstraintList = constraintInfoList
	tableInfoList = append(tableInfoList, tableInfo)
	tableInfoLists[3] = tableInfoList

	testCases := []testPgContext{
		{
			Name:             "alter table rename index name",
			SQL:              `ALTER INDEX idx_name RENAME TO new_idx_name;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[0], IndexInfoList: indexInfoListModify},
		},
		{
			Name:             "alter table rename table name",
			SQL:              `ALTER TABLE person RENAME TO person_ok;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[1], IndexInfoList: indexInfoList},
		},
		{
			Name:             "alter table rename column name",
			SQL:              `ALTER TABLE person RENAME COLUMN id TO id1;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[2], IndexInfoList: indexInfoList},
		},
		{
			Name:             "alter table rename constraint name",
			SQL:              `ALTER TABLE person RENAME CONSTRAINT pk_person TO pk_person_id;`,
			ExpectSchema:     "test",
			ExpectSchemaInfo: SchemaInfo{SchemaName: "test", TableInfoList: tableInfoLists[3], IndexInfoList: indexInfoList},
		},
	}

	for _, testCase := range testCases {
		i := NewMockDriver(nil, "test", "test", nil, nil)
		tableInfo = &TableInfo{
			TableName: "person",
			OwnerName: "test",
		}
		tableInfoList = make([]*TableInfo, 0)
		columnInfoListTemp = copyColumnList(columnInfoListTemp, columnInfoList)
		tableInfo.ColumnInfoList = columnInfoListTemp
		tableInfoList = make([]*TableInfo, 0)
		constraintInfoList = make([]*ConstraintInfo, 0)
		constraintInfoList = append(constraintInfoList, &ConstraintInfo{
			ConstraintName:    "pk_person",
			ConstraintType:    ConstraintTypeP,
			ColumnList:        []string{"id"},
			ConstraintContent: "CONSTRAINT pk_person PRIMARY KEY (id)",
		})
		tableInfo.ConstraintList = constraintInfoList
		tableInfoList = append(tableInfoList, tableInfo)
		indexInfoList = make([]*IndexInfo, 0)
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

		i.pgContext.UsingType = UsingTypeOffline
		runTest(t, testCase, i)
	}
}

func TestOfflineCreateIndex(t *testing.T) {
	i := NewMockDriver(nil, "test", "test", nil, nil)
	indexInfoList := make([]*IndexInfo, 0)
	indexInfoList = append(indexInfoList, &IndexInfo{
		IndexName:  "idx_name",
		OwnerName:  "test",
		TableName:  "person",
		ColumnList: []string{"name"},
	})
	testCase := testPgContext{
		Name:             "create index",
		SQL:              `create index idx_name on person(name)`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test", IndexInfoList: indexInfoList},
	}

	i.pgContext.UsingType = UsingTypeOffline
	runTest(t, testCase, i)
}

func TestOfflineDropIndex(t *testing.T) {
	i := NewMockDriver(nil, "test", "test", nil, nil)
	testCase := testPgContext{
		Name:             "drop index",
		SQL:              `drop index idx_name`,
		ExpectSchema:     "test",
		ExpectSchemaInfo: SchemaInfo{SchemaName: "test"},
	}

	i.pgContext.UsingType = UsingTypeOffline
	runTest(t, testCase, i)
}

// runTest :运行测试
func runTest(t *testing.T, testCase testPgContext, i *driverImpl) bool {
	return t.Run(testCase.Name, func(t *testing.T) {
		ast, err := SqlParserFunc(testCase.SQL)
		assert.NoError(t, err)
		handlePgContext := HandlePgContext{PgContext: i.pgContext, RawStmt: ast.(*parser.RawStmt)}
		err = handlePgContext.Handle()
		assert.NoError(t, err)
		actualSchemaInfo := handlePgContext.PgContext.DatabaseInfo.SchemaInfoMap[testCase.ExpectSchema]
		if actualSchemaInfo == nil {
			actualSchemaInfo = &SchemaInfo{}
		}
		actualResult, msg := equalSchemaInfo(&testCase.ExpectSchemaInfo, actualSchemaInfo)
		if len(msg) > 0 {
			fmt.Printf("test error output=[%s]\n", msg)
		}
		assert.True(t, actualResult)
	})
}

func copyColumnList(columnInfoListTemp []*ColumnInfo, columnInfoList [4]*ColumnInfo) []*ColumnInfo {
	columnInfoListTemp = make([]*ColumnInfo, len(columnInfoList[:3]))
	for i, info := range columnInfoList[:3] {
		clone := *info                 // 使用指针的解引用进行值的复制
		columnInfoListTemp[i] = &clone // 创建新的指针并赋值给新切片
	}
	return columnInfoListTemp
}

func equalSchemaInfo(s1, s2 *SchemaInfo) (bool, string) {
	if s1 == nil || s2 == nil {
		// 检查两个结构体是否都为 nil
		return s1 == s2, ""
	}
	if s1.SchemaName != s2.SchemaName {
		return false, "SchemaName is different"
	}
	if s1.IsLoadFromDb != s2.IsLoadFromDb {
		return false, "SchemaName's IsLoadFromDb is different"
	}
	if len(s1.TableInfoList) != len(s2.TableInfoList) {
		return false, "Table length is different"
	}
	if len(s1.IndexInfoList) != len(s2.IndexInfoList) {
		return false, "Index length is different"
	}
	// 递归比较每个表和索引的信息
	for i := range s1.TableInfoList {
		if s1.TableInfoList[i].TableName != s2.TableInfoList[i].TableName {
			return false, fmt.Sprintf("TableName=%s is different", s1.TableInfoList[i].TableName)
		}
		if s1.TableInfoList[i].OwnerName != s2.TableInfoList[i].OwnerName {
			return false, fmt.Sprintf("TableName=%s OwnerName=%s is different",
				s1.TableInfoList[i].TableName, s1.TableInfoList[i].OwnerName)
		}
		if len(s1.TableInfoList[i].ColumnInfoList) != len(s2.TableInfoList[i].ColumnInfoList) {
			return false, fmt.Sprintf("TableName=%s OwnerName=%s ColumnInfoList length is different, "+
				"expect length=%d actual length=%d", s1.TableInfoList[i].TableName, s1.TableInfoList[i].OwnerName,
				len(s1.TableInfoList[i].ColumnInfoList), len(s2.TableInfoList[i].ColumnInfoList))
		}
		if len(s1.TableInfoList[i].ConstraintList) != len(s2.TableInfoList[i].ConstraintList) {
			return false, fmt.Sprintf("TableName=%s OwnerName=%s ConstraintList length is different, "+
				"expect length=%d actual length=%d", s1.TableInfoList[i].TableName, s1.TableInfoList[i].OwnerName,
				len(s1.TableInfoList[i].ConstraintList), len(s2.TableInfoList[i].ConstraintList))
		}
		for j := range s1.TableInfoList[i].ColumnInfoList {
			if s1.TableInfoList[i].ColumnInfoList[j].ColumnName != s2.TableInfoList[i].ColumnInfoList[j].ColumnName {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s ColumnName is different, "+
					"expect ColumnName=%s actual ColumnName=%s", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].ColumnName,
					s2.TableInfoList[i].ColumnInfoList[j].ColumnName)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].TableName != s2.TableInfoList[i].ColumnInfoList[j].TableName {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s TableName is different, "+
					"expect TableName=%s actual TableName=%s", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].TableName,
					s2.TableInfoList[i].ColumnInfoList[j].TableName)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].OwnerName != s2.TableInfoList[i].ColumnInfoList[j].OwnerName {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s OwnerName is different, "+
					"expect OwnerName=%s actual OwnerName=%s", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].OwnerName,
					s2.TableInfoList[i].ColumnInfoList[j].OwnerName)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].ColumnType != s2.TableInfoList[i].ColumnInfoList[j].ColumnType {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s ColumnType value is different, "+
					"expect ColumnType=%s actual ColumnType=%s", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].ColumnType,
					s2.TableInfoList[i].ColumnInfoList[j].ColumnType)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].ColumnLength != s2.TableInfoList[i].ColumnInfoList[j].ColumnLength {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s ColumnLength length is different, "+
					"expect length=%d actual length=%d", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].ColumnLength,
					s2.TableInfoList[i].ColumnInfoList[j].ColumnLength)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].ColumnPrecision != s2.TableInfoList[i].ColumnInfoList[j].ColumnPrecision {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s ColumnPrecision length is different, "+
					"expect length=%d actual length=%d", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].ColumnPrecision,
					s2.TableInfoList[i].ColumnInfoList[j].ColumnPrecision)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].IsNullable != s2.TableInfoList[i].ColumnInfoList[j].IsNullable {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s IsNullable value is different, "+
					"expect value=%t actual value=%t", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].IsNullable,
					s2.TableInfoList[i].ColumnInfoList[j].IsNullable)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].IsPrimaryColumn != s2.TableInfoList[i].ColumnInfoList[j].IsPrimaryColumn {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s IsPrimaryColumn value is different, "+
					"expect value=%t actual value=%t", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].IsPrimaryColumn,
					s2.TableInfoList[i].ColumnInfoList[j].IsPrimaryColumn)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].IsUnique != s2.TableInfoList[i].ColumnInfoList[j].IsUnique {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s IsUnique value is different, "+
					"expect value=%t actual value=%t", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].IsUnique,
					s2.TableInfoList[i].ColumnInfoList[j].IsUnique)
			}
			if s1.TableInfoList[i].ColumnInfoList[j].IsLoadFromDb != s2.TableInfoList[i].ColumnInfoList[j].IsLoadFromDb {
				return false, fmt.Sprintf("TableName=%s OwnerName=%s IsLoadFromDb value is different, "+
					"expect value=%t actual value=%t", s1.TableInfoList[i].TableName,
					s1.TableInfoList[i].OwnerName, s1.TableInfoList[i].ColumnInfoList[j].IsLoadFromDb,
					s2.TableInfoList[i].ColumnInfoList[j].IsLoadFromDb)
			}
		}
		for j := range s1.TableInfoList[i].ConstraintList {
			if s1.TableInfoList[i].ConstraintList[j].ConstraintName != s2.TableInfoList[i].ConstraintList[j].ConstraintName {
				return false, "constraint ConstraintName is different"
			}
			if s1.TableInfoList[i].ConstraintList[j].ConstraintType != s2.TableInfoList[i].ConstraintList[j].ConstraintType {
				return false, "constraint ConstraintType is different"
			}
			if s1.TableInfoList[i].ConstraintList[j].ConstraintContent != s2.TableInfoList[i].ConstraintList[j].ConstraintContent {
				return false, fmt.Sprintf("constraint ConstraintContent is different, "+
					"expect content=%s actual content=%s", s1.TableInfoList[i].ConstraintList[j].ConstraintContent,
					s2.TableInfoList[i].ConstraintList[j].ConstraintContent)
			}
			if s1.TableInfoList[i].ConstraintList[j].ReferencedTable != s2.TableInfoList[i].ConstraintList[j].ReferencedTable {
				return false, "constraint ReferencedTable is different"
			}
			if len(s1.TableInfoList[i].ConstraintList[j].ColumnList) != len(s2.TableInfoList[i].ConstraintList[j].ColumnList) {
				return false, "constraint ColumnList length is different"
			}
			if len(s1.TableInfoList[i].ConstraintList[j].ReferencedColumns) != len(s2.TableInfoList[i].ConstraintList[j].ReferencedColumns) {
				return false, "constraint ReferencedColumns length is different"
			}
			if s1.TableInfoList[i].ConstraintList[j].CheckCondition != s2.TableInfoList[i].ConstraintList[j].CheckCondition {
				return false, "constraint CheckCondition is different"
			}
		}
	}
	for i := range s1.IndexInfoList {
		if s1.IndexInfoList[i].IndexName != s2.IndexInfoList[i].IndexName {
			return false, fmt.Sprintf("IndexName is different, expect value=%s actual value=%s",
				s1.IndexInfoList[i].IndexName, s2.IndexInfoList[i].IndexName)
		}
		if s1.IndexInfoList[i].TableName != s2.IndexInfoList[i].TableName {
			return false, fmt.Sprintf("IndexName=%s TableName is different, expect value=%s actual value=%s",
				s1.IndexInfoList[i].IndexName, s1.IndexInfoList[i].TableName, s2.IndexInfoList[i].TableName)
		}
		if s1.IndexInfoList[i].OwnerName != s2.IndexInfoList[i].OwnerName {
			return false, fmt.Sprintf("IndexName=%s OwnerName is different, expect value=%s actual value=%s",
				s1.IndexInfoList[i].IndexName, s1.IndexInfoList[i].OwnerName, s2.IndexInfoList[i].OwnerName)
		}
		if s1.IndexInfoList[i].IsUnique != s2.IndexInfoList[i].IsUnique {
			return false, fmt.Sprintf("IndexName=%s IsUnique is different, expect value=%t actual value=%t",
				s1.IndexInfoList[i].IndexName, s1.IndexInfoList[i].IsUnique, s2.IndexInfoList[i].IsUnique)
		}
		if s1.IndexInfoList[i].IsLoadFromDb != s2.IndexInfoList[i].IsLoadFromDb {
			return false, fmt.Sprintf("IndexName=%s IsLoadFromDb is different, expect value=%t actual value=%t",
				s1.IndexInfoList[i].IndexName, s1.IndexInfoList[i].IsLoadFromDb, s2.IndexInfoList[i].IsLoadFromDb)
		}
		if len(s1.IndexInfoList[i].ColumnList) != len(s2.IndexInfoList[i].ColumnList) {
			return false, fmt.Sprintf("IndexName=%s ColumnList length is different, expect value=%d actual value=%d",
				s1.IndexInfoList[i].IndexName, len(s1.IndexInfoList[i].ColumnList), len(s2.IndexInfoList[i].ColumnList))
		}
	}
	return true, ""
}
