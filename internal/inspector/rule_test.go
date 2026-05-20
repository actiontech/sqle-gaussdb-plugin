package inspector

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	hclog "github.com/hashicorp/go-hclog"
)

// Inspect implements driver.Driver interface
func newTestDriver(ruleName string, database, schema string, conn *executor.Executor) *driverImpl {
	rules := make([]*driverV2.Rule, 0, len(RuleHandlerMap))
	dsnParams := &driverV2.DSN{}
	ah := &driverPkg.AuditHandler{
		SqlParserFn:      SqlParserFunc,
		RuleToRawHandler: make(map[string]driverPkg.RawSQLRuleHandler),
		RuleToASTHandler: make(map[string]driverPkg.AstSQLRuleHandler),
	}
	for _, rh := range RuleHandlers {
		rule := rh.Rule
		if rule.Name == RuleId9 {
			rule.Params.SetParamValue("max_column_count", "10")
		}
		if rule.Name == ruleName {
			rules = append(rules, &rule)
			ruleHandler := RuleHandlerMap[ruleName]
			if ruleHandler.RawSQLHandler != nil {
				ah.RuleToRawHandler[ruleName] = ruleHandler.RawSQLHandler
			} else if ruleHandler.AstSQLHandler != nil {
				ah.RuleToASTHandler[ruleName] = ruleHandler.AstSQLHandler
			}
		}
	}
	return NewMockDriver(
		&driverV2.Config{
			DSN:   dsnParams,
			Rules: rules,
		},
		database,
		schema,
		ah,
		conn,
	)
}

type testResults struct {
	*driverV2.AuditResults
}

func newTestResults() *testResults {
	return &testResults{
		&driverV2.AuditResults{},
	}
}

func (r *testResults) add(ruleId string, args ...interface{}) *testResults {
	r.Add(RuleHandlerMap[ruleId].Rule.Level, "", localizeMessage(RuleHandlerMap[ruleId].Message), args...)
	return r
}

type epOutPut struct {
	ColumnName string
	Row        string
}

func testSingleSqlAudit(ruleName string, t *testing.T, sql string, expectResult *testResults, pgContext *PgContext, dataTypes, tables []string) {
	testAuditWithEpMockConn(ruleName, t, []string{sql}, []*testResults{expectResult}, &epOutPut{}, pgContext, dataTypes, tables)
}

func testAuditWithEpMockConn(ruleName string, t *testing.T, sqls []string, expectResult []*testResults, mockEp *epOutPut, pgContext *PgContext, dataTypes, tables []string) {
	var d *driverImpl
	var exe *executor.Executor
	var err error
	var mock sqlmock.Sqlmock
	exe, mock, err = executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		t.Error(err)
		return
	}
	d = newTestDriver(ruleName, "test", "test", exe)
	d.pgContext = pgContext

	if mockEp != nil {
		if pgContext.UsingType == UsingTypeOnline {
			// mock pg data type
			dataTypeRows := mock.NewRows([]string{"typname"})
			for _, dataType := range dataTypes {
				dataTypeRows.AddRow(dataType)
			}
			mock.ExpectQuery(`SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid 
                          WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)`).WillReturnRows(dataTypeRows)
			// mock explain
			mock.ExpectQuery("explain (FORMAT JSON) " + sqls[0]).
				WillReturnRows(mock.NewRows([]string{mockEp.ColumnName}).AddRow(mockEp.Row))
			// mock get tables by schema
			tablesRows := mock.NewRows([]string{"table_name"})
			for _, table := range tables {
				tablesRows.AddRow(table)
			}
			mock.ExpectQuery(`SELECT table_name FROM information_schema.tables
		                 WHERE table_schema = $1 AND table_type = 'BASE TABLE'`).WillReturnRows(tablesRows)
		}
	}

	currentSchema := ""
	pgContext.Executor = exe
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: exe, pgContext: pgContext}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	results, err := d.Audit(ctx, sqls)
	if err != nil {
		t.Error(err)
		return
	}
	for i, r := range results {
		if r.Level() != expectResult[i].Level() {
			t.Errorf("expect level is %s, actual is %s\nsqls: %s", expectResult[i].Level(), r.Level(), sqls)
		}
		if r.Message() != expectResult[i].Message() {
			t.Errorf("expect message is %s\n actual is %s\nsqls: %s", expectResult[i].Message(), r.Message(), sqls)
		}
	}
	return
}

// mockDbOperate :模拟数据库查询
type mockDbOperate struct {
	sql      string
	args     []string
	fields   []string
	mockData []map[string]sql.NullString
}

// mock数据库查询测试方法
func testAuditWithDbQueryMockConn(ruleName string, t *testing.T, sqls []string, expectResult []*testResults, dbOperates *[]*mockDbOperate, pgContext *PgContext) {
	var d *driverImpl
	var exe *executor.Executor
	var err error
	var mock sqlmock.Sqlmock

	exe, mock, err = executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		t.Error(err)
		return
	}
	d = newTestDriver(ruleName, "test", "test", exe)

	if len(*dbOperates) > 0 {
		// mock db query
		for _, operate := range *dbOperates {
			rows := sqlmock.NewRows(operate.fields)
			for _, row := range operate.mockData {
				values := make([]driver.Value, 0)
				for _, field := range operate.fields {
					value, _ := row[field].Value()
					values = append(values, value)
				}
				rows.AddRow(values...)
			}
			if len(operate.args) > 0 {
				arguments := make([]driver.Value, 0)
				for _, arg := range operate.args {
					arguments = append(arguments, arg)
				}
				mock.ExpectQuery(operate.sql).WithArgs(arguments...).WillReturnRows(rows)
			} else {
				mock.ExpectQuery(operate.sql).WillReturnRows(rows)
			}
		}
	}

	if pgContext == nil {
		database, schema := "test", "test"
		schemaInfoMap := make(map[string]*SchemaInfo)
		tableInfoList := make([]*TableInfo, 0)
		indexInfoList := make([]*IndexInfo, 0)
		indexInfo := &IndexInfo{IndexName: "uniq_name", OwnerName: "test", TableName: "test"}
		indexInfoList = append(indexInfoList, indexInfo)
		schemaInfoMap[schema] = &SchemaInfo{
			SchemaName:    schema,
			TableInfoList: tableInfoList,
			IndexInfoList: indexInfoList,
		}
		databaseInfo := &DatabaseInfo{
			DatabaseName:  "test",
			CurrentSchema: schema,
			SchemaInfoMap: schemaInfoMap,
		}
		d.pgContext = &PgContext{
			CurrentDatabase:      database,
			Executor:             exe,
			DatabaseInfo:         databaseInfo,
			DeletedSchemaMap:     make(map[string]string),
			DeletedTableMap:      make(map[string]string),
			DeletedIndexMap:      make(map[string]string),
			DeletedColumnMap:     make(map[string]string),
			DeletedConstraintMap: make(map[string]string),
		}
	} else {
		pgContext.Executor = exe
	}
	d.pgContext = pgContext

	currentSchema := ""
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: exe, pgContext: d.pgContext}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	results, err := d.Audit(ctx, sqls)
	if err != nil {
		t.Error(err)
		return
	}
	for i, r := range results {
		if r.Level() != expectResult[i].Level() {
			t.Errorf("expect level is %s, actual is %s\nsqls: %s", expectResult[i].Level(), r.Level(), sqls)
		}
		if r.Message() != expectResult[i].Message() {
			t.Errorf("expect message is %s\n actual is %s\nsqls: %s", expectResult[i].Message(), r.Message(), sqls)
		}
	}
	return
}

// 测试两条sql一起审核的情况
func testTwoSqlAudit(ruleName string, t *testing.T, sqls []string, expectResults []*testResults, pgContext *PgContext, dataTypes, tables []string) {
	testAuditWithEpMockConn(ruleName, t, sqls, expectResults, &epOutPut{}, pgContext, dataTypes, tables)
}

func TestDriverImpl_Rule1(t *testing.T) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "id",
		TableName:  "t1",
		OwnerName:  "public",
	})
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "t1",
		ColumnInfoList: columnInfoList,
		OwnerName:      "public",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId1, t, "SELECT * FROM t1", newTestResults().add(RuleId1), pgContext, nil, nil)
	testSingleSqlAudit(RuleId1, t, "SELECT id FROM t1", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule2(t *testing.T) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "id",
		TableName:  "t1",
		OwnerName:  "public",
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "core",
		TableName:  "t1",
		OwnerName:  "public",
	})
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "t1",
		ColumnInfoList: columnInfoList,
		OwnerName:      "public",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId2, t, "DELETE FROM t1", newTestResults().add(RuleId2), pgContext, nil, nil)
	testSingleSqlAudit(RuleId2, t, "DELETE FROM t1 WHERE id = 1", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId2, t, "UPDATE t1 SET core=100", newTestResults().add(RuleId2), pgContext, nil, nil)
	testSingleSqlAudit(RuleId2, t, "UPDATE t1 SET core=100 WHERE id =1", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule3(t *testing.T) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "id",
		TableName:  "t1",
		OwnerName:  "public",
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "core",
		TableName:  "t1",
		OwnerName:  "public",
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "age",
		TableName:  "t1",
		OwnerName:  "public",
	})
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "t1",
		ColumnInfoList: columnInfoList,
		OwnerName:      "public",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId3, t, "SELECT id, avg(age) as avg_age FROM t1 GROUP BY id HAVING avg_age >1", newTestResults().add(RuleId3), pgContext, nil, nil)
}

func TestDriverImpl_Rule4(t *testing.T) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnInfoListForDb1 := make([]*ColumnInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "id",
		TableName:  "t1",
		OwnerName:  "public",
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "core",
		TableName:  "t1",
		OwnerName:  "public",
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "age",
		TableName:  "t1",
		OwnerName:  "public",
	})
	columnInfoListForDb1 = append(columnInfoListForDb1, &ColumnInfo{
		ColumnName: "id",
		TableName:  "t1",
		OwnerName:  "db1",
	})
	tableInfoList := make([]*TableInfo, 0)
	tableInfoListForDb1 := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "t1",
		ColumnInfoList: columnInfoList,
		OwnerName:      "public",
	})
	tableInfoListForDb1 = append(tableInfoListForDb1, &TableInfo{
		TableName:      "t1",
		ColumnInfoList: columnInfoListForDb1,
		OwnerName:      "db1",
	})
	indexInfoList := make([]*IndexInfo, 0)
	indexInfoList = append(indexInfoList, &IndexInfo{
		IndexName: "idx_1",
		OwnerName: "public",
		TableName: "t1",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
		IndexInfoList: indexInfoList,
	}
	schemaInfoMap["db1"] = &SchemaInfo{
		SchemaName:    "db1",
		TableInfoList: tableInfoListForDb1,
	}
	pgContext := &PgContext{
		UsingType: UsingTypeOffline,
		DatabaseInfo: &DatabaseInfo{
			DatabaseName:  "postgres",
			CurrentSchema: "public",
			SchemaInfoMap: schemaInfoMap,
		},
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId4, t, "drop index idx_1", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId4, t, "drop table db1.t1", newTestResults().add(RuleId4), pgContext, nil, nil)
	testSingleSqlAudit(RuleId4, t, "drop table t1", newTestResults().add(RuleId4), pgContext, nil, nil)
	testSingleSqlAudit(RuleId4, t, "drop view v1", newTestResults().add(RuleId4), pgContext, nil, nil)
	testSingleSqlAudit(RuleId4, t, "drop database db1", newTestResults().add(RuleId4), pgContext, nil, nil)
}

func TestDriverImpl_Rule5(t *testing.T) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "id",
		TableName:  "t1",
		OwnerName:  "public",
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "core",
		TableName:  "t1",
		OwnerName:  "public",
	})
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "t1",
		ColumnInfoList: columnInfoList,
		OwnerName:      "public",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId5, t, "create view v1 as select * from t1", newTestResults().add(RuleId5), pgContext, nil, nil)
	testSingleSqlAudit(RuleId5, t, "create or REPLACE view v1 as select * from t1", newTestResults().add(RuleId5), pgContext, nil, nil)
}

func TestDriverImpl_Rule6(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}}
	testSingleSqlAudit(RuleId6, t, `
		CREATE TRIGGER check_update
			BEFORE UPDATE on accounts
			FOR EACH ROW
			EXECUTE FUNCTION check_account_update();`,
		newTestResults().add(RuleId6), pgContext, nil, nil)
}

func TestDriverImpl_Rule7(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId7, t, `
		CREATE TABLE t1(
			id int,
			name varchar(255)
		);
		`, newTestResults().add(RuleId7), pgContext, nil, nil)

	testSingleSqlAudit(RuleId7, t, `
		CREATE TABLE t11(
			id int primary key,
			name varchar(255)
		);
		`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId7, t, `
		CREATE TABLE t111(
			id int,
			name varchar(255),
			primary key (id)
		);
		`, newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule8(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId8, t, `
		CREATE TABLE contacts(
			contact_id INT GENERATED ALWAYS AS IDENTITY,
			customer_id INT,
			contact_name VARCHAR(255) NOT NULL,
			phone VARCHAR(15),
			email VARCHAR(100),
			PRIMARY KEY(contact_id),
			CONSTRAINT fk_customer
				FOREIGN KEY(customer_id)
				REFERENCES customers(customer_id)
		);
		`, newTestResults().add(RuleId8), pgContext, nil, nil)
}

func TestDriverImpl_Rule9(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId9, t, `
		CREATE TABLE t1(
			id int primary key,
			id2 int,
			id3 int,
			id4 int,
			id5 int,
			id6 int,
			id7 int,
			id8 int,
			id9 int
		);
		`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId9, t, `
		CREATE TABLE t11(
			id int primary key,
			id2 int,
			id3 int,
			id4 int,
			id5 int,
			id6 int,
			id7 int,
			id8 int,
			id9 int,
			id10 int
		);
		`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId9, t, `
		CREATE TABLE t111(
			id int primary key,
			id2 int,
			id3 int,
			id4 int,
			id5 int,
			id6 int,
			id7 int,
			id8 int,
			id9 int,
			id10 int,
			id11 int
		);
		`, newTestResults().add(RuleId9, 10, 11), pgContext, nil, nil)
}

func TestDriverImpl_Rule10(t *testing.T) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "id",
		TableName:  "t1",
		OwnerName:  "public",
	})
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "t1",
		ColumnInfoList: columnInfoList,
		OwnerName:      "public",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId10].Rule
	rule.Params.SetParamValue("sql_length", "20")
	defer func() {
		rule.Params.SetParamValue("sql_length", "1024")
	}()

	testSingleSqlAudit(RuleId10, t, `select id   from t1` /* length 19 */, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId10, t, `select id    from t1` /* length 20 */, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId10, t, `select id     from t1` /* length 21 */, newTestResults().add(RuleId10, 20), pgContext, nil, nil)
}

func TestDriverImpl_Rule11(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t11",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "col_name",
				TableName:  "t11",
				OwnerName:  "public",
			},
		},
		OwnerName: "public",
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t111",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "col_name",
				TableName:  "t111",
				OwnerName:  "public",
			},
		},
		OwnerName: "public",
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table1",
				OwnerName:  "public",
			},
			{
				ColumnName: "id1",
				TableName:  "table1",
				OwnerName:  "public",
			},
			{
				ColumnName: "id2",
				TableName:  "table1",
				OwnerName:  "public",
			},
			{
				ColumnName: "id3",
				TableName:  "table1",
				OwnerName:  "public",
			},
			{
				ColumnName: "id4",
				TableName:  "table1",
				OwnerName:  "public",
			},
		},
		OwnerName: "public",
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "goods",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "goods",
				OwnerName:  "public",
			},
			{
				ColumnName: "id1",
				TableName:  "goods",
				OwnerName:  "public",
			},
			{
				ColumnName: "id2",
				TableName:  "goods",
				OwnerName:  "public",
			},
			{
				ColumnName: "id3",
				TableName:  "goods",
				OwnerName:  "public",
			},
			{
				ColumnName: "id4",
				TableName:  "goods",
				OwnerName:  "public",
			},
		},
		OwnerName: "public",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId11].Rule
	rule.Params.SetParamValue("expect_max_index_column_number", "3")
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId11, t, sql, newTestResults(), pgContext, nil, nil)
	}
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId11, t, sql, newTestResults().add(RuleId11, 3), pgContext, nil, nil)
	}

	// ------ create table test ------
	// url: https://www.postgresql.org/docs/14/sql-createtable.html
	// 不触发规则
	assertSqlCorrect(`create table t1(id int,id1 int,id2 int,id3 int,id4 int,name varchar(255),primary key (id));`)
	assertSqlCorrect(`create table t1_1(id1 int primary key, id2 int, id3 int, id4 int, title varchar(100))`)
	assertSqlCorrect(`create table t1_2(id int primary key,name varchar(255), title varchar(100));`)
	assertSqlCorrect(`create table t1_3(id int primary key,name varchar(20) UNIQUE, title varchar(100), director varchar(100), rating varchar(100));`)
	assertSqlCorrect(`
		create table t1_4(
			did integer primary key GENERATED BY DEFAULT AS IDENTITY,
			name varchar(40) NOT NULL CHECK (name <> ''));
	`)
	assertSqlCorrect(`create table t1_5(column1 int primary key, column2 int, column3 int, column4 int, title varchar(100))`)

	// ------- create index test  -------
	// url: https://www.postgresql.org/docs/14/sql-createindex.html
	// 不触发规则
	assertSqlCorrect(`create index idx_1 on t1_1(id2);`)
	assertSqlCorrect(`create unique index idx_2 on t1_2(title);`)
	assertSqlCorrect(`create unique index idx_3 on t1_3(title) include (director, rating);`)
	assertSqlCorrect(`create index idx_4 on t1(id1, id2, id3);`)
	// 触发规则
	assertSqlIncorrect(`create index idx_5 on t1_5(column1,column2,column3,column4);`)
	assertSqlIncorrect(`create unique index idx_6 on t1_1(id1, id2, id3, id4);`)

	// ------ alter table test  ------
	// url: https://www.postgresql.org/docs/14/sql-altertable.html
	// 不触发规则
	assertSqlCorrect(`alter table t111 drop column col_name;`)
	assertSqlCorrect(`
		alter table t11
			add column id1 int,
			add column id2 int,
			add column id3 int,
			add column id4 int,
			drop column col_name;
	`)
	assertSqlCorrect(`
		alter table t11
			ADD COLUMN status varchar(30) DEFAULT 'old',
  			ALTER COLUMN status SET default 'current';
  	`)
	assertSqlCorrect(`alter table table1 add constraint  myUnique check(id > 10);`)
	assertSqlCorrect(`alter table goods add constraint unique_goods_sid unique(sid);`)
	assertSqlCorrect(`alter table table1 add constraint  myUnique check(id1 > 10 and id2 > 10 and id3 > 10 and id4 > 10);`)
	// 触发规则
	assertSqlIncorrect(`alter table goods add constraint unique_goods_sid unique(id1, id2, id3, id4);`)
	assertSqlIncorrect(`alter table goods add constraint uq_1 primary key (id1, id2, id3, id4);`)
	assertSqlIncorrect(`
		alter table goods
			add constraint  myUnique1 check(id > 10),
			add constraint uq_1 primary key (id1, id2, id3, id4),
			add constraint  myUnique2 check(id1 > 10 and id2 > 10 and id3 > 10 and id4 > 10)
	`)
	assertSqlIncorrect(`
		alter table goods
			add constraint uq_1 primary key (id1, id2, id3, id4),
			add constraint  myUnique1 check(id > 10),
			add constraint  myUnique2 check(id1 > 10 and id2 > 10 and id3 > 10 and id4 > 10)
	`)
}

func TestDriverImpl_Rule12(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId12].Rule
	rule.Params.SetParamValue("expect_index_name_prefix", "idx_")
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId12, t, sql, newTestResults(), pgContext, nil, nil)
	}
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId12, t, sql, newTestResults().add(RuleId12, "idx_"), pgContext, nil, nil)
	}

	// create table add index constraint
	// 不走规则
	assertSqlCorrect(`create table t1(id1 int primary key, id2 int, id3 int, id4 int, title varchar(100), title2 varchar(100), CONSTRAINT idx_id2 unique(id2))`)
	// 走规则
	assertSqlCorrect(`create table t1_1(id1 int primary key, id2 int, id3 int, id4 int, title varchar(100), title2 varchar(100), code varchar(100), CONSTRAINT idx1_id2 unique(id2))`)

	// alter table add index constraint
	// 不走规则
	assertSqlCorrect(`ALTER TABLE t1 ADD CONSTRAINT idx_id1 UNIQUE (id1);`)
	// 走规则
	assertSqlCorrect(`ALTER TABLE t1_1 ADD CONSTRAINT idx1_id1 UNIQUE (id1);`)

	// create index
	// 不走规则
	assertSqlCorrect(`create index idx_1 on t1(id3)`)
	assertSqlCorrect(`create unique index idx_2 on t1_1(id3)`)
	assertSqlCorrect(`create index idx_3 on t1(title collate "de_DE")`)
	assertSqlCorrect(`create index idx_4 on t1_1(title nulls first)`)
	// 走规则
	assertSqlIncorrect(`create index index1 on t1(id4)`)
	assertSqlIncorrect(`create index index2 on t1((lower(title2)))`)
	assertSqlIncorrect(`create index index3 on t1_1(title2) with (deduplicate_items = off)`)
	assertSqlIncorrect(`create index index4 on t1_1(code) tablespace indexspace`)
}

func TestDriverImpl_Rule15(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table_name",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "column1",
			},
			{
				ColumnName: "column2",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table_old",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "column1",
			},
			{
				ColumnName: "column2",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "col_old",
			},
			{
				ColumnName: "column2",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table3",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "col_old",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table3_1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "col_old",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "goods",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "sid",
			},
			{
				ColumnName: "tid",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "tid",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	assertCorrect := func(sql string) {
		correctResult := newTestResults()
		testSingleSqlAudit(RuleId15, t, sql, correctResult, pgContext, nil, nil)
	}
	assertIncorrect := func(sql string, keywords []string) {
		incorrectResult := newTestResults().add(RuleId15, strings.Join(keywords, ", "))
		testSingleSqlAudit(RuleId15, t, sql, incorrectResult, pgContext, nil, nil)
	}

	singleKeyword := []string{"abs"}
	doubleKeywords := []string{"abs", "bit"}

	//create tabel key word test
	assertIncorrect(`CREATE TABLE test.ABS (
		        id int primary key,
	            id2 int,
	            id3 int,
	            id4 int,
	            id5 int
	            );`, singleKeyword)
	assertCorrect(`CREATE TABLE user1 (
		        id int primary key,
		        id2 int,
		        id3 int,
		        id4 int,
		        id5 int
		        );`)

	//create tabel key word in columns
	assertIncorrect(`CREATE TABLE login (
		        id int primary key,
		        id2 int,
		        id3 int,
		        id4 int,
		        ABS date
		        );`, singleKeyword)
	assertCorrect(`CREATE TABLE login_1 (
		        id int primary key,
		        id2 int,
		        id3 int,
		        id4 int,
		        date1 date
		        );`)
	// create table key word in constraint
	assertIncorrect(`
	           CREATE TABLE tableconstrainttest(
				id1 int primary key,
				id2 int  NOT NULL UNIQUE,
				id3 int,
				id4 int,
				constraint ABS unique(id3),
				constraint constraint2 check(id4 < 1000)
				);`, singleKeyword)
	assertIncorrect(`
	           CREATE TABLE tableconstrainttest_1(
				id1 int primary key,
				id2 int  NOT NULL UNIQUE,
				id3 int,
				id4 int,
				constraint ABS unique(id3),
				constraint BIT check(id4 < 1000)
				);`, doubleKeywords)
	assertCorrect(`
	           CREATE TABLE tableconstrainttest_2(
				id1 int primary key,
				id2 int  NOT NULL UNIQUE,
				id3 int,
				id4 int,
				constraint constraint1 unique(id3),
				constraint constraint2 check(id4 < 1000)
				);`)
	//create  index kew word test
	assertIncorrect(`CREATE INDEX ABS ON test.table_name (column1,column2);`, singleKeyword)
	assertCorrect(`CREATE INDEX idx_column1 ON test.table_name (column1);`)

	//create database key word test
	assertIncorrect(`CREATE DATABASE ABS;`, singleKeyword)
	assertCorrect(`CREATE DATABASE db1;`)

	//"create tabel as" key word test
	assertIncorrect(`create table ABS  as select * from test_user;`, singleKeyword)
	assertCorrect(`create table table1  as select * from test_user;`)

	//create schema key word test
	assertIncorrect(`create schema ABS;`, singleKeyword)
	assertCorrect(`create schema schema1;`)

	//alter table rename table name to key word
	assertIncorrect(`alter table table_old rename to ABS;`, singleKeyword)
	assertCorrect(`alter table ABS rename to table_old;`)

	// alter column rename to key word
	assertIncorrect(`alter table table2 rename column col_old to ABS;`, singleKeyword)
	assertCorrect(`alter table table2 rename column ABS to col_old;`)

	// add column key word test
	assertIncorrect(`alter table table3 add ABS date;`, singleKeyword)
	assertCorrect(`alter table table3 add date1 date;`)
	assertIncorrect(`alter table table3_1 add ABS date, add BIT date;`, doubleKeywords)
	assertCorrect(`alter table table3 add column1 date;`)

	// add constraint key word test
	assertIncorrect(`alter table goods add constraint ABS unique(sid);`, singleKeyword)
	assertCorrect(`alter table goods add constraint unique_goods_sid unique(sid);`)

	assertIncorrect(`alter table goods add constraint ABS unique(sid),add constraint constraint2 unique(tid);`, singleKeyword)
	assertIncorrect(`alter table goods add constraint constraint1 unique(sid),add constraint ABS unique(tid);`, singleKeyword)
	assertIncorrect(`alter table goods add constraint ABS unique(sid),add constraint BIT unique(tid);`, doubleKeywords)
	assertCorrect(`alter table goods add constraint constraint1 unique(sid),add constraint constraint2 unique(tid);`)

	assertIncorrect(`alter table table1 add constraint ABS check(id > 10);`, singleKeyword)
	assertCorrect(`alter table table1 add constraint myuniquecontraint check(id > 10);`)

	assertIncorrect(`alter table table1 add constraint ABS check(id > 10),add constraint constraint2 check(tid < 1000);`, singleKeyword)
	assertIncorrect(`alter table table1 add constraint constraint1 check(id > 10),add constraint ABS check(tid < 1000);`, singleKeyword)
	assertIncorrect(`alter table table1 add constraint ABS check(id > 10),add constraint BIT check(tid < 1000);`, doubleKeywords)
	assertCorrect(`alter table table1 add constraint constraint1 check(id > 10),add constraint constraint2 check(tid < 1000);`)
}

func TestDriverImpl_Rule16(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "tablename",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "name",
			},
			{
				ColumnName: "first_name",
			},
			{
				ColumnName: "last_name",
			},
			{
				ColumnName: "customer",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "customer",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "name",
			},
			{
				ColumnName: "first_name",
			},
			{
				ColumnName: "last_name",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	assertCorrect := func(sql string) {
		correctResult := newTestResults()
		testSingleSqlAudit(RuleId16, t, sql, correctResult, pgContext, nil, nil)
	}
	assertIncorrect := func(sql string, keywords ...string) {
		incorrectResult := newTestResults().add(RuleId16, strings.Join(keywords, ", "))
		testSingleSqlAudit(RuleId16, t, sql, incorrectResult, pgContext, nil, nil)
	}
	//simple sql
	assertCorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE 'her';`)
	assertCorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE 'her%';`)
	assertCorrect(`SELECT first_name, last_name FROM customer WHERE first_name = '%aher%';`)
	assertCorrect(`SELECT first_name, last_name FROM customer WHERE first_name = '%aher';`)
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE '%aher%';`, "%aher%")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE '%aher';`, "%aher")

	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE 'her' and last_name LIKE '%aher' ;`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE 'her%' or last_name LIKE '%aher';`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE '%aher' and last_name LIKE '%bher' ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name LIKE '%aher' or last_name LIKE '%bher' ;`, "%aher", "%bher")

	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE exists (select name from tablename WHERE first_name LIKE '%aher') and  exists (select name from tablename WHERE first_name LIKE '%bher') ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE exists (select name from tablename WHERE first_name LIKE '%aher') or  exists (select name from tablename WHERE last_name LIKE '%bher') ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name in (select name from tablename WHERE first_name LIKE '%aher') and last_name in (select name from tablename WHERE last_name LIKE '%bher') ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name in (select name from tablename WHERE first_name LIKE '%aher') or  last_name in (select name from tablename WHERE last_name LIKE '%bher') ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE exists (select name from tablename WHERE first_name LIKE '%aher') and last_name in (select name from tablename WHERE last_name LIKE '%bher') ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE exists (select name from tablename WHERE first_name LIKE '%aher') or  last_name in (select name from tablename WHERE last_name LIKE '%bher') ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name in (select name from tablename WHERE first_name LIKE '%aher') and exists (select name from tablename WHERE last_name LIKE '%bher') ;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM customer WHERE first_name in (select name from tablename WHERE first_name LIKE '%aher') or  exists (select name from tablename WHERE last_name LIKE '%bher') ;`, "%aher", "%bher")

	//subquery in fromclause
	assertCorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her') t1;`)
	assertCorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her%') t1;`)
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher') t1;`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher%') t1;`, "%aher%")

	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her' and last_name LIKE '%aher') t1;`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her' or last_name LIKE '%aher') t1;`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher' and last_name LIKE '%bher') t1;`, "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher' or last_name LIKE '%bher') t1;`, "%aher", "%bher")

	assertCorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her') t1 WHERE first_name LIKE 'her';`)
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher') t1 WHERE first_name LIKE 'her';`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her') t1 WHERE first_name LIKE '%aher';`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher') t1 WHERE first_name LIKE '%bher';`, "%bher", "%aher")

	//subquery in fromclause and whereclause
	assertCorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her') t1 WHERE first_name in (select name from tablename WHERE first_name LIKE 'her');`)
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher') t1 WHERE first_name in (select name from tablename WHERE first_name LIKE 'her');`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her') t1 WHERE first_name in (select name from tablename WHERE first_name LIKE '%aher');`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher') t1 WHERE first_name in (select name from tablename WHERE first_name LIKE '%bher');`, "%bher", "%aher")

	assertCorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her') t1 WHERE exists (select name from tablename WHERE first_name LIKE 'her');`)
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher') t1 WHERE exists (select name from tablename WHERE first_name LIKE 'her');`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE 'her') t1 WHERE exists (select name from tablename WHERE first_name LIKE '%aher');`, "%aher")
	assertIncorrect(`SELECT first_name, last_name FROM (select * from tablename WHERE first_name LIKE '%aher') t1 WHERE exists (select name from tablename WHERE first_name LIKE '%bher');`, "%bher", "%aher")

	//mixture test
	assertCorrect(`SELECT first_name, last_name,(select count(*) from tablename WHERE first_name in (select name from tablename WHERE first_name LIKE 'her') and sex LIKE 'her') FROM (select * from tablename WHERE first_name LIKE 'her') t1 WHERE first_name in (select name from tablename WHERE first_name LIKE 'her');`)
	assertIncorrect(`SELECT first_name, last_name,(select count(*) from tablename WHERE first_name in (select name from tablename WHERE first_name LIKE '%aher') and sex LIKE '%bher') FROM (select * from tablename WHERE first_name LIKE '%cher') t1 WHERE first_name in (select name from tablename WHERE first_name LIKE '%dher');`, "%dher", "%cher", "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name,(select count(*) from tablename WHERE first_name in (select name from tablename WHERE first_name LIKE '%aher') or sex LIKE '%bher') FROM (select * from tablename WHERE first_name LIKE '%cher') t1 WHERE first_name in (select name from tablename WHERE first_name LIKE '%dher');`, "%dher", "%cher", "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name,(select count(*) from tablename WHERE exists (select name from tablename WHERE first_name LIKE '%aher') and sex LIKE '%bher') FROM (select * from tablename WHERE first_name LIKE '%cher') t1 WHERE exists (select name from tablename WHERE first_name LIKE '%dher');`, "%dher", "%cher", "%aher", "%bher")
	assertIncorrect(`SELECT first_name, last_name,(select count(*) from tablename WHERE exists (select name from tablename WHERE first_name LIKE '%aher') or sex LIKE '%bher') FROM (select * from tablename WHERE first_name LIKE '%cher') t1 WHERE exists (select name from tablename WHERE first_name LIKE '%dher');`, "%dher", "%cher", "%aher", "%bher")

	//update test
	assertCorrect(`update tablename set last_name="%jojo" where first_name LIKE 'her';`)
	assertCorrect(`update tablename set last_name="%jojo" where first_name LIKE 'her%';`)
	assertIncorrect(`update tablename set last_name="%jojo" where first_name LIKE '%aher';`, "%aher")
	assertIncorrect(`update tablename set last_name="%jojo" where first_name LIKE '%aher%';`, "%aher%")

	assertCorrect(`update tablename set last_name="%jojo" WHERE first_name LIKE 'her' and last_name LIKE 'her' ;`)
	assertIncorrect(`update tablename set last_name="%jojo" WHERE first_name LIKE 'her' and last_name LIKE '%aher' ;`, "%aher")
	assertIncorrect(`update tablename set last_name="%jojo" WHERE first_name LIKE 'her%' or last_name LIKE '%aher';`, "%aher")
	assertIncorrect(`update tablename set last_name="%jojo" WHERE first_name LIKE '%aher' and last_name LIKE '%bher' ;`, "%aher", "%bher")
	assertIncorrect(`update tablename set last_name="%jojo" WHERE first_name LIKE '%aher' or last_name LIKE '%bher' ;`, "%aher", "%bher")

	//delete from test
	assertCorrect(`delete from  tablename where first_name LIKE 'her';`)
	assertCorrect(`delete from  tablename  where first_name LIKE 'her%';`)
	assertIncorrect(`delete from  tablename where first_name LIKE '%aher';`, "%aher")
	assertIncorrect(`delete from  tablename where first_name LIKE '%aher%';`, "%aher%")

	assertCorrect(`delete from  tablename  WHERE first_name LIKE 'her' and last_name LIKE 'her' ;`)
	assertIncorrect(`delete from  tablename  WHERE first_name LIKE 'her' and last_name LIKE '%aher' ;`, "%aher")
	assertIncorrect(`delete from  tablename  WHERE first_name LIKE 'her%' or last_name LIKE '%aher';`, "%aher")
	assertIncorrect(`delete from  tablename  WHERE first_name LIKE '%aher' and last_name LIKE '%bher' ;`, "%aher", "%bher")
	assertIncorrect(`delete from  tablename  WHERE first_name LIKE '%aher' or last_name LIKE '%bher' ;`, "%aher", "%bher")
}

func TestDriverImpl_Rule17(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{
		UsingType: UsingTypeOnline,
		DatabaseInfo: &DatabaseInfo{
			DatabaseName:  "postgres",
			CurrentSchema: "public",
			SchemaInfoMap: schemaInfoMap,
		},
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId17].Rule
	rule.Params.SetParamValue("expect_number_of_scan_line", "1000")
	assertSqlCorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults()
		testAuditWithEpMockConn(RuleId17, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}
	assertSqlIncorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults().add(RuleId17, "1000")
		testAuditWithEpMockConn(RuleId17, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}

	// test select
	sqlSmt := "select * from t1;"
	mockEp := epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			"Plan": {
			"Node Type": "Seq Scan",
			"Parallel Aware": false,
			"Async Capable": false,
			"Relation Name": "t1",
			"Alias": "t1",
			"Startup Cost": 0.00,
			"Total Cost": 1.11,
			"Plan Rows": 100000,
			"Plan Width": 11
			}
		}]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)
	/*sqlSmt be omitted */
	sqlSmt = "select * from t1"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			"Plan": {
			"Node Type": "Seq Scan",
			"Parallel Aware": false,
			"Async Capable": false,
			"Relation Name": "t1",
			"Alias": "t1",
			"Startup Cost": 0.00,
			"Total Cost": 1.11,
			"Plan Rows": 100,
			"Plan Width": 11
			}
		}]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)

	sqlSmt = "select * from t1 where id in (select id from t2);"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `  [{
			  "Plan": {
				"Node Type": "Hash Join",
				"Parallel Aware": false,
				"Async Capable": false,
				"Join Type": "Semi",
				"Startup Cost": 31896.00,
				"Total Cost": 35806.26,
				"Plan Rows": 11,
				"Plan Width": 11,
				"Inner Unique": false,
				"Hash Cond": "(t1.id = t2.id)",
				"Plans": [
				  {
					"Node Type": "Seq Scan",
					"Parent Relationship": "Outer",
					"Parallel Aware": false,
					"Async Capable": false,
					"Relation Name": "t1",
					"Alias": "t1",
					"Startup Cost": 0.00,
					"Total Cost": 1.11,
					"Plan Rows": 11,
					"Plan Width": 11
				  },
				  {
					"Node Type": "Hash",
					"Parent Relationship": "Inner",
					"Parallel Aware": false,
					"Async Capable": false,
					"Startup Cost": 15489.00,
					"Total Cost": 15489.00,
					"Plan Rows": 10,
					"Plan Width": 4,
					"Plans": [
					  {
						"Node Type": "Seq Scan",
						"Parent Relationship": "Outer",
						"Parallel Aware": false,
						"Async Capable": false,
						"Relation Name": "t2",
						"Alias": "t2",
						"Startup Cost": 0.00,
						"Total Cost": 15489.00,
						"Plan Rows": 100,
						"Plan Width": 4
					  }
					]
				  }
				]
			  }
			}] `,
	}
	assertSqlCorrect(sqlSmt, mockEp)

	sqlSmt = "select * from t1 where id in (select id from t2)"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `  [{
			  "Plan": {
				"Node Type": "Hash Join",
				"Parallel Aware": false,
				"Async Capable": false,
				"Join Type": "Semi",
				"Startup Cost": 31896.00,
				"Total Cost": 35806.26,
				"Plan Rows": 11,
				"Plan Width": 11,
				"Inner Unique": false,
				"Hash Cond": "(t3.id = t4.id)",
				"Plans": [
				  {
					"Node Type": "Seq Scan",
					"Parent Relationship": "Outer",
					"Parallel Aware": false,
					"Async Capable": false,
					"Relation Name": "t3",
					"Alias": "t3",
					"Startup Cost": 0.00,
					"Total Cost": 1.11,
					"Plan Rows": 11,
					"Plan Width": 11
				  },
				  {
					"Node Type": "Hash",
					"Parent Relationship": "Inner",
					"Parallel Aware": false,
					"Async Capable": false,
					"Startup Cost": 15489.00,
					"Total Cost": 15489.00,
					"Plan Rows": 1000000,
					"Plan Width": 4,
					"Plans": [
					  {
						"Node Type": "Seq Scan",
						"Parent Relationship": "Outer",
						"Parallel Aware": false,
						"Async Capable": false,
						"Relation Name": "t4",
						"Alias": "t4",
						"Startup Cost": 0.00,
						"Total Cost": 15489.00,
						"Plan Rows": 1000000,
						"Plan Width": 4
					  }
					]
				  }
				]
			  }
			}] `,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// test insert
	sqlSmt = "insert into t1 values(12,'fa',4);"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			  "Plan": {
				"Node Type": "ModifyTable",
				"Operation": "Insert",
				"Parallel Aware": false,
				"Async Capable": false,
				"Relation Name": "t1",
				"Alias": "t1",
				"Startup Cost": 0.00,
				"Total Cost": 0.01,
				"Plan Rows": 0,
				"Plan Width": 0,
				"Plans": [
				  {
					"Node Type": "Result",
					"Parent Relationship": "Outer",
					"Parallel Aware": false,
					"Async Capable": false,
					"Startup Cost": 0.00,
					"Total Cost": 0.01,
					"Plan Rows": 1,
					"Plan Width": 66
				  }
				]
			  }
			}]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)

	sqlSmt = "insert into t1 values(12,'fa',4)"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			  "Plan": {
				"Node Type": "ModifyTable",
				"Operation": "Insert",
				"Parallel Aware": false,
				"Async Capable": false,
				"Relation Name": "t1",
				"Alias": "t1",
				"Startup Cost": 0.00,
				"Total Cost": 0.01,
				"Plan Rows": 0,
				"Plan Width": 0,
				"Plans": [
				  {
					"Node Type": "Result",
					"Parent Relationship": "Outer",
					"Parallel Aware": false,
					"Async Capable": false,
					"Startup Cost": 0.00,
					"Total Cost": 0.01,
					"Plan Rows": 100000,
					"Plan Width": 66
				  }
				]
			  }
			}]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// test delete
	sqlSmt = "delete from t1 where id=2;"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: ` [{
			  "Plan": {
				"Node Type": "ModifyTable",
				"Operation": "Delete",
				"Parallel Aware": false,
				"Async Capable": false,
				"Relation Name": "t1",
				"Alias": "t1",
				"Startup Cost": 0.00,
				"Total Cost": 1.14,
				"Plan Rows": 0,
				"Plan Width": 0,
				"Plans": [
				  {
					"Node Type": "Seq Scan",
					"Parent Relationship": "Outer",
					"Parallel Aware": false,
					"Async Capable": false,
					"Relation Name": "t1",
					"Alias": "t1",
					"Startup Cost": 0.00,
					"Total Cost": 1.14,
					"Plan Rows": 1,
					"Plan Width": 6,
					"Filter": "(id = 2)"
				  }
				]
			  }
			}]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)

	sqlSmt = "delete from t1 where id=2"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: ` [{
			  "Plan": {
				"Node Type": "ModifyTable",
				"Operation": "Delete",
				"Parallel Aware": false,
				"Async Capable": false,
				"Relation Name": "t1",
				"Alias": "t1",
				"Startup Cost": 0.00,
				"Total Cost": 1.14,
				"Plan Rows": 0,
				"Plan Width": 0,
				"Plans": [
				  {
					"Node Type": "Seq Scan",
					"Parent Relationship": "Outer",
					"Parallel Aware": false,
					"Async Capable": false,
					"Relation Name": "t1",
					"Alias": "t1",
					"Startup Cost": 0.00,
					"Total Cost": 1.14,
					"Plan Rows": 1000000,
					"Plan Width": 6,
					"Filter": "(id = 2)"
				  }
				]
			  }
			}]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// test update
	sqlSmt = "update t1 set id=1 where name='fa';"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			"Plan": {
			  "Node Type": "ModifyTable",
			  "Operation": "Update",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Relation Name": "t1",
			  "Alias": "t1",
			  "Startup Cost": 0.00,
			  "Total Cost": 1.14,
			  "Plan Rows": 0,
			  "Plan Width": 0,
			  "Plans": [
				{
				  "Node Type": "Seq Scan",
				  "Parent Relationship": "Outer",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Relation Name": "t1",
				  "Alias": "t1",
				  "Startup Cost": 0.00,
				  "Total Cost": 1.14,
				  "Plan Rows": 1,
				  "Plan Width": 10,
				  "Filter": "((name)::text = 'fa'::text)"
				}
			  ]
			}
		  }]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)

	sqlSmt = "update t1 set id=1 where name='fa'"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			"Plan": {
			  "Node Type": "ModifyTable",
			  "Operation": "Update",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Relation Name": "t1",
			  "Alias": "t1",
			  "Startup Cost": 0.00,
			  "Total Cost": 1.14,
			  "Plan Rows": 0,
			  "Plan Width": 0,
			  "Plans": [
				{
				  "Node Type": "Seq Scan",
				  "Parent Relationship": "Outer",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Relation Name": "t1",
				  "Alias": "t1",
				  "Startup Cost": 0.00,
				  "Total Cost": 1.14,
				  "Plan Rows": 10000000,
				  "Plan Width": 10,
				  "Filter": "((name)::text = 'fa'::text)"
				}
			  ]
			}
		  }]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)
}

func TestDriverImpl_Rule19(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "a1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "a2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "a3",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "b1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "b2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "c1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "c2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "d1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "d2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId19].Rule

	expectedNestingLayers := rule.Params.GetParam("expected_nesting_layers").Int()
	assertCorrect := func(sql string) {
		correctResult := newTestResults()
		testSingleSqlAudit(RuleId19, t, sql, correctResult, pgContext, nil, nil)
	}

	assertIncorrect := func(sql string, realCount int) {
		incorrectResult := newTestResults().add(RuleId19, expectedNestingLayers, realCount)
		testSingleSqlAudit(RuleId19, t, sql, incorrectResult, pgContext, nil, nil)
	}

	//single subQuery in FROM clause
	assertCorrect(`select * 
	            from (SELECT * FROM (SELECT * FROM B1) B1) T1 
	            where id in (select id from (select bid from a3) t3);`)
	assertIncorrect(`select * 
                from (SELECT * FROM (SELECT * FROM ( SELECT * FROM  C1) C1) B1) T1 
                where id in (select id from (select bid from a3) t3);`, 4)
	assertIncorrect(`select * 
	            from (SELECT * FROM (SELECT * FROM ( SELECT * FROM (SELECT * FROM D1 ) D1) C1) B1) T1 
				where id in (select id from (select bid from a3) t3);`, 5)
	//double subQuery in FROM clause
	assertCorrect(`select * 
	            from (select * from a1) t1, (select * from a2) t2 
				where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`)
	assertIncorrect(`select * 
                from (select * from a1) t1, (SELECT * FROM (SELECT * FROM ( SELECT * FROM C1) C1) B1) T2 
                where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 4)
	assertIncorrect(`select * 
                from (SELECT * FROM (SELECT * FROM ( SELECT * FROM  C1) C1) B1) T2, (select * from a1) t1 
                where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 4)
	assertIncorrect(`select * 
	            from (select * from a1) t1, (SELECT * FROM (SELECT * FROM ( SELECT * FROM (SELECT * FROM D1 ) D1) C1) B1) T2 
				where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 5)
	assertIncorrect(`select * 
	            from (SELECT * FROM (SELECT * FROM ( SELECT * FROM (SELECT * FROM D1 ) D1) C1) B1) T2, (select * from a1) t1 
				where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 5)
	//double subQuery in subQuery's FROM clause
	assertCorrect(`select * 
	            from (select * from a1) t1, (select * from (SELECT * FROM B1 ) B1,(SELECT * FROM B2) B2 WHERE D1.ID=D2.ID ) t2 
	            where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`)
	assertCorrect(`select * 
	            from (select * from (SELECT * FROM B1 ) B1,(SELECT * FROM B2) B2 WHERE D1.ID=D2.ID ) t2, (select * from a1) t1
	            where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`)
	assertIncorrect(`select * 
                from (select * from a1) t1, (select * from ( SELECT * FROM (SELECT * FROM D1 ) D1,(SELECT * FROM D2) D2 WHERE D1.ID=D2.ID) b1) t2 
                where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 4)
	assertIncorrect(`select * 
                from (select * from ( SELECT * FROM (SELECT * FROM D1 ) D1,(SELECT * FROM D2) D2 WHERE D1.ID=D2.ID ) b1) t2,(select * from a1) t1 
                where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 4)
	assertIncorrect(`select * 
	            from (select * from a1) t1, (select * from (select * from ( SELECT * FROM (SELECT * FROM D1 ) D1,(SELECT * FROM D2) D2 WHERE D1.ID=D2.ID ) c1) b1) t2 
				where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 5)
	assertIncorrect(`select * 
	            from (select * from (select * from ( SELECT * FROM (SELECT * FROM D1 ) D1,(SELECT * FROM D2) D2 WHERE D1.ID=D2.ID ) c1) b1) t2,(select * from a1) t1 
				where id in (select id from (select bid from a3) t3) and t1.id=t2.id;`, 5)
}

func TestDriverImpl_Rule21(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t3",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t4",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "col1",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "tb_1",
		OwnerName: "testdb",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "col1",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["testdb"] = &SchemaInfo{
		SchemaName:    "testdb",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// simple sql
	testSingleSqlAudit(RuleId21, t, `select version();`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `select id from testdb.tb_1;`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `select id from testdb.tb_1 for update;`,
		newTestResults().add(RuleId21), pgContext, nil, nil)
	// from clause
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM (SELECT * FROM table1) ss WHERE col1 = 5;`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM (SELECT * FROM table1 FOR UPDATE) ss WHERE col1 = 5;`,
		newTestResults().add(RuleId21), pgContext, nil, nil)

	// where clause
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM t1 ss WHERE id in (select id from t2 where id2=4);`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM t1 ss WHERE id in (select id from t2 where id2=4 for update);`,
		newTestResults().add(RuleId21), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM t1 ss WHERE name = 4 and id in (select id from t2 where id2=4 for update);`,
		newTestResults().add(RuleId21), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM t1 ss WHERE name = 4 and id > 100 or id in (select id from t2 where id2=4 for update);`,
		newTestResults().add(RuleId21), pgContext, nil, nil)

	// union clause
	testSingleSqlAudit(RuleId21, t, `select id from (select id from t2) as aa union all select id from t2 limit 5;`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `select id from (select id from t2 for update) as aa union select id from t2 limit 5;`,
		newTestResults().add(RuleId21), pgContext, nil, nil)

	// join clause
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM t1 INNER JOIN (select * from t2) as aa ON t1.id = aa.id;`, newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM t1 INNER JOIN (select * from t2 for update) as aa ON t1.id = aa.id;`,
		newTestResults().add(RuleId21), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM t1 INNER JOIN (select * from t2 for update) as aa ON t1.id = aa.id for update;`,
		newTestResults().add(RuleId21), pgContext, nil, nil)
	testSingleSqlAudit(RuleId21, t, `SELECT * FROM (select * from t1 for update) as aa INNER JOIN (select * from t2 for update) as bb ON aa.id = bb.id;`,
		newTestResults().add(RuleId21), pgContext, nil, nil)

	testSingleSqlAudit(RuleId21, t,
		`select id
			from (select id from t2 where id=5) as aa
			where id in (select id from t3 where id =3)
				or id in (select id from t4 where id =3)
		union
		select id
			from (select id from t2 where id=5) as bb
			where id in (select id from t3 where id =3);`,
		newTestResults(), pgContext, nil, nil)
}

func testAuditWithSearchPathMockConn(ruleName string, t *testing.T, sqls []string, expectResult []*testResults, mockSearchPaths []string, mockExistSchemas map[string]struct{}, dbOperates *[]*mockDbOperate, pgContext *PgContext) {
	var d *driverImpl
	exe, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		t.Error(err)
		return
	}
	d = newTestDriver(ruleName, "test", "test", exe)

	if len(mockSearchPaths) > 0 {
		mock.ExpectQuery(`SHOW search_path`).
			WillReturnRows(mock.NewRows([]string{"search_path"}).
				AddRow(strings.Join(mockSearchPaths, ",")))
		mock.ExpectQuery(`SELECT quote_literal(user)`).
			WillReturnRows(mock.NewRows([]string{"quote_literal"}).
				AddRow("postgres"))
		// mock存在的schema
		for s := range mockExistSchemas {
			mock.ExpectQuery(fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = %v)", s)).
				WillReturnRows(mock.NewRows([]string{"exist"}).
					AddRow("true"))
		}
		// mock不存在的schema
		for _, s := range mockSearchPaths {
			if _, ok := mockExistSchemas[s]; !ok {
				mock.ExpectQuery(fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = %v)", s)).
					WillReturnRows(mock.NewRows([]string{"exist"}).
						AddRow("false"))
			}
		}
		mock.MatchExpectationsInOrder(false)
	}

	if len(*dbOperates) > 0 {
		// mock db query
		for _, operate := range *dbOperates {
			rows := sqlmock.NewRows(operate.fields)
			for _, row := range operate.mockData {
				values := make([]driver.Value, 0)
				for _, field := range operate.fields {
					value, _ := row[field].Value()
					values = append(values, value)
				}
				rows.AddRow(values...)
			}
			if len(operate.args) > 0 {
				arguments := make([]driver.Value, 0)
				for _, arg := range operate.args {
					arguments = append(arguments, arg)
				}
				mock.ExpectQuery(operate.sql).WithArgs(arguments...).WillReturnRows(rows)
			} else {
				mock.ExpectQuery(operate.sql).WillReturnRows(rows)
			}
		}
	}

	if pgContext == nil {
		database, schema := "test", "test"
		schemaInfoMap := make(map[string]*SchemaInfo)
		tableInfoList := make([]*TableInfo, 0)
		indexInfoList := make([]*IndexInfo, 0)
		indexInfo := &IndexInfo{IndexName: "uniq_name", OwnerName: "test", TableName: "test"}
		indexInfoList = append(indexInfoList, indexInfo)
		schemaInfoMap[schema] = &SchemaInfo{
			SchemaName:    schema,
			TableInfoList: tableInfoList,
			IndexInfoList: indexInfoList,
		}
		databaseInfo := &DatabaseInfo{
			DatabaseName:  "test",
			CurrentSchema: schema,
			SchemaInfoMap: schemaInfoMap,
		}
		d.pgContext = &PgContext{
			CurrentDatabase:      database,
			Executor:             exe,
			DatabaseInfo:         databaseInfo,
			DeletedSchemaMap:     make(map[string]string),
			DeletedTableMap:      make(map[string]string),
			DeletedIndexMap:      make(map[string]string),
			DeletedColumnMap:     make(map[string]string),
			DeletedConstraintMap: make(map[string]string),
		}
	} else {
		pgContext.Executor = exe
	}
	d.pgContext = pgContext

	currentSchema := ""
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: exe, pgContext: d.pgContext}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	results, err := d.Audit(ctx, sqls)
	if err != nil {
		t.Error(err)
		return
	}
	for i, r := range results {
		if r.Level() != expectResult[i].Level() {
			t.Errorf("expect level is %s, actual is %s\n", expectResult[i].Level(), r.Level())
		}
		if r.Message() != expectResult[i].Message() {
			t.Errorf("expect message is %s\n actual is %s\n", expectResult[i].Message(), r.Message())
		}
	}
	return
}

func testAuditWithSpecifiedSchema(ruleName string, t *testing.T, database, schema string, sqls []string, expectResult []*testResults, dbOperates *[]*mockDbOperate, pgContext *PgContext) {
	var d *driverImpl
	var exe *executor.Executor
	var err error
	var mock sqlmock.Sqlmock
	if exe, mock, err = executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})); err != nil {
		t.Error(err)
		return
	}
	d = newTestDriver(ruleName, database, schema, exe)

	if len(*dbOperates) > 0 {
		// mock db query
		for _, operate := range *dbOperates {
			rows := sqlmock.NewRows(operate.fields)
			for _, row := range operate.mockData {
				values := make([]driver.Value, 0)
				for _, field := range operate.fields {
					value, _ := row[field].Value()
					values = append(values, value)
				}
				rows.AddRow(values...)
			}
			if len(operate.args) > 0 {
				arguments := make([]driver.Value, 0)
				for _, arg := range operate.args {
					arguments = append(arguments, arg)
				}
				mock.ExpectQuery(operate.sql).WithArgs(arguments...).WillReturnRows(rows)
			} else {
				mock.ExpectQuery(operate.sql).WillReturnRows(rows)
			}
		}
	}

	if pgContext == nil {
		database, schema := "test", "test"
		schemaInfoMap := make(map[string]*SchemaInfo)
		tableInfoList := make([]*TableInfo, 0)
		indexInfoList := make([]*IndexInfo, 0)
		indexInfo := &IndexInfo{IndexName: "uniq_name", OwnerName: "test", TableName: "test"}
		indexInfoList = append(indexInfoList, indexInfo)
		schemaInfoMap[schema] = &SchemaInfo{
			SchemaName:    schema,
			TableInfoList: tableInfoList,
			IndexInfoList: indexInfoList,
		}
		databaseInfo := &DatabaseInfo{
			DatabaseName:  "test",
			CurrentSchema: schema,
			SchemaInfoMap: schemaInfoMap,
		}
		d.pgContext = &PgContext{
			CurrentDatabase:      database,
			Executor:             exe,
			DatabaseInfo:         databaseInfo,
			DeletedSchemaMap:     make(map[string]string),
			DeletedTableMap:      make(map[string]string),
			DeletedIndexMap:      make(map[string]string),
			DeletedColumnMap:     make(map[string]string),
			DeletedConstraintMap: make(map[string]string),
		}
	} else {
		pgContext.Executor = exe
	}
	d.pgContext = pgContext

	currentSchema := ""
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: exe, pgContext: d.pgContext}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	results, err := d.Audit(ctx, sqls)
	if err != nil {
		t.Error(err)
		return
	}
	for i, r := range results {
		if r.Level() != expectResult[i].Level() {
			t.Errorf("expect level is %s, actual is %s\n", expectResult[i].Level(), r.Level())
		}
		if r.Message() != expectResult[i].Message() {
			t.Errorf("expect message is %s\n actual is %s\n", expectResult[i].Message(), r.Message())
		}
	}
	return
}

func TestDriverImpl_Rule22(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	assertResults := func(sqls []string, results []*testResults, mockSearchPaths []string, mockExistSchemas map[string]struct{}, dbOperates *[]*mockDbOperate) {
		testAuditWithSearchPathMockConn(RuleId22, t, sqls, results, mockSearchPaths, mockExistSchemas, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	// 触发规则
	mockDataTypeName := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "text",
				Valid:  true,
			},
		},
	}

	mockDbOperateObjTypeName := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockDataTypeName,
	}
	dbOperates = append(dbOperates, mockDbOperateObjTypeName)

	mockDataTableName := []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObjTableName := &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockDataTableName,
	}
	dbOperates = append(dbOperates, mockDbOperateObjTableName)
	dbOperates = append(dbOperates, mockDbOperateObjTableName)

	mockDataSchemaName := []map[string]sql.NullString{
		{
			"schema_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
		{
			"schema_name": sql.NullString{
				String: "db1",
				Valid:  true,
			},
		},
		{
			"schema_name": sql.NullString{
				String: "db2",
				Valid:  true,
			},
		},
		{
			"schema_name": sql.NullString{
				String: "db3",
				Valid:  true,
			},
		},
	}
	mockDbOperateObjSchemaName := &mockDbOperate{
		sql:      "select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;",
		fields:   []string{"schema_name"},
		mockData: mockDataSchemaName,
	}
	dbOperates = append(dbOperates, mockDbOperateObjSchemaName)

	// create table
	{
		// 不需要考虑默认schema的情况
		{
			sqls := []string{
				`COMMENT ON TABLE table1_without_comment IS 'test comment'`,
				`CREATE TABLE table1_without_comment (first_column text);`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults().add(RuleId22, "table1_without_comment"),
			}
			t.Run("[create table]it has no effect the comment before creating table", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			dbOperates = make([]*mockDbOperate, 0)
			dbOperates = append(dbOperates, mockDbOperateObjTypeName)
			dbOperates = append(dbOperates, mockDbOperateObjSchemaName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment (first_column text);`,
				`CREATE TABLE table2_with_comment (first_column text);`,
				`COMMENT ON TABLE db1.table1_with_comment IS 'table1_with_comment comment'`,
				`COMMENT ON TABLE table2_with_comment IS 'table2_with_comment comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]all tables have comment", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			dbOperates = make([]*mockDbOperate, 0)
			dbOperates = append(dbOperates, mockDbOperateObjTypeName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment2 (first_column text);`,
				`CREATE TABLE db1.table2_with_comment1 (first_column text);`,
				`COMMENT ON TABLE db1.table1_with_comment2 IS 'table1_with_comment2 comment'`,
				`COMMENT ON TABLE db1.table2_with_comment1 IS 'table2_with_comment1 comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]all tables have comment", func(t *testing.T) {
				testAuditWithSpecifiedSchema(RuleId22, t, "", "db1", sqls, results, &dbOperates, pgContext)
			})
		}
		// 有set search_path语句
		{
			sqls := []string{
				`SET search_path TO db1`,
				`CREATE TABLE db1.table1_with_comment3 (
					   first_column text
				);`,
				`CREATE TABLE table2_without_comment (
					   first_column text
				);`,
				`COMMENT ON TABLE db1.table1_with_comment3 IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults().add(RuleId22, "table2_without_comment"),
				newTestResults(),
			}
			t.Run("[create table]set search_path before create table. one table has no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment4 (
					   first_column text
				);`,
				`CREATE TABLE db1.table2_without_comment1 (
					   first_column text
				);`,
				`SET search_path TO db1`,
				`COMMENT ON TABLE table1_with_comment4 IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults().add(RuleId22, "table2_without_comment1"),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]set search_path after create table. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`SET search_path TO db1`,
				`CREATE TABLE db1.table1_with_comment5 (
					   first_column text
				);`,
				`CREATE TABLE db2.table2_without_comment3 (
					   first_column text
				);`,
				`COMMENT ON TABLE db1.table1_with_comment5 IS 'test comment'`,
				`COMMENT ON TABLE table2_without_comment3 IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults().add(RuleId22, "table2_without_comment3"),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]set search_path and comment one table without specified schema. columns of that table should have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

		}
		// 没有有效的set search_path语句，需要连接实例读取
		{
			sqls := []string{
				`CREATE TABLE db1.table1_with_comment6 (
					   first_column text
				);`,
				`CREATE TABLE table2_without_comment4 (
					   first_column text
				);`,
				`COMMENT ON TABLE db1.table1_with_comment6 IS 'test comment'`,
				`COMMENT ON TABLE db2.table2_without_comment4 IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults().add(RuleId22, "table2_without_comment4"),
				newTestResults(),
				newTestResults(),
			}
			mockSearchPaths := []string{`db2`, `db1`, `db3`}
			mockExistSchemas := map[string]struct{}{
				"db1": {},
				"db3": {},
			}
			t.Run("[create table]get current schema.one table has no comment.", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment7 (
					   first_column text
				);`,
				`CREATE TABLE table2_with_comment3 (
					   first_column text
				);`,
				`COMMENT ON TABLE db1.table1_with_comment7 IS 'table1_with_comment comment'`,
				`COMMENT ON TABLE table2_with_comment3 IS 'table2_with_comment3 comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			mockSearchPaths = []string{`db2`, `db1`, `db3`}
			mockExistSchemas = map[string]struct{}{
				"db1": {},
				"db3": {},
			}

			t.Run("[create table]all tables have comment", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})
		}
	}

	//	//	// todo create table as
	//	//	{
	//	//		sqls := []string{
	//	//			`
	//	//		CREATE TABLE db1.table1_with_comment AS TABLE tb1
	//	//		`,
	//	//			`
	//	//		CREATE TABLE table2_without_comment AS TABLE tb1
	//	//`,
	//	//			`
	//	//		COMMENT ON TABLE table1_with_comment IS 'table1_with_comment comment'
	//	//`,
	//	//		}
	//	//		results := []*testResults{
	//	//			newTestResults(),
	//	//			newTestResults().add(RuleId22, "table2_without_comment"),
	//	//			newTestResults(),
	//	//		}
	//	//		t.Run("[create table as]one table has no comment", func(t *testing.T) {
	//	//			assertResults(sqls, results)
	//	//		})
	//	//
	//	//		sqls = []string{
	//	//			`
	//	//		COMMENT ON TABLE table1_without_comment IS 'table1_without_comment comment'
	//	//`,
	//	//			`
	//	//		CREATE TABLE table1_without_comment AS TABLE tb1
	//	//`,
	//	//		}
	//	//		results = []*testResults{
	//	//			newTestResults(),
	//	//			newTestResults().add(RuleId22, "table1_without_comment"),
	//	//		}
	//	//		t.Run("[create table as]it has no effect the comment before creating table", func(t *testing.T) {
	//	//			assertResults(sqls, results)
	//	//		})
	//	//
	//	//		sqls = []string{
	//	//			`
	//	//		CREATE TABLE table1_with_comment AS TABLE tb1
	//	//		`,
	//	//			`
	//	//		CREATE TABLE table2_with_comment AS TABLE tb1
	//	//`,
	//	//			`
	//	//		COMMENT ON TABLE table1_with_comment IS 'table1_with_comment comment'
	//	//`,
	//	//			`
	//	//		COMMENT ON TABLE table2_with_comment IS 'table2_with_comment comment'
	//	//`,
	//	//		}
	//	//		results = []*testResults{
	//	//			newTestResults(),
	//	//			newTestResults(),
	//	//			newTestResults(),
	//	//			newTestResults(),
	//	//		}
	//	//		t.Run("[create table as]all tables has comment", func(t *testing.T) {
	//	//			assertResults(sqls, results)
	//	//		})
	//	//	}
	//	//
	//	//	// create foreign table
	//	//	{
	//	//		sqls := []string{
	//	//			`
	//	//CREATE FOREIGN TABLE db1.table1_with_comment (
	//	//    code        char(5) NOT NULL,
	//	//    title       varchar(40) NOT NULL
	//	//)
	//	//SERVER film_server;
	//	//		`,
	//	//			`
	//	//CREATE FOREIGN TABLE table2_without_comment
	//	//    PARTITION OF measurement FOR VALUES FROM ('2016-07-01') TO ('2016-08-01')
	//	//    SERVER server_07;
	//	//`,
	//	//			`
	//	//		COMMENT ON TABLE table1_with_comment IS 'table1_with_comment comment'
	//	//`,
	//	//		}
	//	//		results := []*testResults{
	//	//			newTestResults(),
	//	//			newTestResults().add(RuleId22, "table2_without_comment"),
	//	//			newTestResults(),
	//	//		}
	//	//		t.Run("[create foreign table]one table has no comment", func(t *testing.T) {
	//	//			assertResults(sqls, results)
	//	//		})
	//	//
	//	//		sqls = []string{
	//	//			`
	//	//		COMMENT ON TABLE table1_without_comment IS 'table1_without_comment comment'
	//	//`,
	//	//			`
	//	//CREATE FOREIGN TABLE table1_without_comment
	//	//    PARTITION OF measurement FOR VALUES FROM ('2016-07-01') TO ('2016-08-01')
	//	//    SERVER server_07;
	//	//`,
	//	//		}
	//	//		results = []*testResults{
	//	//			newTestResults(),
	//	//			newTestResults().add(RuleId22, "table1_without_comment"),
	//	//		}
	//	//		t.Run("[create foreign table]it has no effect the comment before creating table", func(t *testing.T) {
	//	//			assertResults(sqls, results)
	//	//		})
	//	//
	//	//		sqls = []string{
	//	//			`
	//	//CREATE FOREIGN TABLE table1_with_comment (
	//	//    code        char(5) NOT NULL,
	//	//    title       varchar(40) NOT NULL
	//	//)
	//	//SERVER film_server;
	//	//`,
	//	//			`
	//	//CREATE FOREIGN TABLE table2_with_comment
	//	//    PARTITION OF measurement FOR VALUES FROM ('2016-07-01') TO ('2016-08-01')
	//	//    SERVER server_07;
	//	//`,
	//	//			`
	//	//		COMMENT ON TABLE table1_with_comment IS 'table1_with_comment comment'
	//	//`,
	//	//			`
	//	//		COMMENT ON TABLE table2_with_comment IS 'table1_with_comment comment'
	//	//`,
	//	//		}
	//	//		results = []*testResults{
	//	//			newTestResults(),
	//	//			newTestResults(),
	//	//			newTestResults(),
	//	//			newTestResults(),
	//	//		}
	//	//		t.Run("[create table as]all tables has comment", func(t *testing.T) {
	//	//			assertResults(sqls, results)
	//	//		})
	//	//	}
}

func TestDriverImpl_Rule23(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	assertResults := func(sqls []string, results []*testResults, mockSearchPaths []string, mockExistSchemas map[string]struct{}, dbOperates *[]*mockDbOperate) {
		testAuditWithSearchPathMockConn(RuleId23, t, sqls, results, mockSearchPaths, mockExistSchemas, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	// 触发规则
	mockDataTypeName := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "text",
				Valid:  true,
			},
		},
	}

	mockDbOperateObjTypeName := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockDataTypeName,
	}
	dbOperates = append(dbOperates, mockDbOperateObjTypeName)

	mockDataTableName := []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObjTableName := &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockDataTableName,
	}
	dbOperates = append(dbOperates, mockDbOperateObjTableName)
	dbOperates = append(dbOperates, mockDbOperateObjTableName)

	mockDataSchemaName := []map[string]sql.NullString{
		{
			"schema_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
		{
			"schema_name": sql.NullString{
				String: "db1",
				Valid:  true,
			},
		},
		{
			"schema_name": sql.NullString{
				String: "db2",
				Valid:  true,
			},
		},
		{
			"schema_name": sql.NullString{
				String: "db3",
				Valid:  true,
			},
		},
	}
	mockDbOperateObjSchemaName := &mockDbOperate{
		sql:      "select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;",
		fields:   []string{"schema_name"},
		mockData: mockDataSchemaName,
	}
	dbOperates = append(dbOperates, mockDbOperateObjSchemaName)

	mockDataIndex := []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObjIndex := &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockDataIndex,
	}
	dbOperates = append(dbOperates, mockDbOperateObjIndex)

	// create table
	{
		// 不需要考虑当前schema的情况
		{
			sqls := []string{
				`COMMENT ON COLUMN table1_without_comment.first_column IS 'test comment'`,
				`CREATE TABLE table1_without_comment (first_column text,second_column integer);`,
				`COMMENT ON COLUMN table1_without_comment.second_column IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults().add(RuleId23, "table1_without_comment", "first_column"),
				newTestResults(),
			}
			t.Run("[create table]it has no effect the comment before creating table", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			dbOperates = make([]*mockDbOperate, 0)
			dbOperates = append(dbOperates, mockDbOperateObjTypeName)
			dbOperates = append(dbOperates, mockDbOperateObjSchemaName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment (first_column text,second_column integer);`,
				`CREATE TABLE table2_with_comment (first_column text,second_column integer);`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column IS 'table1_with_comment comment'`,
				`COMMENT ON COLUMN table2_with_comment.first_column IS 'table2_with_comment comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment.second_column IS 'table1_with_comment comment'`,
				`COMMENT ON COLUMN table2_with_comment.second_column IS 'table1_with_comment comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]all columns have comment", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			dbOperates = make([]*mockDbOperate, 0)
			dbOperates = append(dbOperates, mockDbOperateObjTypeName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjIndex)

			sqls = []string{
				`CREATE TABLE if not exists public.sbtest1(
					id character varying(32) NOT NULL DEFAULT sys_guid(),
					name character varying(100) NOT NULL,
					CONSTRAINT user_pkey PRIMARY KEY (id)
				)with (oids = false);`,
				`COMMENT ON TABLE public.sbtest1 IS '用户表';`,
				`COMMENT ON COLUMN public.sbtest1.id IS '主键';`,
				`COMMENT ON COLUMN public.sbtest1.name IS '姓名';`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table] with constraint column, all columns have comment", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			dbOperates = make([]*mockDbOperate, 0)
			dbOperates = append(dbOperates, mockDbOperateObjTypeName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)
			dbOperates = append(dbOperates, mockDbOperateObjTableName)

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment1 (first_column text,second_column integer);`,
				`CREATE TABLE db1.table2_with_comment (first_column text,second_column integer);`,
				`COMMENT ON COLUMN db1.table1_with_comment1.first_column IS 'table1_with_comment1 comment'`,
				`COMMENT ON COLUMN db1.table2_with_comment.first_column IS 'table2_with_comment comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment1.second_column IS 'table1_with_comment1 comment'`,
				`COMMENT ON COLUMN db1.table2_with_comment.second_column IS 'table2_with_comment comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]all columns have comment", func(t *testing.T) {
				testAuditWithSpecifiedSchema(RuleId23, t, "", "db1", sqls, results, &dbOperates, pgContext)
			})
		}
		// 有set search_path语句
		{
			sqls := []string{
				`SET search_path TO db1`,
				`CREATE TABLE db1.table1_with_comment2 (first_column text,second_column integer);`,
				`CREATE TABLE db3.table2_without_comment (first_column text,second_column integer);`,
				`COMMENT ON COLUMN db1.table1_with_comment2.first_column IS 'test comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment2.second_column IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column, second_column"),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]set search_path before create table. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment3 (first_column text,second_column integer);`,
				`CREATE TABLE db1.table2_without_comment (first_column text,second_column integer);`,
				`SET search_path TO db1`,
				`COMMENT ON COLUMN db1.table1_with_comment3.first_column IS 'test comment'`,
				`COMMENT ON COLUMN table1_with_comment3.second_column IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column, second_column"),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]set search_path after create table. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`SET search_path TO db1`,
				`CREATE TABLE db1.table1_with_comment4 (first_column text,second_column integer);`,
				`CREATE TABLE db2.table2_without_comment (first_column text,second_column integer);`,
				`COMMENT ON COLUMN db1.table1_with_comment4.first_column IS 'test comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment4.second_column IS 'test comment'`,
				`COMMENT ON COLUMN table2_without_comment.first_column IS 'test comment'`,
				`COMMENT ON COLUMN table2_without_comment.second_column IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column, second_column"),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[create table]set search_path and comment one table without specified schema. columns of that table should have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

		}
		// 没有有效的set search_path语句，需要连接实例读取
		{
			sqls := []string{
				`CREATE TABLE db1.table1_with_comment5 (first_column text,second_column integer);`,
				`CREATE TABLE table2_without_comment1 (first_column text,second_column integer);`,
				`COMMENT ON COLUMN db1.table1_with_comment5.first_column IS 'test comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment5.second_column IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment1", "first_column, second_column"),
				newTestResults(),
				newTestResults(),
			}
			mockSearchPaths := []string{`db2`, `db1`, `db3`}
			mockExistSchemas := map[string]struct{}{
				"db1": {},
				"db3": {},
			}
			t.Run("[create table]get current schema. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})

			sqls = []string{
				`CREATE TABLE db1.table1_with_comment6 (first_column text,second_column integer);`,
				`CREATE TABLE table2_without_comment3 (first_column text,second_column integer);`,
				`COMMENT ON COLUMN db1.table1_with_comment6.first_column IS 'test comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment6.second_column IS 'test comment'`,
				`COMMENT ON COLUMN db2.table2_without_comment3.first_column IS 'test comment'`,
				`COMMENT ON COLUMN db2.table2_without_comment3.second_column IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment3", "first_column, second_column"),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			mockSearchPaths = []string{`db2`, `db1`, `db3`}
			mockExistSchemas = map[string]struct{}{
				"db1": {},
				"db3": {},
			}
			t.Run("[create table]get current schema. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})
			sqls = []string{
				`CREATE TABLE db1.table1_with_comment7 (first_column text,second_column integer);`,
				`CREATE TABLE table2_with_comment1 (first_column text,second_column integer);`,
				`COMMENT ON COLUMN db1.table1_with_comment7.first_column IS 'table1_with_comment7 comment'`,
				`COMMENT ON COLUMN table2_with_comment1.first_column IS 'table2_with_comment1 comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment7.second_column IS 'table1_with_comment7 comment'`,
				`COMMENT ON COLUMN table2_with_comment1.second_column IS 'table2_with_comment1 comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			mockSearchPaths = []string{`db2`, `db1`, `db3`}
			mockExistSchemas = map[string]struct{}{
				"db1": {},
				"db3": {},
			}

			t.Run("[create table]all columns have comment", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})
		}
	}
	// add column
	{
		// 不需要考虑当前schema的情况
		{
			sqls := []string{
				`ALTER TABLE public.table1_without_comment ADD COLUMN first_column1 int;`,
				`COMMENT ON COLUMN table1_without_comment.first_column IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults().add(RuleId23, "table1_without_comment", "first_column1"),
				newTestResults(),
			}
			t.Run("[add column]it has no effect the comment before adding column", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`ALTER TABLE db1.table1_with_comment ADD COLUMN first_column1 int;`,
				`ALTER TABLE db1.table1_with_comment ADD COLUMN second_column1 int;`,
				`ALTER TABLE table2_with_comment ADD COLUMN first_column1 int;`,
				`ALTER TABLE table2_with_comment ADD COLUMN second_column1 int;`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column1 IS 'table1_with_comment comment'`,
				`COMMENT ON COLUMN table2_with_comment.first_column1 IS 'table2_with_comment comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment.second_column1 IS 'table1_with_comment comment'`,
				`COMMENT ON COLUMN table2_with_comment.second_column1 IS 'table1_with_comment comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[add column]all columns have comment", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`ALTER TABLE db1.table1_with_comment ADD COLUMN first_column11 int;`,
				`ALTER TABLE db1.table1_with_comment ADD COLUMN second_column11 int;`,
				`ALTER TABLE db1.table2_with_comment ADD COLUMN first_column2 int;`,
				`ALTER TABLE db1.table2_with_comment ADD COLUMN second_column2 int;`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column11 IS 'table1_with_comment comment'`,
				`COMMENT ON COLUMN db1.table2_with_comment.first_column2 IS 'table2_with_comment comment'`,
				`COMMENT ON COLUMN db1.table1_with_comment.second_column11 IS 'table1_with_comment comment'`,
				`COMMENT ON COLUMN db1.table2_with_comment.second_column2 IS 'table1_with_comment comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[add column]all columns have comment", func(t *testing.T) {
				testAuditWithSpecifiedSchema(RuleId23, t, "", "db1", sqls, results, &dbOperates, pgContext)
			})
		}
		// 有set search_path语句
		{
			sqls := []string{
				`SET search_path TO db1`,
				`ALTER TABLE db1.table1_with_comment ADD COLUMN first_column2 int`,
				`ALTER TABLE table2_without_comment ADD COLUMN first_column2 int`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column2 IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column2"),
				newTestResults(),
			}
			t.Run("[add column]set search_path before add column. column of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`ALTER TABLE db1.table1_with_comment ADD COLUMN first_column3 int`,
				`ALTER TABLE db1.table2_without_comment ADD COLUMN first_column3 int`,
				`SET search_path TO db1`,
				`COMMENT ON COLUMN table1_with_comment.first_column3 IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column3"),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[add column]set search_path after add column. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})

			sqls = []string{
				`SET search_path TO db1`,
				`ALTER TABLE db1.table1_with_comment ADD COLUMN first_column4 int;`,
				`ALTER TABLE db2.table2_without_comment ADD COLUMN first_column4 int;`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column4 IS 'test comment'`,
				`COMMENT ON COLUMN table2_without_comment.first_column4 IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column4"),
				newTestResults(),
				newTestResults(),
			}
			t.Run("[alter table]set search_path and comment one table without specified schema. columns of that table should have no comment.", func(t *testing.T) {
				assertResults(sqls, results, nil, nil, &dbOperates)
			})
		}
		// 没有有效的set search_path语句，需要连接实例读取
		{
			sqls := []string{
				`ALTER TABLE db1.table1_with_comment ADD COLUMN first_column5 int;`,
				`ALTER TABLE table2_without_comment ADD COLUMN first_column5 int;`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column5 IS 'test comment'`,
			}
			results := []*testResults{
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column5"),
				newTestResults(),
			}
			mockSearchPaths := []string{`db2`, `db1`, `db3`}
			mockExistSchemas := map[string]struct{}{
				"db1": {},
				"db3": {},
			}
			t.Run("[add column]get current schema. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})

			sqls = []string{
				`ALTER TABLE db1.table1_with_comment ADD COLUMN first_column6 int;`,
				`ALTER TABLE table2_without_comment ADD COLUMN first_column6 int;`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column6 IS 'test comment'`,
				`COMMENT ON COLUMN table2_without_comment.first_column IS 'test comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults().add(RuleId23, "table2_without_comment", "first_column6"),
				newTestResults(),
				newTestResults(),
			}
			mockSearchPaths = []string{`db2`, `db1`, `db3`}
			mockExistSchemas = map[string]struct{}{
				"db1": {},
				"db3": {},
			}
			t.Run("[add column]get current schema. columns of one table have no comment.", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})

			sqls = []string{
				`ALTER TABLE db1.table1_with_comment  ADD COLUMN first_column7 int;`,
				`ALTER TABLE table2_with_comment ADD COLUMN first_column7 int;`,
				`COMMENT ON COLUMN db1.table1_with_comment.first_column7 IS 'table1_with_comment comment'`,
				`COMMENT ON COLUMN table2_with_comment.first_column7 IS 'table1_with_comment comment'`,
			}
			results = []*testResults{
				newTestResults(),
				newTestResults(),
				newTestResults(),
				newTestResults(),
			}
			mockSearchPaths = []string{`db2`, `db1`, `db3`}
			mockExistSchemas = map[string]struct{}{
				"db1": {},
				"db3": {},
			}

			t.Run("[add column]all columns have comment", func(t *testing.T) {
				assertResults(sqls, results, mockSearchPaths, mockExistSchemas, &dbOperates)
			})
		}
	}
}

func TestDriverImpl_Rule26(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "products",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "price",
			},
			{
				ColumnName: "product_no",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId26)
		testAuditWithDbQueryMockConn(RuleId26, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId26, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "numeric",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "text",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	// 走规则
	assertSqlIncorrect("ALTER TABLE products ALTER COLUMN price TYPE numeric(10,2);", &dbOperates)
	assertSqlIncorrect("ALTER TABLE products ALTER COLUMN price TYPE numeric(10,2)", &dbOperates)

	// 不走规则
	assertSqlCorrect("ALTER TABLE products ADD COLUMN description text;", &dbOperates)
	assertSqlCorrect("ALTER TABLE products DROP COLUMN description;", &dbOperates)
	assertSqlCorrect("ALTER TABLE products ALTER COLUMN price SET DEFAULT 7.77;", &dbOperates)

	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "numeric",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "text",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlCorrect("ALTER TABLE products RENAME COLUMN product_no TO product_number;", &dbOperates)
	assertSqlCorrect("ALTER TABLE products RENAME TO items;", &dbOperates)
}

func TestDriverImpl_Rule27(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t1",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "t1",
			},
			{
				ColumnName: "name",
				TableName:  "t1",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t2",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "t2",
			},
			{
				ColumnName: "name",
				TableName:  "t2",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId27, t, "insert into t1 values(1, 'a');", newTestResults().add(RuleId27), pgContext, nil, nil)
	testSingleSqlAudit(RuleId27, t, "insert into t1 values(1, 'a'), (2, 'b');", newTestResults().add(RuleId27), pgContext, nil, nil)

	testSingleSqlAudit(RuleId27, t, "insert into t1(id, name) values(1, 'a');", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId27, t, "insert into t1(id, name) values(1, 'a'), (2, 'b');", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId27, t, "insert into t1 select id, name from t2", newTestResults().add(RuleId27), pgContext, nil, nil)
	testSingleSqlAudit(RuleId27, t, "insert into t1(id, name) select id, name from t2", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule28(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table1",
			},
			{
				ColumnName: "name",
				TableName:  "table1",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table2",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table2",
			},
			{
				ColumnName: "name",
				TableName:  "table2",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{
		UsingType: UsingTypeOnline,
		DatabaseInfo: &DatabaseInfo{
			DatabaseName:  "postgres",
			CurrentSchema: "public",
			SchemaInfoMap: schemaInfoMap,
		},
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	assertSqlIncorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults().add(RuleId28)
		testAuditWithEpMockConn(RuleId28, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}
	assertSqlCorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults()
		testAuditWithEpMockConn(RuleId28, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}

	// Incorrect
	sqlSmt := "select t1.*,t2.* from table1 t1,table2 t2;"
	mockEp := epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
				"Plan": {
				  "Node Type": "Nested Loop",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Join Type": "Inner",
				  "Startup Cost": 0.00,
				  "Total Cost": 1307.20,
				  "Plan Rows": 102400,
				  "Plan Width": 444,
				  "Inner Unique": false,
				  "Plans": [
					{
					  "Node Type": "Seq Scan",
					  "Parent Relationship": "Outer",
					  "Parallel Aware": false,
					  "Async Capable": false,
					  "Relation Name": "table1",
					  "Alias": "t1",
					  "Startup Cost": 0.00,
					  "Total Cost": 13.20,
					  "Plan Rows": 320,
					  "Plan Width": 222
					},
					{
					  "Node Type": "Materialize",
					  "Parent Relationship": "Inner",
					  "Parallel Aware": false,
					  "Async Capable": false,
					  "Startup Cost": 0.00,
					  "Total Cost": 14.80,
					  "Plan Rows": 320,
					  "Plan Width": 222,
					  "Plans": [
						{
						  "Node Type": "Seq Scan",
						  "Parent Relationship": "Outer",
						  "Parallel Aware": false,
						  "Async Capable": false,
						  "Relation Name": "table2",
						  "Alias": "t2",
						  "Startup Cost": 0.00,
						  "Total Cost": 13.20,
						  "Plan Rows": 320,
						  "Plan Width": 222
						}
					  ]
					}
				  ]
				}
			  }]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// Correct
	sqlSmt = "select t1.*,t2.* from table1 t1,table2 t2 where t1.id = t2.id;"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
				"Plan": {
				  "Node Type": "Hash Join",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Join Type": "Inner",
				  "Startup Cost": 17.20,
				  "Total Cost": 49.12,
				  "Plan Rows": 512,
				  "Plan Width": 444,
				  "Inner Unique": false,
				  "Hash Cond": "(t1.id = t2.id)",
				  "Plans": [
					{
					  "Node Type": "Seq Scan",
					  "Parent Relationship": "Outer",
					  "Parallel Aware": false,
					  "Async Capable": false,
					  "Relation Name": "table1",
					  "Alias": "t1",
					  "Startup Cost": 0.00,
					  "Total Cost": 13.20,
					  "Plan Rows": 320,
					  "Plan Width": 222
					},
					{
					  "Node Type": "Hash",
					  "Parent Relationship": "Inner",
					  "Parallel Aware": false,
					  "Async Capable": false,
					  "Startup Cost": 13.20,
					  "Total Cost": 13.20,
					  "Plan Rows": 320,
					  "Plan Width": 222,
					  "Plans": [
						{
						  "Node Type": "Seq Scan",
						  "Parent Relationship": "Outer",
						  "Parallel Aware": false,
						  "Async Capable": false,
						  "Relation Name": "table2",
						  "Alias": "t2",
						  "Startup Cost": 0.00,
						  "Total Cost": 13.20,
						  "Plan Rows": 320,
						  "Plan Width": 222
						}
					  ]
					}
				  ]
				}
			  }]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)
}

func TestDriverImpl_Rule31(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId31)
		testAuditWithDbQueryMockConn(RuleId31, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId31, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	// 触发规则
	mockData := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"schema_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;",
		fields:   []string{"schema_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrect("create table test.testIndex1(id int, column1 int unique, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex1 unique(column1));", &dbOperates)
	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
	}

	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"schema_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;",
		fields:   []string{"schema_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlCorrect("create table test.testIndex2(id int, column1 int, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex1 unique(column1));", &dbOperates)

	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
	}

	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"schema_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;",
		fields:   []string{"schema_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrect("create table test.testIndex3(id int, column1 int unique, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex1 unique(column1,column2));", &dbOperates)
	assertSqlCorrect("create table test.testIndex4(id int, column1 int, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex1 unique(column1,column2));", &dbOperates)

	assertSqlIncorrect("create table test.testIndex5(id int, column1 int, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex1 unique(column1), CONSTRAINT uni_testIndex2 unique(column1,column2));", &dbOperates)
	assertSqlCorrect("create table test.testIndex6(id int, column1 int, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex2 unique(column1,column2));", &dbOperates)

	assertSqlIncorrect("create table test.testIndex7(id int, column1 int, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex1 unique(column1,column2), CONSTRAINT uni_testIndex2 unique(column1));", &dbOperates)
	assertSqlCorrect("create table test.testIndex8(id int, column1 int, column2 varchar(100), column3 DATE, CONSTRAINT uni_testIndex1 unique(column1,column2));", &dbOperates)

	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
	}

	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"schema_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select schema_name from information_schema.schemata where catalog_name = $1 and schema_name not like $2 and schema_name != $3;",
		fields:   []string{"schema_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrect("create index idx_testIndex2_1 on test.testindex1(column1, column2);", &dbOperates)
	assertSqlCorrect("create index idx_testIndex2_2 on test.testindex1(column2);", &dbOperates)

	assertSqlIncorrect("create unique index idx_testIndex2_3 on test.testIndex2(column1, column2);", &dbOperates)
	assertSqlCorrect("create unique index idx_testIndex2_4 on test.testIndex2(column2);", &dbOperates)

	assertSqlIncorrect("alter table test.testIndex1 add constraint idx_testIndex3_1 unique (column1, column2);", &dbOperates)
	assertSqlCorrect("alter table test.testIndex1 drop constraint idx_testIndex3_1;alter table test.testIndex1 add constraint idx_testIndex3_2 unique (column2);", &dbOperates)
}

func TestDriverImpl_Rule32(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "temporary_test",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "temporary_test",
				OwnerName:  "public",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test1",
				OwnerName:  "test",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test2",
				OwnerName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test2",
				OwnerName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test2",
				OwnerName:  "test",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId32, t, "select id,name,age from test.test1;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "select id,name,age from test.test1 where id = id and 1 = 1;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "select id,name,age from test.test1 where id > 0 or 1 = 1;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "select id,name,age from test.test1 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId32, t, "select id,name,age from test.test1 where id in (select id from test.test2 where 1=1 and id = id);", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "select id,name,age from test.test1 where id in (select id from test.test2 where 1=1 or id > 0);", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "select id,name,age from test.test1 where id in (select id from test.test2 where id > 0);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId32, t, "insert into test.test1 select id,name,age from test.test2;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "insert into test.test1 select id,name,age from test.test2 where 1 = 1 and id = id;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "insert into test.test1 select id,name,age from test.test2 where 1 = 1 or id > 0;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "insert into test.test1 select id,name,age from test.test2 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId32, t, "update test.test1 set name = 'test';", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "update test.test1 set name = 'test' where 1 = 1 and id = id;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "update test.test1 set name = 'test' where 1 = 1 or id > 0;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "update test.test1 set name = 'test' where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId32, t, "delete from test.test1;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "delete from test.test1 where 1 = 1 and id = id;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "delete from test.test1 where 1 = 1 or id > 0;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "delete from test.test1 where id > 0", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2) insert into test.test1 (id, name, age) select id, name, age from temporary_test where id > 0;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2 where 1=1 or id > 0) insert into test.test1 (id, name, age) select id, name, age from temporary_test where id > 0;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2 where id > 0) insert into test.test1 (id, name, age) select id, name, age from temporary_test where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2) update test.test1 set name = temporary_test.name, age = temporary_test.age from temporary_test where test.test1.id = temporary_test.id;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2 where id > 0 or 1=1) update test.test1 set name = temporary_test.name, age = temporary_test.age from temporary_test where test.test1.id = temporary_test.id;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2 where id > 0) update test.test1 set name = temporary_test.name, age = temporary_test.age from temporary_test where test.test1.id = temporary_test.id;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2) delete from test.test1 using temporary_test where test.test1.id = temporary_test.id;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2 where id > 0 or 1=1) delete from test.test1 using temporary_test where test.test1.id = temporary_test.id;", newTestResults().add(RuleId32), pgContext, nil, nil)
	testSingleSqlAudit(RuleId32, t, "with temporary_test as (select * from test.test2 where id > 0) delete from test.test1 using temporary_test where test.test1.id = temporary_test.id;", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule33(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test1",
				OwnerName:  "test",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test2",
				OwnerName:  "test",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId33, t, "select name, (select id from test1 limit 1) as nameCount from test2;", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select name, (select id from test1 offset 10 fetch first 1 ROWS ONLY) as nameCount from test2;", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select name, (select 10 from test1) as nameCount from test2;", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select name, (select 'china' from test1) as country from test2;", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select name, (select count(*) as num from test1) as nameCount from test2;", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select name, (select count(id) from test1) as nameCount from test2;", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select name, (select count(10) as num from test1) as nameCount from test2;", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select name from test2;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId33, t, "select * from test1 where age > (select avg(age) from test2);", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select * from test1 where age > 1;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId33, t, "select * from test1 where id in (select max(id) from test2);", newTestResults().add(RuleId33), pgContext, nil, nil)
	testSingleSqlAudit(RuleId33, t, "select * from test1 where id in (select id from test2);", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule34(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test1",
				OwnerName:  "test",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test2",
				OwnerName:  "test",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId34, t, "select id,name,age from test.test1 where name like 'zhang%' or id > 0;", newTestResults().add(RuleId34), pgContext, nil, nil)
	testSingleSqlAudit(RuleId34, t, "select t.id,t.name,t.age from test.test1 t where t.name like 'zhang%' or t.id > 0;", newTestResults().add(RuleId34), pgContext, nil, nil)
	testSingleSqlAudit(RuleId34, t, "select id,name,age from test.test1 where name like 'zhang%' union select id,name,age from test.test1 where name like 'li%';", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId34, t, "select id,(select name from test.test1 where name like 'zhang%' or id > 0),age from test.test2 where id > 0;", newTestResults().add(RuleId34), pgContext, nil, nil)
	testSingleSqlAudit(RuleId34, t, "select id,(select name from test.test1 where name like 'zhang%' union select name from test.test1 where name like 'li%'),age from test.test2 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId34, t, "select t.id,t.name,t.age from (select id,name,age from test.test1 where name like 'zhang%' or id > 0) t where t.id > 0;", newTestResults().add(RuleId34), pgContext, nil, nil)
	testSingleSqlAudit(RuleId34, t, "select t.id,t.name,t.age from (select id,name,age from test.test1 where name like 'zhang%' union select id,name,age from test.test1 where name like 'li%') t where t.id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId34, t, "select id,name,age from test.test1 where id in(select id from test.test2 where name like 'zhang%' or id > 0);", newTestResults().add(RuleId34), pgContext, nil, nil)
	testSingleSqlAudit(RuleId34, t, "select id,name,age from test.test1 where id in(select id from test.test2 where name like 'zhang%' and id > 0)", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule35(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
				OwnerName:  "public",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test1",
				OwnerName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test1",
				OwnerName:  "test",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test2",
				OwnerName:  "test",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test3",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test3",
				OwnerName:  "test",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId35, t, "select * from test.test right join test.test1 on test.id = test1.id left join test2 on test.id = test2.id right join test.test3 on test.id = test3.id;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "select * from test.test left join test1 on test.id = test1.id;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "select * from test.test left join test.test1 on test.id = test1.id;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "select * from test.test left join test1 on test.test.id = test1.id left join test2 on test.id = test2.id;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "select * from test.test left join test.test1 on test.id = test1.id left join test.test2 on test.id = test2.id;;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "select * from test.test right join test1 on test.id = test1.id left join test2 on test.id = test2.id;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "select * from test.test right join test.test1 on test.id = test1.id left join test.test2 on test.id = test2.id;;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "select * from test.test right join test.test1 on test.id = test1.id left join test2 on test.id = test2.id;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "select * from test.test right join test.test1 on test.id = test1.id left join test.test2 on test.id = test2.id;;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "select * from test;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "select * from test.test;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "update test set id = 0;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "update test.test set id = 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "update test.test set id = 0 where id in (select id from test1);", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "update test.test set id = 0 where id in (select id from test.test1);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "update test.test set id = (select max(id) from test1) where id = 0;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "update test.test set id = (select max(id) from test.test1) where id = 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "delete from test where id = 0;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "delete from test.test where id = 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "insert into test select * from test.test;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "insert into test.test select * from test.test;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create table test11(id int,name varchar(100));", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create table test.test11(id int,name varchar(100));", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create table test_alias as select * from test;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create table test.test_alias as select * from test.test;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create view test.view_test as select id from test where id > 0;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create view test.view_test as select id from test.test where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create view test.view_test as select x.id from test.test x left join test.test1 y on x.id = y.id left join test3 z on y.id = z.id where id > 0;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create view test.view_test as select x.id from test.test x left join test.test1 y on x.id = y.id left join test.test3 z on y.id = z.id where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create index idx_test_name on test(name);", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create index idx_test_name on test.test(name);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create sequence sequence_test start with 1 increment by 1 minvalue 1 maxvalue 999999999 cache 2;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create sequence test.sequence_test start with 1 increment by 1 minvalue 1 maxvalue 999999999 cache 2;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create or replace function log_test_changes() returns trigger as $$ begin insert into test (id,name) values (1,'aaa');end;$$ language plpgsql;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create or replace function test.log_test_changes() returns trigger as $$ begin insert into test (id,name) values (1,'aaa');end;$$ language plpgsql;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create trigger trigger_test after insert or update on test for each row execute function log_test_changes();", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create trigger trigger_test after insert or update on test.test for each row execute function log_test_changes();", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "create or replace procedure audit_test_changes(\"id\" int,\"name\" varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(\"id\",\"name\");END;$$", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "create or replace procedure test.audit_test_changes(\"id\" int,\"name\" varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(\"id\",\"name\");END;$$", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "alter table test add column age1 int;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "alter table test.test add column age11 int;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "alter sequence sequence_test restart 1 increment by 1;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "alter sequence test.sequence_test restart 1 increment by 1;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "alter trigger trigger_test on test rename to new_trigger_name;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "alter trigger trigger_test on test.test rename to new_trigger_name;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "drop view view_test;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "drop view test.view_test;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "drop index idx_test_name;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "drop index test.idx_test_name;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "drop table test11;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "drop table test.test11;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "drop sequence sequence_test;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "drop sequence test.sequence_test;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "drop function if exists log_test_changes;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "drop function if exists test.log_test_changes;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "drop trigger trigger_test on test;", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "drop trigger trigger_test on test.test;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId35, t, "drop procedure if exists audit_test_changes(int,varchar(100));", newTestResults().add(RuleId35), pgContext, nil, nil)
	testSingleSqlAudit(RuleId35, t, "drop procedure if exists test.audit_test_changes(int,varchar(100));", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule36(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table_name",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table_name",
				OwnerName:  "public",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table_name1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table_name1",
				OwnerName:  "public",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
		IndexInfoList: []*IndexInfo{
			{
				IndexName: "index_name",
				OwnerName: "public",
				TableName: "table_name1",
			},
		},
	}
	schemaInfoMap["schema_name"] = &SchemaInfo{
		SchemaName:    "schema_name",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId36, t, "DROP TABLE table_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), []string{"table_name"})
	testSingleSqlAudit(RuleId36, t, "DROP TABLESPACE tablespace_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
	testSingleSqlAudit(RuleId36, t, "DROP VIEW view_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
	testSingleSqlAudit(RuleId36, t, "DROP INDEX index_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), []string{"table_name1"})
	testSingleSqlAudit(RuleId36, t, "DROP FUNCTION get_employee_details(integer);", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
	testSingleSqlAudit(RuleId36, t, "DROP TRIGGER trigger_name ON table_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
	testSingleSqlAudit(RuleId36, t, "DROP SCHEMA schema_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
	testSingleSqlAudit(RuleId36, t, "DROP DATABASE database_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
	testSingleSqlAudit(RuleId36, t, "DROP SEQUENCE sequence_name;", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
	testSingleSqlAudit(RuleId36, t, "DROP PROCEDURE get_employee_details(integer);", newTestResults().add(RuleId36), pgContext, make([]string, 0), make([]string, 0))
}

func TestDriverImpl_Rule37(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table1",
				OwnerName:  "public",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId37, t, "ALTER TABLE table1 DROP COLUMN id;", newTestResults().add(RuleId37), pgContext, nil, nil)
}

func TestDriverImpl_Rule38(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}}
	testSingleSqlAudit(RuleId38, t, "SELECT id,name from test1 union SELECT id,name from test2;", newTestResults().add(RuleId38), pgContext, nil, nil)
	testSingleSqlAudit(RuleId38, t, "SELECT id,name from test1 union all SELECT id,name from test2;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId38, t, "SELECT id,name from test1 union SELECT id,name from test2 union SELECT id,name from test3;", newTestResults().add(RuleId38), pgContext, nil, nil)
	testSingleSqlAudit(RuleId38, t, "SELECT id,name from test1 union all SELECT id,name from test2 union all SELECT id,name from test3;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId38, t, "SELECT id,name from test1 union all SELECT id,name from test2 union SELECT id,name from test3 union SELECT id,name from test4;", newTestResults().add(RuleId38), pgContext, nil, nil)
	testSingleSqlAudit(RuleId38, t, "SELECT id,name from test1 union all SELECT id,name from test2 union all SELECT id,name from test3 union all SELECT id,name from test4;", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule39(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test11",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "col1",
			},
			{
				ColumnName: "col2",
			},
			{
				ColumnName: "col3",
			},
			{
				ColumnName: "col4",
			},
			{
				ColumnName: "col5",
			},
			{
				ColumnName: "col6",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId39].Rule
	rule.Params.SetParamValue("expect_max_index_column_number", "5")
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId39, 5)
		testAuditWithDbQueryMockConn(RuleId39, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId39, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}

	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlCorrect("create table test1(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test unique (col1,col2,col3,col4,col5));", &dbOperates)
	assertSqlIncorrect("create table test2(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test unique (col1,col2,col3,col4,col5,col6));", &dbOperates)

	assertSqlCorrect("create table test3(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint pk_test PRIMARY KEY (col1,col2,col3,col4,col5));", &dbOperates)
	assertSqlIncorrect("create table test4(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint pk_test PRIMARY KEY (col1,col2,col3,col4,col5,col6));", &dbOperates)

	assertSqlCorrect(`create table test5(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test1 unique(col1,col2,col3),constraint constraint_test2 unique(col4,col5));`, &dbOperates)
	assertSqlIncorrect(`create table test6(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test1 unique(col1,col2,col3),constraint constraint_test2 unique(col4,col5,col6));`, &dbOperates)

	assertSqlCorrect(`create table test7(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test1 unique(col1,col2,col3),constraint constraint_test2 PRIMARY KEY(col4,col5));`, &dbOperates)
	assertSqlIncorrect(`create table test8(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test1 unique(col1,col2,col3),constraint constraint_test2 PRIMARY KEY(col4,col5,col6));`, &dbOperates)

	assertSqlCorrect(`alter table test1 add constraint idx_constraint_test PRIMARY KEY(col1,col2,col3,col4);`, &dbOperates)
	assertSqlIncorrect(`alter table test2 add constraint idx_constraint_test PRIMARY KEY(col1,col2,col3,col4,col5,col6);`, &dbOperates)

	assertSqlCorrect(`alter table test3 add constraint idx_constraint_test unique(col1,col2,col3,col4);`, &dbOperates)
	assertSqlIncorrect(`alter table test4 add constraint idx_constraint_test unique(col1,col2,col3,col4,col5,col6);`, &dbOperates)

	assertSqlCorrect(`create index idx_index_name1 on test.test11(col1,col2,col3,col4);`, &dbOperates)
	assertSqlIncorrect(`create index idx_index_name2 on test.test11(col1,col2,col3,col4,col5,col6);`, &dbOperates)
	assertSqlIncorrect(`create index idx_index_name3 on test.test11(col1,col2,col3,col4,col5);`, &dbOperates)
}

func TestDriverImpl_Rule40(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "name",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	// 走规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId40)
		testAuditWithDbQueryMockConn(RuleId40, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId40, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDataDefaultValue := []map[string]sql.NullString{
		{
			"column_name": sql.NullString{
				String: "int4",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "nextval('table_name_id_seq')",
				Valid:  true,
			},
		},
	}
	mockDbOperateObjDefaultValue := &mockDbOperate{
		sql:      "SELECT column_name,column_default FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 AND column_default LIKE 'nextval%'",
		fields:   []string{"column_name", "column_default"},
		mockData: mockDataDefaultValue,
	}
	dbOperates = append(dbOperates, mockDbOperateObjDefaultValue)

	// 触发规则
	assertSqlIncorrect("CREATE TABLE test1 (id varchar(100) PRIMARY KEY, name varchar(100), age int);", &dbOperates)
	assertSqlIncorrect("CREATE TABLE test2 (id INT DEFAULT 1 PRIMARY KEY, name varchar(100), age int);", &dbOperates)
	assertSqlIncorrect("CREATE TABLE test3 (id INT PRIMARY KEY, name varchar(100), age int);", &dbOperates)
	assertSqlIncorrect("CREATE TABLE test4 (id INT default 1, name varchar(100), age int, CONSTRAINT pk_test PRIMARY KEY (id));", &dbOperates)
	assertSqlIncorrect("CREATE TABLE test5 (id INT, name varchar(100), age int, CONSTRAINT pk_test PRIMARY KEY (id));", &dbOperates)
	assertSqlIncorrect("ALTER TABLE test ADD COLUMN id1 INT DEFAULT 1, ADD PRIMARY KEY (id1);", &dbOperates)
	assertSqlIncorrect("ALTER TABLE test ADD COLUMN id2 INT, ADD PRIMARY KEY (id2);", &dbOperates)
	assertSqlIncorrect("ALTER TABLE test ALTER COLUMN id3 SET DEFAULT 1, ADD PRIMARY KEY (id3);", &dbOperates)

	// 不触发规则
	assertSqlCorrect("CREATE TABLE test11 (id varchar(100) DEFAULT nextval('table_name_id_seq') PRIMARY KEY, name varchar(100), age int);", &dbOperates)
	assertSqlCorrect("CREATE TABLE test21 (id INT DEFAULT nextval('table_name_id_seq') PRIMARY KEY, name varchar(100), age int);", &dbOperates)
	assertSqlCorrect("CREATE TABLE test31 (id serial PRIMARY KEY, name varchar(100), age int);", &dbOperates)
	assertSqlCorrect("CREATE TABLE test41 (id bigserial PRIMARY KEY,name varchar(100), age int);", &dbOperates)
	assertSqlCorrect("CREATE TABLE test51 (id INT default nextval('table_name_id_seq'), name varchar(100), age int, CONSTRAINT pk_test PRIMARY KEY (id));", &dbOperates)
	assertSqlCorrect("CREATE TABLE test61 (id serial, name varchar(100), age int, CONSTRAINT pk_test PRIMARY KEY (id));", &dbOperates)
	assertSqlCorrect("CREATE TABLE test71 (id bigserial, name varchar(100), age int, CONSTRAINT pk_test PRIMARY KEY (id));", &dbOperates)
	assertSqlCorrect("ALTER TABLE test ADD COLUMN id4 INT DEFAULT nextval('table_name_id_seq'), ADD PRIMARY KEY (id4);", &dbOperates)
	assertSqlCorrect("ALTER TABLE test ADD COLUMN id5 serial, ADD PRIMARY KEY (id5);", &dbOperates)
	assertSqlCorrect("ALTER TABLE test ADD COLUMN id6 bigserial, ADD PRIMARY KEY (id6);", &dbOperates)
	assertSqlCorrect("ALTER TABLE test3 ALTER COLUMN id SET DEFAULT nextval('test_id_seq'), ADD PRIMARY KEY (id);", &dbOperates)
	assertSqlCorrect(`CREATE TABLE color (color_id INT GENERATED BY DEFAULT AS IDENTITY (START WITH 10 INCREMENT BY 10),	color_name VARCHAR NOT NULL);`, &dbOperates)
	// 联合主键不触发规则
	assertSqlCorrect("CREATE TABLE test8 (id bigserial, name varchar(100), age int, CONSTRAINT pk_test PRIMARY KEY (id, name));", &dbOperates)
	assertSqlCorrect("CREATE TABLE test9 (id bigserial, name varchar(100), age int, CONSTRAINT pk_test PRIMARY KEY (name, id));", &dbOperates)
}

func TestDriverImpl_Rule41(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
		OwnerName: "test",
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
		OwnerName: "test",
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test3",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
		OwnerName: "test",
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test4",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
		OwnerName: "test",
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test5",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId41].Rule
	rule.Params.SetParamValue("max_table_connect_number", "3")
	// 走规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId41, 3)
		testAuditWithDbQueryMockConn(RuleId41, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId41, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	mockData := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrect("SELECT x.id from test.test1 x,test.test2 y,test.test3 z,test.test4 q, test.test5 m where x.id = y.id and y.id = z.id and z.id = q.id and q.id = m.id;", &dbOperates)
	assertSqlCorrect("SELECT x.id from test.test1 x,test.test2 y,test.test3 z where x.id = y.id and y.id = z.id;", &dbOperates)

	assertSqlIncorrect("SELECT x.id from test.test1 x join test.test2 y on x.id = y.id join test.test3 z on y.id = z.id join test.test4 q on z.id = q.id join test.test5 m on q.id = m.id;", &dbOperates)
	assertSqlCorrect("SELECT x.id from test.test1 x join test.test2 y on x.id = y.id join test.test3 z on y.id = z.id;", &dbOperates)

	assertSqlIncorrect("select id from test.test1 where id in(SELECT x.id from test.test1 x,test.test2 y,test.test3 z,test.test4 q, test.test5 m where x.id = y.id and y.id = z.id and z.id = q.id and q.id = m.id);", &dbOperates)
	assertSqlCorrect("select id from test.test1 where id in(SELECT x.id from test.test1 x,test.test2 y,test.test3 z where x.id = y.id and y.id = z.id);", &dbOperates)

	assertSqlIncorrect("insert into test.test1(id) SELECT x.id from test.test1 x,test.test2 y,test.test3 z,test.test4 q, test.test5 m where x.id = y.id and y.id = z.id and z.id = q.id and q.id = m.id", &dbOperates)
	assertSqlCorrect("insert into test.test1(id) SELECT x.id from test.test1 x,test.test2 y,test.test3 z where x.id = y.id and y.id = z.id;", &dbOperates)

	assertSqlIncorrect("update test.test1 set name='test' where id in (SELECT x.id from test.test1 x,test.test2 y,test.test3 z,test.test4 q, test.test5 m where x.id = y.id and y.id = z.id and z.id = q.id and q.id = m.id);", &dbOperates)
	assertSqlCorrect("update test.test1 set name='test' where id in (SELECT x.id from test.test1 x,test.test2 y,test.test3 z where x.id = y.id and y.id = z.id);", &dbOperates)

	assertSqlIncorrect("delete from test.test1 where id in(SELECT x.id from test.test1 x,test.test2 y,test.test3 z,test.test4 q, test.test5 m where x.id = y.id and y.id = z.id and z.id = q.id and q.id = m.id);", &dbOperates)
	assertSqlCorrect("delete from test.test1 where id in(SELECT x.id from test.test1 x,test.test2 y,test.test3 z where x.id = y.id and y.id = z.id);", &dbOperates)

	assertSqlIncorrect("insert into test.test1(id) SELECT x.id from test.test1 x left join test.test2 y on x.id = y.id left join test.test3 z on y.id = z.id left join test.test4 q on z.id = q.id join test.test5 m on q.id = m.id;", &dbOperates)
	assertSqlCorrect("insert into test.test1(id) SELECT x.id from test.test1 x left join test.test2 y on x.id = y.id left join test.test3 z on y.id = z.id;", &dbOperates)

	assertSqlIncorrect("update test.test1 set name = 'test' where id in(SELECT x.id from test.test1 x left join test.test2 y on x.id = y.id left join test.test3 z on y.id = z.id left join test.test4 q on z.id = q.id join test.test5 m on q.id = m.id);", &dbOperates)
	assertSqlCorrect("update test.test1 set name = 'test' where id in(SELECT x.id from test.test1 x left join test.test2 y on x.id = y.id left join test.test3 z on y.id = z.id);", &dbOperates)

	assertSqlIncorrect("delete from test.test1 where id in(SELECT x.id from test.test1 x left join test.test2 y on x.id = y.id left join test.test3 z on y.id = z.id left join test.test4 q on z.id = q.id join test.test5 m on q.id = m.id);", &dbOperates)
	assertSqlCorrect("delete from test.test1 where id in(SELECT x.id from test.test1 x left join test.test2 y on x.id = y.id left join test.test3 z on y.id = z.id);", &dbOperates)
}

func TestDriverImpl_Rule42(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "testindex",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "column1",
			},
			{
				ColumnName: "column3",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId42)
		testAuditWithDbQueryMockConn(RuleId42, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId42, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	mockData := []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	// 触发规则（使用函数）
	assertSqlIncorrect("insert into test.testindex select x.* from test.testindex x where abs(x.column1) > 0;", &dbOperates)
	assertSqlIncorrect("update test.testindex set column3 = 1 where column1 in(select x.column1 from test.testindex x where abs(x.column1) > 0);", &dbOperates)
	assertSqlIncorrect("delete from test.testindex where column1 in(select x.column1 from test.testindex x where abs(x.column1) > 0);", &dbOperates)
	assertSqlIncorrect("select x.* from test.testindex x where abs(x.column1) > 0;", &dbOperates)
	// 触发规则（表达式）
	assertSqlIncorrect("insert into test.testindex select x.* from test.testindex x where column1 + 1 > 0;", &dbOperates)
	assertSqlIncorrect("update test.testindex set column3 = 1 where column1 in(select x.column1 from test.testindex x where column1 + 1 > 0);", &dbOperates)
	assertSqlIncorrect("delete from test.testindex where column1 in(select x.column1 from test.testindex x where column1 + 1 > 0);", &dbOperates)
	assertSqlIncorrect("select x.* from test.testindex x where column1 + 1 > 0;", &dbOperates)

	// 不触发规则
	assertSqlCorrect("insert into test.testindex select x.* from test.testindex x ;", &dbOperates)
	assertSqlCorrect("insert into test.testindex select x.* from test.testindex x;", &dbOperates)
	assertSqlCorrect("update test.testindex set column3 = 1 where column1 in(select x.column1 from test.testindex x);", &dbOperates)
	assertSqlCorrect("delete from test.testindex where column1 in(select x.column1 from test.testindex x);", &dbOperates)
	assertSqlCorrect("select x.* from test.testindex x;", &dbOperates)
	// 不触发规则
	assertSqlCorrect("insert into test.testindex select x.* from test.testindex x;", &dbOperates)
	assertSqlCorrect("update test.testindex set column3 = 1 where column1 in(select x.column1 from test.testindex x);", &dbOperates)
	assertSqlCorrect("delete from test.testindex where column1 in(select x.column1 from test.testindex x);", &dbOperates)
	assertSqlCorrect("select x.* from test.testindex x;", &dbOperates)
}

func TestDriverImpl_Rule43(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 走规则
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age + 1 < 100;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age - 1 < 100;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age * 1 < 100;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age / 1 < 100;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where 100 > age + 1;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age in (select age from test.test2 where age + 1 < 100);", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, (select age from test.test2 where age + 1 < 100) age from test.test1 where age > 0;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT t.id, t.name, t.age from (select id, name, age from test.test1 where age + 1 < 100) t where t.age > 0;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where max(age) < 100;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age in (select age from test.test2 where max(age) < 100);", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, (select age from test.test2 where max(age) < 100) age from test.test1 where age > 0;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT t.id, t.name, t.age from (select id, name, age from test.test1 where max(age) < 100) t where t.age > 0;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "insert into test.test1 SELECT id, name, age from test.test1 where age + 1 < 100;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "insert into test.test1 SELECT id, name, age from test.test1 where max(age) < 100;", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "update test.test1 set name = 'test' where id in (SELECT id from test.test1 where age + 1 < 100);", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "update test.test1 set name = 'test' where id in (SELECT id from test.test1 where max(age) < 100);", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "delete from test.test1 where id in (SELECT id from test.test1 where age + 1 < 100);", newTestResults().add(RuleId43), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "delete from test.test1 where id in (SELECT id from test.test1 where max(age) < 100);", newTestResults().add(RuleId43), pgContext, nil, nil)
	// 不走规则
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age < 100;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where 100 > age;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, age from test.test1 where age in (select age from test.test2 where age < 100);", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id, name, (select age from test.test2 where age < 100) age from test.test1 where age > 0;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT t.id, t.name, t.age from (select id, name, age from test.test1 where age < 100) t where t.age > 0;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "insert into test.test1 SELECT id, name, age from test.test1 where age < 100;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "update test.test1 set name = 'test' where id in (SELECT id from test.test1 where age < 100);", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "delete from test.test1 where id in (SELECT id from test.test1 where age < 100);", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT id + 1 as new_id from test.test1 where age < 100;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId43, t, "SELECT max(id) from test.test1 where age < 100;", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule44(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId44)
		testAuditWithDbQueryMockConn(RuleId44, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId44, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	// mock table test1
	mockData := []map[string]sql.NullString{
		{
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "name",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "age",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObj := &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	// mock table test2
	mockDbOperateObj = &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrect("insert into test.test1(id) select x.id from test.test1 x,test.test2 y where x.id > 0 or y.name like 'li%';", &dbOperates)
	assertSqlCorrect("insert into test.test1(id) select x.id from test.test1 x,test.test2 y where x.id > 0 or x.name like 'li%';", &dbOperates)

	assertSqlIncorrect("update test.test1 set name='test' where id in (select x.id from test.test1 x,test.test2 y where x.id > 0 or y.name like 'li%');", &dbOperates)
	assertSqlCorrect("update test.test1 set name='test' where id in (select x.id from test.test1 x,test.test2 y where x.id > 0 or x.name like 'li%');", &dbOperates)

	assertSqlIncorrect("delete from test.test1 where id in (select x.id from test.test1 x,test.test2 y where x.id > 0 or y.name like 'li%');", &dbOperates)
	assertSqlCorrect("delete from test.test1 where id in (select x.id from test.test1 x,test.test2 y where x.id > 0 or x.name like 'li%');", &dbOperates)

	assertSqlIncorrect("select x.id,y.name from test.test1 x, test.test2 y where x.id > 0 or y.name like 'Li%';", &dbOperates)
	assertSqlCorrect("select x.id,x.name from test.test1 x where x.id > 0 union select y.id, y.name from test.test2 y where y.name like 'Li%';", &dbOperates)
}

func TestDriverImpl_Rule45(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{
		UsingType: UsingTypeOnline,
		DatabaseInfo: &DatabaseInfo{
			DatabaseName:  "postgres",
			CurrentSchema: "public",
			SchemaInfoMap: schemaInfoMap,
		},
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	assertSqlIncorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults().add(RuleId45)
		testAuditWithEpMockConn(RuleId45, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}
	assertSqlCorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults()
		testAuditWithEpMockConn(RuleId45, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}
	// Incorrect
	sqlSmt := "select * from table1;"
	mockEp := epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
				"Plan": {
				  "Node Type": "Seq Scan",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Relation Name": "table1",
				  "Alias": "table1",
				  "Startup Cost": 0.00,
				  "Total Cost": 13.20,
				  "Plan Rows": 320,
				  "Plan Width": 222
				}
        }]`,
	}

	// 主表：全表扫描，子表：非全表扫描
	assertSqlIncorrect(sqlSmt, mockEp)
	sqlSmt = "select * from table1 where name in (select name from table2 where id = 1);"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[
		  {
			"Plan": {
			  "Node Type": "Hash Join",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Join Type": "Inner",
			  "Startup Cost": 8.17,
			  "Total Cost": 20.33,
			  "Plan Rows": 1,
			  "Plan Width": 440,
			  "Inner Unique": true,
			  "Hash Cond": "((table1.name)::text = (table2.name)::text)",
			  "Plans": [
				{
				  "Node Type": "Seq Scan",
				  "Parent Relationship": "Outer",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Relation Name": "table1",
				  "Alias": "table1",
				  "Startup Cost": 0.00,
				  "Total Cost": 11.70,
				  "Plan Rows": 170,
				  "Plan Width": 440
				},
				{
				  "Node Type": "Hash",
				  "Parent Relationship": "Inner",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Startup Cost": 8.16,
				  "Total Cost": 8.16,
				  "Plan Rows": 1,
				  "Plan Width": 218,
				  "Plans": [
					{
					  "Node Type": "Index Scan",
					  "Parent Relationship": "Outer",
					  "Parallel Aware": false,
					  "Async Capable": false,
					  "Scan Direction": "Forward",
					  "Index Name": "table2_pkey",
					  "Relation Name": "table2",
					  "Alias": "table2",
					  "Startup Cost": 0.14,
					  "Total Cost": 8.16,
					  "Plan Rows": 1,
					  "Plan Width": 218,
					  "Index Cond": "(id = 1)"
					}
				  ]
				}
			  ]
			}
		  }
		]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// 主表：非全表扫描，子表：全表扫描
	assertSqlIncorrect(sqlSmt, mockEp)
	sqlSmt = "select * from table1 where id = 1 and name in (select name from table2);"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[
		  {
			"Plan": {
			  "Node Type": "Nested Loop",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Join Type": "Semi",
			  "Startup Cost": 0.14,
			  "Total Cost": 21.99,
			  "Plan Rows": 1,
			  "Plan Width": 440,
			  "Inner Unique": false,
			  "Join Filter": "((table1.name)::text = (table2.name)::text)",
			  "Plans": [
				{
				  "Node Type": "Index Scan",
				  "Parent Relationship": "Outer",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Scan Direction": "Forward",
				  "Index Name": "table1_pkey",
				  "Relation Name": "table1",
				  "Alias": "table1",
				  "Startup Cost": 0.14,
				  "Total Cost": 8.16,
				  "Plan Rows": 1,
				  "Plan Width": 440,
				  "Index Cond": "(id = 1)"
				},
				{
				  "Node Type": "Seq Scan",
				  "Parent Relationship": "Inner",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Relation Name": "table2",
				  "Alias": "table2",
				  "Startup Cost": 0.00,
				  "Total Cost": 11.70,
				  "Plan Rows": 170,
				  "Plan Width": 218
				}
			  ]
			}
		  }
		]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// 主表：全表扫描，子表：全表扫描
	assertSqlIncorrect(sqlSmt, mockEp)
	sqlSmt = "select * from table1 where name in (select name from table2);"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[
		  {
			"Plan": {
			  "Node Type": "Hash Join",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Join Type": "Semi",
			  "Startup Cost": 13.82,
			  "Total Cost": 27.86,
			  "Plan Rows": 170,
			  "Plan Width": 440,
			  "Inner Unique": false,
			  "Hash Cond": "((table1.name)::text = (table2.name)::text)",
			  "Plans": [
				{
				  "Node Type": "Seq Scan",
				  "Parent Relationship": "Outer",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Relation Name": "table1",
				  "Alias": "table1",
				  "Startup Cost": 0.00,
				  "Total Cost": 11.70,
				  "Plan Rows": 170,
				  "Plan Width": 440
				},
				{
				  "Node Type": "Hash",
				  "Parent Relationship": "Inner",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Startup Cost": 11.70,
				  "Total Cost": 11.70,
				  "Plan Rows": 170,
				  "Plan Width": 218,
				  "Plans": [
					{
					  "Node Type": "Seq Scan",
					  "Parent Relationship": "Outer",
					  "Parallel Aware": false,
					  "Async Capable": false,
					  "Relation Name": "table2",
					  "Alias": "table2",
					  "Startup Cost": 0.00,
					  "Total Cost": 11.70,
					  "Plan Rows": 170,
					  "Plan Width": 218
					}
				  ]
				}
			  ]
			}
		  }
		]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// Correct
	sqlSmt = "select * from table1 where id = 1;"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			"Plan": {
			  "Node Type": "Index Scan",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Scan Direction": "Forward",
			  "Index Name": "table1_pkey",
			  "Relation Name": "table1",
			  "Alias": "table1",
			  "Startup Cost": 0.15,
			  "Total Cost": 8.17,
			  "Plan Rows": 1,
			  "Plan Width": 222,
			  "Index Cond": "(id = 1)"
			}
		  }]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)

	// 主表：非全表扫描，子表：非全表扫描
	sqlSmt = "select * from table1 where name in (select name from table2 where id = 1) and id = 100;"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[
		  {
			"Plan": {
			  "Node Type": "Nested Loop",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Join Type": "Inner",
			  "Startup Cost": 0.29,
			  "Total Cost": 16.34,
			  "Plan Rows": 1,
			  "Plan Width": 440,
			  "Inner Unique": true,
			  "Join Filter": "((table1.name)::text = (table2.name)::text)",
			  "Plans": [
				{
				  "Node Type": "Index Scan",
				  "Parent Relationship": "Outer",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Scan Direction": "Forward",
				  "Index Name": "table1_pkey",
				  "Relation Name": "table1",
				  "Alias": "table1",
				  "Startup Cost": 0.14,
				  "Total Cost": 8.16,
				  "Plan Rows": 1,
				  "Plan Width": 440,
				  "Index Cond": "(id = 100)"
				},
				{
				  "Node Type": "Index Scan",
				  "Parent Relationship": "Inner",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Scan Direction": "Forward",
				  "Index Name": "table2_pkey",
				  "Relation Name": "table2",
				  "Alias": "table2",
				  "Startup Cost": 0.14,
				  "Total Cost": 8.16,
				  "Plan Rows": 1,
				  "Plan Width": 218,
				  "Index Cond": "(id = 1)"
				}
			  ]
			}
		  }
		]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)
}

func TestDriverImpl_Rule47(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where id != 0;", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where id <> 0;", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where id not in(1,2,3);", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where id > 3;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where not EXISTS(select 1 from test.test2 t where t.id > 1000);", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where EXISTS(select 1 from test.test2 t where t.id <= 1000);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where name not like 'li%';", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where name like 'zhang%';", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where name is not null;", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "select * from test.test1 where length(name) > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where id !=0", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where id >0", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where id <>0", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where id not in  (0,1,2)", newTestResults().add(RuleId47), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where  not exists (select 1 from test.test2 t where t.id > 1000);", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where EXISTS(select 1 from test.test2 t where t.id <= 1000);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where name is not null", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1 where length(name) > 0", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1  where name not like 'li%';", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "update test.test1 set id = 1  where name  like 'li%';", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "insert into test.test1 select * from test.test2 where id != 0;", newTestResults().add(RuleId47), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "insert into test.test1 select * from test.test2 where id <> 0;", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "insert into test.test1 select * from test.test2 where id not in(1,2,3);", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "insert into test.test1 select * from test.test2 where not exists (select 1 from test.test2 t where t.id > 1000);", newTestResults().add(RuleId47), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "insert into test.test1 select * from test.test1 where name is not null;", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "insert into test.test1 select * from test.test1 where name not like 'li%';", newTestResults().add(RuleId47), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "delete from test.test1 where id != 0;", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "delete from test.test1 where id > 0", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId47, t, "delete from test.test1 where id <> 0;", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "delete from test.test1 where id not in (1,2,3);", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "delete from test.test1 where not exists (select 1 from test.test2 t where t.id > 1000); ", newTestResults().add(RuleId47), pgContext, nil, nil)
	testSingleSqlAudit(RuleId47, t, "delete from test.test1 where EXISTS(select 1 from test.test2 t where t.id <= 1000);", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule48(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId48, t, "create table test1(id int);", newTestResults().add(RuleId48), pgContext, nil, nil)
	testSingleSqlAudit(RuleId48, t, "create table test2(id int, created_time date);", newTestResults().add(RuleId48), pgContext, nil, nil)
	testSingleSqlAudit(RuleId48, t, "create table test3(id int, created_time TIMESTAMP);", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule49(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId49, t, "create table test1(id int);", newTestResults().add(RuleId49), pgContext, nil, nil)
	testSingleSqlAudit(RuleId49, t, "create table test2(id int, modified_time date);", newTestResults().add(RuleId49), pgContext, nil, nil)
	testSingleSqlAudit(RuleId49, t, "create table test3(id int, modified_time TIMESTAMP);", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule50(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId50].Rule
	assertSqlCorrect := func(sql string) {
		rule.Params.SetParamValue("expect_object_name_character_max_length", "63")
		testSingleSqlAudit(RuleId50, t, sql, newTestResults(), pgContext, nil, nil)
	}
	assertSqlIncorrect := func(sql string) {
		rule.Params.SetParamValue("expect_object_name_character_max_length", "5")
		testSingleSqlAudit(RuleId50, t, sql, newTestResults().add(RuleId50, 5), pgContext, nil, nil)
	}

	assertSqlIncorrect(`create table test123456(id int, name varchar(5));`)
	assertSqlCorrect(`create table test(id int, name varchar(5), name1 varchar(5), modified_time date);`)

	assertSqlIncorrect(`create table test2(id int, nick_name varchar(5), modified_time date);`)
	assertSqlCorrect(`create table test3(id int, name varchar(5), alias varchar(5));`)

	assertSqlIncorrect(`alter table test add column new123456 int;`)
	assertSqlCorrect(`alter table test add column newCn int;`)

	assertSqlIncorrect(`alter table test3 rename to new_table_name;`)
	assertSqlCorrect(`alter table test2 rename to test1;`)

	assertSqlIncorrect(`alter table test rename column modified_time to new_column_name;`)
	assertSqlCorrect(`alter table test1 rename column modified_time to time;`)

	assertSqlIncorrect(`create index idx_123456 on test(name);`)
	assertSqlCorrect(`create index idx_1 on test(name1);`)

	assertSqlIncorrect(`alter sequence sequence_name rename to new_sequence_name;`)
	assertSqlCorrect(`alter sequence sequence_name rename to n_seq;`)

	assertSqlIncorrect(`create or replace function calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;`)
	assertSqlCorrect(`create or replace function n_fun(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;`)
	assertSqlIncorrect(`create view view_name as select id, modified_time from test where id > 0;`)
	assertSqlCorrect(`create view vname as select id, modified_time from test where id > 0;`)

	assertSqlIncorrect(`create or replace view view_name as select id, modified_time from test where id > 0;`)
	assertSqlCorrect(`create or replace view vname as select id, modified_time from test where id > 0;`)

	assertSqlIncorrect(`create sequence sequence_name start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;`)
	assertSqlCorrect(`create sequence seq_1 start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;`)
}

func TestDriverImpl_Rule51(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId51, t, "create table test.test51_1 (id int,foreign_id int,name varchar(100),constraint fk_id foreign key (foreign_id) references test51_fk(referenced_column));", newTestResults().add(RuleId51), pgContext, nil, nil)
	testSingleSqlAudit(RuleId51, t, "create table test.test51_2 (id int,foreign_id int unique,foreign_id int unique,name varchar(100),constraint fk_id foreign key (foreign_id) references test51_fk(referenced_column));", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId51, t, "create table test.test51_3 (id int,foreign_id int,name varchar(100),constraint fk_id foreign key (foreign_id) references test51_fk(referenced_column));", newTestResults().add(RuleId51), pgContext, nil, nil)
	testSingleSqlAudit(RuleId51, t, "create table test.test51_4 (id int,foreign_id int,name varchar(100),constraint uni_id unique (foreign_id),constraint fk_id foreign key (foreign_id) references test51_fk(referenced_column));", newTestResults(), pgContext, nil, nil)

	// 模拟查询数据库中列是否在索引中
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId51)
		testAuditWithDbQueryMockConn(RuleId51, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId51, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	var mockData []map[string]sql.NullString
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrect("alter table test.test51_1 add constraint fk_constraint_name foreign key (foreign_id) references test1(id);", &dbOperates)
	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "test51_foreign_id_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX test51_foreign_id_key ON test.test51_2 USING btree (foreign_id)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlCorrect("alter table test.test51_2 add constraint fk_constraint_name foreign key (foreign_id) references test1(id);", &dbOperates)

	// 测试列索引在create index或alter table xxx add constraint xxx unique (column1);
	testIncorrectResults := []*testResults{newTestResults().add(RuleId51)}
	testCorrectResults := []*testResults{newTestResults(), newTestResults()}
	// 创建：走规则
	sqls := make([]string, 0)
	sqls = append(sqls, "create table test.test51_5 (id int,foreign_id int,name varchar(100),constraint fk_id foreign key (foreign_id) references test51_fk(referenced_column));")
	testTwoSqlAudit(RuleId51, t, sqls, testIncorrectResults, pgContext, nil, nil)
	// 创建：不走规则
	sqls = make([]string, 0)
	sqls = append(sqls, "create index idx_foreign_id_1 on test.test51_1(foreign_id);")
	testTwoSqlAudit(RuleId51, t, sqls, testCorrectResults, pgContext, nil, nil)

	// 创建：不走规则
	sqls = make([]string, 0)
	sqls = append(sqls, "alter table test.test51_1 add constraint uni_constraint_foreign_id unique (foreign_id);")
	testTwoSqlAudit(RuleId51, t, sqls, testCorrectResults, pgContext, nil, nil)

	// 修改：不走规则
	sqls = make([]string, 0)
	sqls = append(sqls, "alter table test.test51_2 add constraint fk_constraint_name foreign key (foreign_id) references test1(id);")
	sqls = append(sqls, "create index idx_foreign_id_2 on test.test51_2(foreign_id);")
	testTwoSqlAudit(RuleId51, t, sqls, testCorrectResults, pgContext, nil, nil)

	// 修改：不走规则
	sqls = make([]string, 0)
	sqls = append(sqls, "alter table test.test51_3 add constraint uni_constraint_foreign_id unique (foreign_id);")
	testTwoSqlAudit(RuleId51, t, sqls, testCorrectResults, pgContext, nil, nil)
}

func TestDriverImpl_Rule52(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table52",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "distinct_col",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table52_1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "distinct_col",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId52].Rule
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		rule.Params.SetParamValue("expect_index_min_cell_division_percentage", "70")
		results := newTestResults().add(RuleId52, 70)
		testAuditWithDbQueryMockConn(RuleId52, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId52, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"typname": {
				String: "int",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "idx_test_index_single",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE INDEX idx_test_index_single ON test.test_index USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"index_cell_division": sql.NullString{
				String: "60",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select CASE WHEN COUNT(DISTINCT t.cell_division_number) = 0 OR COUNT(t.*) = 0 THEN 0 ELSE COUNT(DISTINCT t.cell_division_number) * 100 / NULLIF(COUNT(t.*), 0) END AS index_cell_division from ( select distinct_col as cell_division_number from test.table52 limit 50000 ) t",
		fields:   []string{"index_cell_division"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "idx_test_index_single",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE INDEX idx_test_index_single ON test.test_index USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrect("create index idx_table52_distinct_col on test.table52(distinct_col);", &dbOperates)

	// 不触发规则
	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": {
				String: "int",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "idx_test_index_single",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE INDEX idx_test_index_single ON test.test_index USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"index_cell_division": sql.NullString{
				String: "100",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select CASE WHEN COUNT(DISTINCT t.cell_division_number) = 0 OR COUNT(t.*) = 0 THEN 0 ELSE COUNT(DISTINCT t.cell_division_number) * 100 / NULLIF(COUNT(t.*), 0) END AS index_cell_division from ( select distinct_col as cell_division_number from test.table52_1 limit 50000 ) t",
		fields:   []string{"index_cell_division"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "idx_test_index_single",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE INDEX idx_test_index_single ON test.test_index USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlCorrect("create index idx_table52_distinct_col_1 on test.table52_1(distinct_col);", &dbOperates)
}

func TestDriverImpl_Rule53(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test_index",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId53)
		testAuditWithDbQueryMockConn(RuleId53, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId53, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"typname": {
				String: "int",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "idx_test_index_single",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE INDEX idx_test_index_single ON test.test_index USING btree (column1)",
				Valid:  true,
			},
		},
		{
			"indexname": sql.NullString{
				String: "idx_test_index_compose",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE INDEX idx_test_index_compose ON test.test_index USING btree (column3, column2, column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	// 走规则
	assertSqlIncorrect("select * from test.test_index where column2 = 100;", &dbOperates)
	assertSqlIncorrect("select * from test.test_index where column1 > 100;", &dbOperates)

	assertSqlIncorrect("select t.* from (select * from test.test_index where column2 = 100) t;", &dbOperates)
	assertSqlIncorrect("select t.* from (select * from test.test_index where column1 > 100) t;", &dbOperates)

	assertSqlIncorrect("insert into test.test(id,name,age) select id,name,age from test.test_index where column2 = 100;", &dbOperates)
	assertSqlIncorrect("insert into test.test(id,name,age) select id,name,age from test.test_index where column1 > 100;", &dbOperates)

	assertSqlIncorrect("update test.test set name = 'test' where id in (select id from test.test_index where column2 = 100);", &dbOperates)
	assertSqlIncorrect("update test.test set name = 'test' where id in (select id from test.test_index where column1 > 100);", &dbOperates)

	assertSqlIncorrect("delete from test.test where id in (select id from test.test_index where column2 = 100);", &dbOperates)
	assertSqlIncorrect("delete from test.test where id in (select id from test.test_index where column1 > 100);", &dbOperates)

	// 不走规则
	assertSqlCorrect("select * from test.test_index where column3 = 100 and column1 = 100;", &dbOperates)
	assertSqlCorrect("select * from test.test_index where column1 = 100;", &dbOperates)

	assertSqlCorrect("select t.* from (select * from test.test_index where column3 = 100 and column1 = 100) t;", &dbOperates)
	assertSqlCorrect("select t.* from (select * from test.test_index where column1 = 100) t;", &dbOperates)

	assertSqlCorrect("insert into test.test(id,name,age) select id,name,age from test.test_index where column3 = 100 and column1 = 100;", &dbOperates)
	assertSqlCorrect("insert into test.test(id,name,age) select id,name,age from test.test_index where column1 = 100;", &dbOperates)

	assertSqlCorrect("update test.test set name = 'test' where id in (select id from test.test_index where column3 = 100 and column1 = 100);", &dbOperates)
	assertSqlCorrect("update test.test set name = 'test' where id in (select id from test.test_index where column1 = 100);", &dbOperates)

	assertSqlCorrect("delete from test.test where id in (select id from test.test_index where column3 = 100 and column1 = 100);", &dbOperates)
	assertSqlCorrect("delete from test.test where id in (select id from test.test_index where column1 = 100);", &dbOperates)
}

func TestDriverImpl_Rule54(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId54, t, "create schema test123456$;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create schema test123456;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create table test123456$(id int, modified_time TIMESTAMP);", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create table test123456(id int, modified_time TIMESTAMP);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create table test1234567$ as select * from test;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create table test1234567 as select * from test;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create table test1(test1234568$ int, modified_time TIMESTAMP, name varchar(100));", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create table test2(test1234568 int, modified_time TIMESTAMP, name varchar(100),name2 varchar(100));", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "alter table test1 add column test123456$ int;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "alter table test2 add column test123456 int;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "alter table test1 rename to test123456$$;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "alter table test2 rename to test1;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "alter table test1 rename column modified_time to test123456$$$;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "alter table test1 rename column test1234568 to test1234567;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create index test123456$ on test1(name);", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create index test123456 on test1(name2);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create view view_name$ as select id, modified_time from test where id > 0;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create view view_name as select id, modified_time from test where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create or replace view view_name$ as select id, modified_time from test where id > 0;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create or replace view view_name as select id, modified_time from test where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, `create trigger test123456$ after insert or update on test for each row execute function log_test_changes();`, newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, `create trigger test123456 after insert or update on test for each row execute function log_test_changes();`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create or replace function calculate_total_price$(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create or replace function calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, `create or replace procedure test123456$(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$`, newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, `create or replace procedure test123456(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "create sequence sequence_name$ start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "create sequence sequence_name start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, "alter sequence sequence_name rename to new_sequence_name$;", newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, "alter sequence sequence_name rename to new_sequence_name;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId54, t, `CREATE TABLESPACE test123456$ OWNER user_name LOCATION 'directory_path';`, newTestResults().add(RuleId54), pgContext, nil, nil)
	testSingleSqlAudit(RuleId54, t, `CREATE TABLESPACE test123456 OWNER user_name LOCATION 'directory_path';`, newTestResults(), pgContext, nil, nil)

}

func TestDriverImpl_Rule55(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test",
			},
			{
				ColumnName: "sex",
				TableName:  "test",
			},
			{
				ColumnName: "salary",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId55].Rule
	rule.Params.SetParamValue("expect_bind_variable_max_number", "3")
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId55, t, sql, newTestResults(), pgContext, nil, nil)
	}
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId55, t, sql, newTestResults().add(RuleId55, 3), pgContext, nil, nil)
	}

	assertSqlIncorrect(`insert into test(id,name,age,sex,salary) select id,name,age,salary from test where id = $1 and name = $2 and age > $3 and sex = $4 and salary = $5;`)
	assertSqlCorrect(`insert into test(id,name,age,sex,salary) select id,name,age,salary from test where id = $1 and name = $2 and age > $3;`)

	assertSqlIncorrect(`insert into test(id,name,age,sex,salary) values($1,$2,$3,$4,$5);`)
	assertSqlCorrect(`insert into test(id,name,age,sex,salary) values($1,$2,$3,1,100000);`)

	assertSqlIncorrect(`update test set id = $1, name = $2 where id > $3 and name = $4 and age = $5;`)
	assertSqlCorrect(`update test set id = $1, name = $2 where id > $3;`)

	assertSqlIncorrect(`delete from test where id = $1 and name = $2 and age > $3 and sex = $4 and salary = $5;`)
	assertSqlCorrect(`delete from test where id = $1 and name = $2 and age > $3;`)

	assertSqlIncorrect(`select * from test where id = $1 and name = $2 and age > $3 and sex = $4 and salary = $5;`)
	assertSqlCorrect(`select * from test where id = $1 and name = $2 and age > $3;`)

	assertSqlIncorrect(`select t.* from (select id from test where id = $1 and name = $2 and age > $3 and sex = $4 and salary = $5) t;`)
	assertSqlCorrect(`select t.* from (select id from test where id = $1 and name = $2 and age > $3) t;`)
}

func TestDriverImpl_Rule56(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId56].Rule
	rule.Params.SetParamValue("expect_fixed_prefix", "tmp_")
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId56, t, sql, newTestResults(), pgContext, nil, nil)
	}
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId56, t, sql, newTestResults().add(RuleId56, "tmp_"), pgContext, nil, nil)
	}

	assertSqlIncorrect(`CREATE TEMPORARY TABLE tmp1_table1 (id SERIAL PRIMARY KEY, name VARCHAR(50), email VARCHAR(50));`)
	assertSqlIncorrect(`CREATE TEMPORARY TABLE TMP1_table2 (id SERIAL PRIMARY KEY, name VARCHAR(50), email VARCHAR(50));`)
	assertSqlCorrect(`CREATE TEMPORARY TABLE tmp_table3 (id SERIAL PRIMARY KEY, name VARCHAR(50), email VARCHAR(50));`)
	assertSqlCorrect(`CREATE TEMPORARY TABLE TMP_TABLE4 (id SERIAL PRIMARY KEY, name VARCHAR(50), email VARCHAR(50));`)
}

func TestDriverImpl_Rule57(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t_table1",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "t_table2",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table_name",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "column_name",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId57, t, "Insert into t1 values(1, 'a');", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "insert into t1 values(1, 'a'), (2, 'b');", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "Update t1 set name = 'test';", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "update t1 set name = 'test';", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "Delete from t1 where id > 0;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "delete from t1 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "delete From t1 where id > 0;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "delete from t1 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "delete from t1 Where id > 0;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "delete from t1 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "Select * from t1 where id > 0;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "select * from t1 where id > 0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "select * from t1 where id > 0 And id < 100;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "select * from t1 where id > 0 and id < 100;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "select * from t1 where id > 0 oR id < 100;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "select * from t1 where id > 0 or id < 100;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "select id,name from t1 Union select id,name from t2;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "select id,name from t1 union select id,name from t2;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "select id,count(name) from t1 where id > 0 Group by id,name;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "select id,count(name) from t1 where id > 0 group by id,name;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "select id,name from t1 where id > 0 group by id,name Having id >0;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "select id,name from t1 where id > 0 group by id,name having id >0;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "select id,Count(name) from t1 where id > 0 group by id,name;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "select id,count(name) from t1 where id > 0 group by id,name;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "create user test_user password 'test_user';", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "Create user test_user password 'test_user';", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "alter user test_user password 'test123';", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "Alter user test_user password 'test123';", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "drop user if exists test1_user;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "drop User if exists test1_user;", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "create database test_db owner test_user;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "create Database test_db owner test_user;", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "alter database test_db rename to t_db;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "alter Database test_db rename to t_db;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "alter dataBase test_db rename to t_db;", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "alter database t_db owner to postgres;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "alter dataBase t_db owner to postgres;", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "create table test1 (id int);", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "Create table test2 (id int);", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "Alter table test1 add id1 int;", newTestResults().add(RuleId57), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "alter table test2 add id1 int;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "alter table test1 drop id1;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "Alter table test2 drop id1;", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "ALTER TABLE table_name ALTER COLUMN column_name TYPE int;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "ALTER TABLE table_name Alter COLUMN column_name TYPE int;", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "drop database if exists t_db;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "drop database if Exists t_db;", newTestResults().add(RuleId57), pgContext, nil, nil)

	testSingleSqlAudit(RuleId57, t, "drop table if exists t_table1;", newTestResults(), pgContext, nil, nil)
	testSingleSqlAudit(RuleId57, t, "drop table if Exists t_table2;", newTestResults().add(RuleId57), pgContext, nil, nil)
}

func TestDriverImpl_Rule59(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId59)
		testAuditWithDbQueryMockConn(RuleId59, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId59, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	// 走规则
	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"typname": {
				String: "int",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = append(mockData, []map[string]sql.NullString{
		{
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "name",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "id1",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "name1",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "id2",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "name2",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
	}...)
	mockDbOperateObj = &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockDbOperateObj = &mockDbOperate{
		sql:      "select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1 join test.table2 t2 on t1.id = t2.column1 and t1.name = t2.name;",
		fields:   []string{"id1", "name1", "id2", "name2"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrect("select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1 join test.table2 t2 on t1.id = t2.column1 and t1.name = t2.name;", &dbOperates)

	// 子查询
	assertSqlIncorrect("select t.* from (select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1 join test.table2 t2 on t1.id = t2.column1 and t1.name = t2.name) t;", &dbOperates)

	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": {
				String: "int",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockData = append(mockData, []map[string]sql.NullString{
		{
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "name",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "id1",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "name1",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "id2",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "name2",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
	}...)
	mockDbOperateObj = &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrect("insert into test.test1(id,name) select t1.id, t1.name from test.table1 t1 join test.table2 t2 on t1.id = t2.column1 and t1.name = t2.name;", &dbOperates)
	assertSqlIncorrect("update test.test1 set name = 'test' where id in (select t1.id from test.table1 t1 join test.table2 t2 on t1.id = t2.column1 and t1.name = t2.name);", &dbOperates)
	assertSqlIncorrect("delete from test.test1 where id in (select t1.id from test.table1 t1 join test.table2 t2 on t1.id = t2.column1 and t1.name = t2.name);", &dbOperates)
	// 没有别名
	assertSqlIncorrect("select id1,name1,id2,name2 from test.table1 join test.table2 on id1 = column1 and name1 = name2;", &dbOperates)
	// 不走规则
	assertSqlCorrect("select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1 join test.table2 t2 on t1.id = t2.id and t1.name = t2.name;", &dbOperates)
	// 子查询
	assertSqlCorrect("select t.* from (select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1 join test.table2 t2 on t1.id = t2.id and t1.name = t2.name) t;", &dbOperates)
	assertSqlCorrect("insert into test.test1(id,name) select t1.id, t1.name from test.table1 t1 join test.table2 t2 on t1.id = t2.id and t1.name = t2.name;", &dbOperates)
	assertSqlCorrect("update test.test1 set name = 'test' where id in (select t1.id from test.table1 t1 join test.table2 t2 on t1.id = t2.id and t1.name = t2.name);", &dbOperates)
	assertSqlCorrect("delete from test.test1 where id in (select t1.id from test.table1 t1 join test.table2 t2 on t1.id = t2.id and t1.name = t2.name);", &dbOperates)
	// 没有别名
	assertSqlCorrect("select id1,name1,id2,name2 from test.table1 join test.table2 on id1 = id2 and name1 = name2;", &dbOperates)
}

func TestDriverImpl_Rule60(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 触发规则
	assertSqlIncorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId60)
		testAuditWithDbQueryMockConn(RuleId60, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId60, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	mockDataForTypename := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "float",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObjForTypename := &mockDbOperate{
		sql:      fmt.Sprintf("SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)"),
		args:     []string{"test"},
		fields:   []string{"typname"},
		mockData: mockDataForTypename,
	}
	dbOperates = append(dbOperates, mockDbOperateObjForTypename)

	mockData := []map[string]sql.NullString{
		{
			"specific_schema": sql.NullString{
				String: "test",
				Valid:  true,
			},
			"specific_name": sql.NullString{
				String: "calculate_average_24972",
				Valid:  true,
			},
			"parameters": sql.NullString{
				String: "double precision,date",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT r.routine_schema as specific_schema, r.routine_name as specific_name, string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters FROM information_schema.routines r JOIN information_schema.parameters p ON r.specific_catalog = p.specific_catalog AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name WHERE r.routine_type = 'FUNCTION' AND r.routine_catalog = $1 AND r.routine_schema = $2 AND r.routine_name = $3 AND p.specific_schema NOT IN ('pg_catalog', 'information_schema') AND p.parameter_mode = 'IN' GROUP BY r.routine_schema, r.routine_name",
		fields:   []string{"specific_schema", "specific_name", "parameters"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrectMock("CREATE FUNCTION calculate_average(a INT, b INT) RETURNS FLOAT AS $$ DECLARE result FLOAT;BEGIN result := (a + b) / 2.0; RETURN result;END;$$ LANGUAGE plpgsql;", &dbOperates)
	assertSqlIncorrectMock("CREATE FUNCTION test.calculate_average(a INT, b INT) RETURNS FLOAT AS $$ DECLARE result FLOAT;BEGIN result := (a + b) / 2.0; RETURN result;END;$$ LANGUAGE plpgsql;", &dbOperates)
	assertSqlIncorrectMock("CREATE or replace FUNCTION Test.Calculate_average(a float, b int) RETURNS FLOAT AS $$ DECLARE result FLOAT;BEGIN result := (a + b) / 2.0; RETURN result;END;$$ LANGUAGE plpgsql;", &dbOperates)
	dbOperates = make([]*mockDbOperate, 0)
	dbOperates = append(dbOperates, mockDbOperateObjForTypename)
	mockData = []map[string]sql.NullString{
		{
			"specific_schema": sql.NullString{
				String: "test",
				Valid:  true,
			},
			"specific_name": sql.NullString{
				String: "calculate_average_24972",
				Valid:  true,
			},
			"parameters": sql.NullString{
				String: "double precision,date",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT r.routine_schema as specific_schema, r.routine_name as specific_name, string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters FROM information_schema.routines r JOIN information_schema.parameters p ON r.specific_catalog = p.specific_catalog AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name WHERE r.routine_type = 'FUNCTION' AND r.routine_catalog = $1 AND r.routine_schema = $2 AND r.routine_name = $3 AND p.specific_schema NOT IN ('pg_catalog', 'information_schema') AND p.parameter_mode = 'IN' GROUP BY r.routine_schema, r.routine_name",
		fields:   []string{"specific_schema", "specific_name", "parameters"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlCorrectMock("CREATE or replace FUNCTION Test.calculate_average_24972(a float, b date) RETURNS FLOAT AS $$ DECLARE result FLOAT;BEGIN result := (a + b) / 2.0; RETURN result;END;$$ LANGUAGE plpgsql;", &dbOperates)
}

func TestDriverImpl_Rule61(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 触发规则
	assertSqlIncorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId61)
		testAuditWithDbQueryMockConn(RuleId61, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId61, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	mockDataForTypename := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "float",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "date",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObjForTypename := &mockDbOperate{
		sql:      fmt.Sprintf("SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)"),
		args:     []string{"test"},
		fields:   []string{"typname"},
		mockData: mockDataForTypename,
	}
	dbOperates = append(dbOperates, mockDbOperateObjForTypename)

	mockData := []map[string]sql.NullString{
		{
			"specific_schema": sql.NullString{
				String: "test",
				Valid:  true,
			},
			"specific_name": sql.NullString{
				String: "get_employee_24974",
				Valid:  true,
			},
			"parameters": sql.NullString{
				String: "integer,character varying",
				Valid:  true,
			},
			"parameters_mode": sql.NullString{
				String: "IN,OUT",
				Valid:  true,
			},
		},
	}

	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT r.routine_schema as specific_schema, r.routine_name as specific_name, string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters, string_agg(p.parameter_mode, ', ' ORDER BY p.ordinal_position) AS parameters_mode FROM information_schema.routines r JOIN information_schema.parameters p ON r.specific_catalog = p.specific_catalog AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name WHERE r.routine_type = 'PROCEDURE' AND r.routine_catalog = $1 AND r.routine_schema = $2 AND r.routine_name = $3 AND p.specific_schema NOT IN ('pg_catalog', 'information_schema') GROUP BY r.routine_schema, r.routine_name",
		fields:   []string{"specific_schema", "specific_name", "parameters", "parameters_mode"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrectMock("CREATE PROCEDURE get_employee(IN employee_id INT, OUT employee_name VARCHAR) LANGUAGE plpgsql AS $$ BEGIN SELECT name INTO employee_name FROM employees WHERE id = employee_id;END;$$;", &dbOperates)
	assertSqlIncorrectMock("CREATE PROCEDURE test.get_employee(IN employee_id INT, OUT employee_name VARCHAR) LANGUAGE plpgsql AS $$ BEGIN SELECT name INTO employee_name FROM employees WHERE id = employee_id;END;$$;", &dbOperates)
	dbOperates = make([]*mockDbOperate, 0)
	dbOperates = append(dbOperates, mockDbOperateObjForTypename)
	mockDataCreateOrReplace := []map[string]sql.NullString{
		{
			"specific_schema": sql.NullString{
				String: "test",
				Valid:  true,
			},
			"specific_name": sql.NullString{
				String: "get_employee_24974",
				Valid:  true,
			},
			"parameters": sql.NullString{
				String: "character varying,character varying",
				Valid:  true,
			},
			"parameters_mode": sql.NullString{
				String: "IN,OUT",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT r.routine_schema as specific_schema, r.routine_name as specific_name, string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters, string_agg(p.parameter_mode, ', ' ORDER BY p.ordinal_position) AS parameters_mode FROM information_schema.routines r JOIN information_schema.parameters p ON r.specific_catalog = p.specific_catalog AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name WHERE r.routine_type = 'PROCEDURE' AND r.routine_catalog = $1 AND r.routine_schema = $2 AND r.routine_name = $3 AND p.specific_schema NOT IN ('pg_catalog', 'information_schema') GROUP BY r.routine_schema, r.routine_name",
		fields:   []string{"specific_schema", "specific_name", "parameters", "parameters_mode"},
		mockData: mockDataCreateOrReplace,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrectMock("CREATE OR REPLACE PROCEDURE test.get_employee(IN employee_id VARCHAR, OUT employee_name VARCHAR) LANGUAGE plpgsql AS $$ BEGIN SELECT name INTO employee_name FROM employees WHERE id = employee_id;END;$$;", &dbOperates)
	dbOperates = make([]*mockDbOperate, 0)
	dbOperates = append(dbOperates, mockDbOperateObjForTypename)
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT r.routine_schema as specific_schema, r.routine_name as specific_name, string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters, string_agg(p.parameter_mode, ', ' ORDER BY p.ordinal_position) AS parameters_mode FROM information_schema.routines r JOIN information_schema.parameters p ON r.specific_catalog = p.specific_catalog AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name WHERE r.routine_type = 'PROCEDURE' AND r.routine_catalog = $1 AND r.routine_schema = $2 AND r.routine_name = $3 AND p.specific_schema NOT IN ('pg_catalog', 'information_schema') GROUP BY r.routine_schema, r.routine_name",
		fields:   []string{"specific_schema", "specific_name", "parameters", "parameters_mode"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlCorrectMock("CREATE OR REPLACE PROCEDURE test.get_employee_24974(IN employee_id INT, OUT employee_name VARCHAR) LANGUAGE plpgsql AS $$ BEGIN SELECT name INTO employee_name FROM employees WHERE id = employee_id;END;$$;", &dbOperates)
}

func TestDriverImpl_Rule62(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId62].Rule
	rule.Params.SetParamValue("expect_max_index_number", "1")
	// 触发规则
	assertSqlIncorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId62, "1")
		testAuditWithDbQueryMockConn(RuleId62, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId62, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"typname": {
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": {
				String: "varchar",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrectMock(`CREATE TABLE test.test1 (id INT primary key, name varchar(100) unique);`, &dbOperates)
	assertSqlIncorrectMock(`CREATE TABLE test.test2 (id INT, name varchar(100) unique, PRIMARY KEY (id));`, &dbOperates)
	assertSqlCorrectMock(`CREATE TABLE test.test3 (id INT, name varchar(100) unique);`, &dbOperates)

	assertSqlIncorrectMock(`CREATE TABLE test.test4 (id INT primary key, name varchar(100) unique);`, &dbOperates)
	assertSqlIncorrectMock(`CREATE TABLE test.test5 (id INT primary key, name varchar(100), unique(name));`, &dbOperates)
	assertSqlCorrectMock(`CREATE TABLE test.test6 (id INT primary key, name varchar(100));`, &dbOperates)

	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": {
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": {
				String: "varchar",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrectMock("create index uniq_id_name1 on test.test1(id,name);", &dbOperates)
	assertSqlIncorrectMock("create unique index uniq_id_name2 on test.test2(id,name);", &dbOperates)
	assertSqlIncorrectMock("create index concurrently uniq_id_name3 on test.test3(id,name);", &dbOperates)
	assertSqlIncorrectMock("alter table test.test1 add PRIMARY key(id);", &dbOperates)
	assertSqlIncorrectMock("alter table test.test2 add unique(id);", &dbOperates)
	assertSqlCorrectMock("drop index test.uniq_id_name1;create index uniq_id_name on test.test1(id,name);", &dbOperates)
}

func TestDriverImpl_Rule63(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				IsNullable: true,
			},
			{
				ColumnName: "name",
				IsNullable: true,
			},
			{
				ColumnName: "age",
				IsNullable: true,
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				IsNullable: false,
			},
			{
				ColumnName: "name",
				IsNullable: true,
			},
			{
				ColumnName: "age",
				IsNullable: true,
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	// 触发规则
	assertSqlIncorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId63)
		testAuditWithDbQueryMockConn(RuleId63, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId63, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	// 走规则
	assertSqlIncorrectMock(`CREATE TABLE test.test11 (id INT primary key, name varchar(100));`, &dbOperates)
	assertSqlIncorrectMock(`CREATE TABLE test.test12 (id INT, name varchar(100) unique);`, &dbOperates)
	assertSqlIncorrectMock(`CREATE TABLE test.test21 (id INT, name varchar(100), CONSTRAINT pk_test_id PRIMARY KEY (id));`, &dbOperates)
	assertSqlIncorrectMock(`CREATE TABLE test.test22 (id INT, name varchar(100), CONSTRAINT uniq_test_id unique (id));`, &dbOperates)
	// 不走规则
	assertSqlCorrectMock(`CREATE TABLE test.test13 (id INT primary key not null, name varchar(100));`, &dbOperates)
	assertSqlCorrectMock(`CREATE TABLE test.test14 (id INT, name varchar(100) unique not null);`, &dbOperates)
	assertSqlCorrectMock(`CREATE TABLE test.test23 (id INT not null , name varchar(100), CONSTRAINT pk_test_id PRIMARY KEY (id));`, &dbOperates)
	assertSqlCorrectMock(`CREATE TABLE test.test24 (id INT not null , name varchar(100), CONSTRAINT pk_test_id unique (id));`, &dbOperates)

	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_column1_key ON test.testindex USING btree (column1)",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	// 走规则
	assertSqlIncorrectMock("create index uniq_id_name on test.test(id,name);", &dbOperates)
	assertSqlIncorrectMock("create unique index uniq_name on test.test(name);", &dbOperates)
	assertSqlIncorrectMock("create index concurrently uniq_age on test.test(age);", &dbOperates)
	assertSqlIncorrectMock("alter table test.test add PRIMARY key(id,name);", &dbOperates)
	assertSqlIncorrectMock("alter table test.test add unique(id,name);", &dbOperates)
	// 不走规则
	assertSqlCorrectMock("alter table test.test1 add PRIMARY key(id);", &dbOperates)
	assertSqlCorrectMock("alter table test.test1 add unique(id);", &dbOperates)
	assertSqlCorrectMock("drop index test.uniq_id_name;create index uniq_id_name on test.test(id);", &dbOperates)
}

func TestDriverImpl_Rule64(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
			},
			{
				ColumnName: "name",
			},
			{
				ColumnName: "age",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	rule := RuleHandlerMap[RuleId64].Rule
	rule.Params.SetParamValue("max_index_count", "1")
	// 触发规则
	assertSqlIncorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId64, "1")
		testAuditWithDbQueryMockConn(RuleId64, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId64, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int4",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_name ON test.test USING btree (name)",
				Valid:  true,
			},
		},
	}

	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrectMock(`CREATE TABLE test.test11 (id INT, name varchar(100) unique, age int, constraint uniq_name_age unique(name,age));`, &dbOperates)
	assertSqlIncorrectMock(`CREATE TABLE test.test22(id INT, name varchar(100), age int, constraint uniq_name unique(name), constraint uniq_name_age unique(name,age));`, &dbOperates)
	assertSqlCorrectMock(`CREATE TABLE test.test33 (id INT, name varchar(100) unique, age int);`, &dbOperates)

	assertSqlIncorrectMock(`CREATE TABLE test.test43 (id INT primary key, name varchar(100), constraint pk_name_age primary key (id,name));`, &dbOperates)
	assertSqlIncorrectMock(`CREATE TABLE test.test55 (id INT primary key, name varchar(100), constraint pk_id primary key (id), constraint pk_id_name primary key (id,name));`, &dbOperates)
	assertSqlCorrectMock(`CREATE TABLE test.test66 (id INT primary key, name varchar(100));`, &dbOperates)

	dbOperates = make([]*mockDbOperate, 0)
	mockData = []map[string]sql.NullString{
		{
			"typname": sql.NullString{
				String: "int",
				Valid:  true,
			},
		},
		{
			"typname": sql.NullString{
				String: "varchar",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.typname as typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typisdefined = true AND n.nspname in ('pg_catalog', $1)",
		fields:   []string{"typname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"indexname": sql.NullString{
				String: "testindex_column1_key",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX testindex_name ON test.test USING btree (name)",
				Valid:  true,
			},
		},
	}

	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT indexname,indexdef FROM pg_indexes where schemaname = $1 and tablename = $2",
		fields:   []string{"indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrectMock("create index uniq_id_name on test.test(id,name);", &dbOperates)
	assertSqlIncorrectMock("create unique index uniq_id_name_1 on test.test1(id,name);", &dbOperates)
	assertSqlIncorrectMock("create index concurrently uniq_id_name_2 on test.test2(id,name);", &dbOperates)
	assertSqlIncorrectMock("alter table test.test add PRIMARY key(id,name);", &dbOperates)
	assertSqlIncorrectMock("alter table test.test add unique(id,name);", &dbOperates)
	assertSqlCorrectMock("create index uniq_age on test.test(age);", &dbOperates)
}

func TestDriverImpl_Rule65(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
		IndexInfoList: []*IndexInfo{
			{
				IndexName:  "idx_test_name1",
				TableName:  "test2",
				OwnerName:  "test",
				ColumnList: []string{"name"},
				IsUnique:   true,
			},
			{
				IndexName:  "idx_test_name11",
				TableName:  "test2",
				OwnerName:  "test",
				ColumnList: []string{"name4"},
				IsUnique:   true,
			},
		},
	}
	pgContext := &PgContext{UsingType: UsingTypeOnline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	testSingleSqlAudit(RuleId65, t, "select id,name,age from test.test limit 1000 offset 10;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select id,name,age from test.test FETCH FIRST 5 ROWS ONLY offset 10;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select id,name,age from test.test limit 1000;", newTestResults(), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select id,name,age from test.test FETCH FIRST 5 ROWS ONLY;", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId65, t, "select (select id,name,age from test.test limit 1000 offset 10) as id,name,age from test.test;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select (select id,name,age from test.test offset 10 FETCH FIRST 5 ROWS ONLY) as id,name,age from test.test;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select (select id,name,age from test.test limit 1000) as id,name,age from test.test;", newTestResults(), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select (select id,name,age from test.test FETCH FIRST 5 ROWS ONLY) as id,name,age from test.test;", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId65, t, "select t.id,t.name,t.age from (select id,name,age from test.test limit 1000 offset 10) t;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select t.id,t.name,t.age from (select id,name,age from test.test FETCH FIRST 5 ROWS ONLY offset 10) t;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select t.id,t.name,t.age from (select id,name,age from test.test limit 1000) t;", newTestResults(), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "select t.id,t.name,t.age from (select id,name,age from test.test FETCH FIRST 5 ROWS ONLY) t;", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId65, t, "insert into test.test(id,name,age) select id,name,age from test.test limit 1000 offset 10;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "insert into test.test(id,name,age) select id,name,age from test.test FETCH FIRST 5 ROWS ONLY offset 10;", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "insert into test.test(id,name,age) select id,name,age from test.test limit 1000;", newTestResults(), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "insert into test.test(id,name,age) select id,name,age from test.test FETCH FIRST 5 ROWS ONLY;", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId65, t, "update test.test set name = 'test' where id in (select id from test.test limit 1000 offset 10);", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "update test.test set name = 'test' where id in (select id from test.test FETCH FIRST 5 ROWS ONLY offset 10);", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "update test.test set name = 'test' where id in (select id from test.test limit 1000);", newTestResults(), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "update test.test set name = 'test' where id in (select id from test.test FETCH FIRST 5 ROWS ONLY);", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId65, t, "delete from test.test where id in (select id from test.test limit 1000 offset 10);", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "delete from test.test where id in (select id from test.test FETCH FIRST 5 ROWS ONLY offset 10);", newTestResults().add(RuleId65), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "delete from test.test where id in (select id from test.test limit 1000);", newTestResults(), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId65, t, "delete from test.test where id in (select id from test.test FETCH FIRST 5 ROWS ONLY);", newTestResults(), pgContext, make([]string, 0), []string{"test"})
}

func TestDriverImpl_Rule66(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	schemaInfoMap := make(map[string]*SchemaInfo)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "name",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "modified_time",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id1",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id2",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id3",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id4",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id5",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id6",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id7",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id8",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id9",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id10",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id11",
				TableName:  "test",
				OwnerName:  "public",
			},
			{
				ColumnName: "foreign_id12",
				TableName:  "test",
				OwnerName:  "public",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test_test",
		OwnerName: "public",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "name",
				TableName:  "test_test",
				OwnerName:  "public",
			},
			{
				ColumnName: "modified_time",
				TableName:  "test_test",
				OwnerName:  "public",
			},
		},
	})
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
		IndexInfoList: []*IndexInfo{
			{
				IndexName:  "idX_29_1",
				TableName:  "test",
				OwnerName:  "public",
				ColumnList: []string{"name"},
			},
			{
				IndexName:  "idX_29_3",
				TableName:  "test",
				OwnerName:  "public",
				ColumnList: []string{"name"},
			},
			{
				IndexName:  "idX_29_4",
				TableName:  "test",
				OwnerName:  "public",
				ColumnList: []string{"name"},
			},
		},
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
		IndexInfoList: []*IndexInfo{
			{
				IndexName:  "idx_29_1",
				TableName:  "test",
				OwnerName:  "test",
				ColumnList: []string{"name"},
			},
			{
				IndexName:  "idx_29_3",
				TableName:  "test",
				OwnerName:  "test",
				ColumnList: []string{"name"},
			},
			{
				IndexName:  "idx_29_4",
				TableName:  "test",
				OwnerName:  "test",
				ColumnList: []string{"name"},
			},
		},
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	testSingleSqlAudit(RuleId66, t, `create schema Test11;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create schema test11111;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create table Test1111118(id int);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create table test1111110(id int);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create table test.Test11111111(id int);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create table Test.test111111111(id int);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create table test.test1111119(id int);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create table test1111111(Id int);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create table test11111100(id int);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create table Test_alias as select * from test;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create table test_alias as select * from test;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `CREATE TEMPORARY TABLE tmp1Table (id SERIAL PRIMARY KEY, name VARCHAR(50), email VARCHAR(50));`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `CREATE TEMPORARY TABLE tmp1_table_1 (id SERIAL PRIMARY KEY, name VARCHAR(50), email VARCHAR(50));`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter table test add column Id int;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter table test add column id1 int;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter table test.test add column Id2 int;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter table test.test add column id3 int;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter table test_test rename to newTableName;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter table newTableName rename to new_table_name;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter table test rename column modified_time to newColumnName;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter table test rename column newColumnName to new_column_name;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter table test add constraint uniConstraintForeignId unique(foreign_id);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter table test add constraint uni_constraint_foreign_id unique(foreign_id);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter table test.test add constraint uniConstraintForeignId unique(foreign_id);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter table test.test add constraint uni_constraint_foreign_id unique(foreign_id);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create sequence sequenceName start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create sequence sequence_name start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create sequence Test.sequence_name start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create sequence test.sequence_name start with 1 increment by 1 minvalue 1 maxvalue 999999999 cycle;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter sequence sequence_name rename to newSequenceName;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter sequence sequence_name rename to new_sequence_name;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter sequence Test.sequence_name rename to newSequenceName;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter sequence test.sequence_name rename to new_sequence_name;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter sequence sequence_name rename to newSequenceName;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter sequence sequence_name rename to new_sequence_name;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `alter sequence test.sequence_name rename to newSequenceName;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `alter sequence test.sequence_name rename to new_sequence_name;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create index idxForeignId on test(foreign_id12);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id on test(foreign_id1);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create index idxForeignId2 on test.test(foreign_id2);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id2 on test.test(foreign_id3);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id3 on Test.test(foreign_id4);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id4 on test.test(foreign_id5);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id5 on test.Test(foreign_id6);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id6 on test.test(foreign_id7);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id7 on test(foreignId8);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id8 on test(foreign_id9);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id9 on test.test(foreignId10);`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create index idx_foreign_id10 on test.test(foreign_id11);`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER INDEX idx_29_1 RENAME TO Idx_29_2;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER INDEX Idx_29_2 RENAME TO idx_29_1;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER INDEX test.idx_29_1 RENAME TO Idx_29_2;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER INDEX test.Idx_29_2 RENAME TO idx_29_1;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER INDEX idx_29_3 SET TABLESPACE newTablespace;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER INDEX idx_29_4 SET TABLESPACE new_tablespace;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER INDEX test.idx_29_3 SET TABLESPACE newTablespace;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER INDEX test.idx_29_4 SET TABLESPACE new_tablespace;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER INDEX idx_29_3 OWNER TO Postgres;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER INDEX idx_29_4 OWNER TO postgres;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER INDEX test.idx_29_3 OWNER TO Postgres;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER INDEX test.idx_29_4 OWNER TO postgres;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER TABLE test OWNER TO Postgres;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER TABLE test OWNER TO postgres;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `ALTER TABLE test.test OWNER TO Postgres;`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `ALTER TABLE test.test OWNER TO postgres;`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `create trigger triggerTest after insert or update on test for each row execute function log_test_changes();`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `create trigger trigger_test after insert or update on test for each row execute function log_test_changes();`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `CREATE TABLESPACE tablespaceName OWNER user_name LOCATION 'directory_path';`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `CREATE TABLESPACE tablespace_name OWNER user_name LOCATION 'directory_path';`, newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId66, t, `CREATE TABLESPACE tablespace_name OWNER userName LOCATION 'directory_path';`, newTestResults().add(RuleId66), pgContext, nil, nil)
	testSingleSqlAudit(RuleId66, t, `CREATE TABLESPACE tablespace_name OWNER user_name LOCATION 'directory_path';`, newTestResults(), pgContext, nil, nil)

	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId66)
		testAuditWithDbQueryMockConn(RuleId66, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId66, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"viewname": sql.NullString{
				String: "view_test",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT viewname FROM pg_catalog.pg_views where schemaname = $1",
		args:     []string{"test"},
		fields:   []string{"viewname"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlIncorrect("create view viewTest as select * from test;", &dbOperates)
	assertSqlIncorrect("create view test.viewTest as select * from test;", &dbOperates)
	assertSqlIncorrect("create or replace view viewTest as select * from test;", &dbOperates)
	assertSqlIncorrect("create or replace view test.viewTest as select * from test;", &dbOperates)
	assertSqlCorrect("create or replace view view_test as select * from test;", &dbOperates)
	assertSqlCorrect("create or replace view test.view_test as select * from test;", &dbOperates)

	mockData = []map[string]sql.NullString{
		{
			"specific_schema": sql.NullString{
				String: "test",
				Valid:  true,
			},
			"specific_name": sql.NullString{
				String: "calculate_average_24972",
				Valid:  true,
			},
			"parameters": sql.NullString{
				String: "double precision,date",
				Valid:  true,
			},
		},
	}

	dbOperates = make([]*mockDbOperate, 0)
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT r.routine_schema as specific_schema, r.routine_name as specific_name, string_agg(p.data_type, ', ' ORDER BY p.ordinal_position) AS parameters FROM information_schema.routines r JOIN information_schema.parameters p ON r.specific_catalog = p.specific_catalog AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name WHERE r.routine_type = 'FUNCTION' AND r.routine_catalog = $1 AND r.routine_schema = $2 AND r.routine_name = $3 AND p.specific_schema NOT IN ('pg_catalog', 'information_schema') AND p.parameter_mode = 'IN' GROUP BY r.routine_schema, r.routine_name",
		fields:   []string{"specific_schema", "specific_name", "parameters"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrect("create function calculateTotalPrice(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlIncorrect("create or replace function calculateTotalPrice(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlCorrect("create or replace function calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlIncorrect("create function Test.calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlIncorrect("create function calculate_total_price(Quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlIncorrect("create function test.calculate_total_price(Quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlIncorrect("create or replace function Test.calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlIncorrect("create or replace function calculate_total_price(Quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlIncorrect("create or replace function test.calculate_total_price(Quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlCorrect("create or replace function test.calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlCorrect("create or replace function calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)
	assertSqlCorrect("create or replace function test.calculate_total_price(quantity integer, price numeric) returns numeric as $$ begin return quantity * price;end;$$ language plpgsql;", &dbOperates)

	mockData = []map[string]sql.NullString{
		{
			"specific_schema": sql.NullString{
				String: "test",
				Valid:  true,
			},
			"specific_name": sql.NullString{
				String: "get_employee_24974",
				Valid:  true,
			},
			"parameters": sql.NullString{
				String: "integer,character varying",
				Valid:  true,
			},
			"parameters_mode": sql.NullString{
				String: "IN,OUT",
				Valid:  true,
			},
		},
	}

	dbOperates = make([]*mockDbOperate, 0)
	mockDbOperateObj = &mockDbOperate{
		sql: `SELECT r.routine_schema as specific_schema, 
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
		GROUP BY r.routine_schema, r.routine_name`,
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrect("create procedure auditTestChanges(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlIncorrect("create or replace procedure auditTestChanges(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlIncorrect("create procedure audit_test_changes(Id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlIncorrect("create procedure Test.audit_test_changes(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlIncorrect("create procedure test.audit_test_changes(Id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlIncorrect("create or replace procedure audit_test_changes(Id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlIncorrect("create or replace procedure Test.audit_test_changes(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlIncorrect("create or replace procedure test.audit_test_changes(Id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlCorrect("create or replace procedure audit_test_changes(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlCorrect("create or replace procedure audit_test_changes(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlCorrect("create or replace procedure test.audit_test_changes(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
	assertSqlCorrect("create or replace procedure test.audit_test_changes(id int,name varchar(100),INOUT msg text) language plpgsql as $$ BEGIN insert into test(id,name) VALUES(id,name);END;$$", &dbOperates)
}

func TestDriverImpl_Rule67(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
		IndexInfoList: []*IndexInfo{
			{
				IndexName:  "idx_test_name1",
				TableName:  "test2",
				OwnerName:  "test",
				ColumnList: []string{"name"},
				IsUnique:   true,
			},
			{
				IndexName:  "idx_test_name11",
				TableName:  "test2",
				OwnerName:  "test",
				ColumnList: []string{"name4"},
				IsUnique:   true,
			},
		},
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}

	rule := RuleHandlerMap[RuleId67].Rule
	rule.Params.SetParamValue("expect_fixed_prefix", "uniq_")
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId67, t, sql, newTestResults(), pgContext, nil, nil)
	}
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId67, t, sql, newTestResults().add(RuleId67, "uniq_"), pgContext, nil, nil)
	}

	assertSqlIncorrect(`CREATE TABLE test2 (
		id SERIAL PRIMARY KEY,
		name VARCHAR(50),
		name2 VARCHAR(50),
		name3 VARCHAR(50),
		name4 VARCHAR(50),
		email VARCHAR(100) UNIQUE,
		CONSTRAINT unique_name_email UNIQUE(name, email)
	);`)
	assertSqlIncorrect(`create UNIQUE index idx_test2_name2 on test2(name2);`)
	assertSqlIncorrect(`create UNIQUE index idx_test2_name3 on test.test2(name3);`)
	assertSqlCorrect(`CREATE TABLE test22 (
		id SERIAL PRIMARY KEY,
		name VARCHAR(50),
		name2 VARCHAR(50),
		name3 VARCHAR(50),
		email VARCHAR(100) UNIQUE,
		CONSTRAINT uniq_name_email UNIQUE(name, email)
	);`)
	assertSqlCorrect(`create UNIQUE index uniq_test22_name2 on test22(nam2);`)
	assertSqlCorrect(`create UNIQUE index uniq_test22_name3 on test.test22(name3);`)

	// 触发规则
	assertSqlIncorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId67, "uniq_")
		testAuditWithDbQueryMockConn(RuleId67, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId67, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	mockData := []map[string]sql.NullString{
		{
			"indexdef": sql.NullString{
				String: "CREATE UNIQUE INDEX idx_test_name1 ON test.test2 USING btree (name)",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT indexdef FROM pg_indexes WHERE schemaname = $1 AND indexname = $2;",
		args:     []string{"test", "idx_test_name1"},
		fields:   []string{"indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrectMock("ALTER INDEX test.idx_test_name1 RENAME TO idx_test_name2_11;", &dbOperates)
	assertSqlIncorrectMock("ALTER TABLE test.test2 ADD CONSTRAINT unique_constraint_name UNIQUE(name);", &dbOperates)
	dbOperates = make([]*mockDbOperate, 0)
	dbOperates = append(dbOperates, mockDbOperateObj)
	assertSqlCorrectMock("ALTER INDEX test.idx_test_name11 RENAME TO uniq_test_name2_111;", &dbOperates)
	assertSqlCorrectMock("ALTER TABLE test.test2 ADD CONSTRAINT uniq_constraint_name UNIQUE(name);", &dbOperates)
}

func TestDriverImpl_Rule68(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId68].Rule
	rule.Params.SetParamValue("max_insert_rows", "2")
	// 走规则
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId68, t, sql, newTestResults().add(RuleId68, 2), pgContext, nil, nil)
	}
	// 不走规则
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId68, t, sql, newTestResults(), pgContext, nil, nil)
	}

	assertSqlIncorrect(`insert into test.test(id,name) values (1,'aa'),(2,'bb'),(3,'cc');`)
	assertSqlCorrect(`insert into test.test(id,name) values (1,'aa'),(2,'bb');`)
}

func TestDriverImpl_Rule69(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "full_index_table",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "full_index_table",
			},
			{
				ColumnName: "name",
				TableName:  "full_index_table",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{
		UsingType: UsingTypeOnline,
		DatabaseInfo: &DatabaseInfo{
			DatabaseName:  "test",
			CurrentSchema: "test",
			SchemaInfoMap: schemaInfoMap,
		},
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	assertSqlIncorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults().add(RuleId69)
		testAuditWithEpMockConn(RuleId69, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}
	assertSqlCorrect := func(sql string, mockEp epOutPut) {
		results := newTestResults()
		testAuditWithEpMockConn(RuleId69, t, []string{sql}, []*testResults{results}, &mockEp, pgContext, make([]string, 0), make([]string, 0))
	}
	// Incorrect
	sqlSmt := "SELECT * FROM test.full_index_table WHERE id = 100;"
	mockEp := epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
				"Plan": {
				  "Node Type": "Index Scan",
				  "Parallel Aware": false,
				  "Async Capable": false,
				  "Relation Name": "full_index_table",
				  "Alias": "full_index_table",
				  "Startup Cost": 0.00,
				  "Total Cost": 13.20,
				  "Plan Rows": 320,
				  "Plan Width": 222
				}
        }]`,
	}
	assertSqlIncorrect(sqlSmt, mockEp)

	// Correct
	sqlSmt = "SELECT * FROM test.full_index_table WHERE id > 100;"
	mockEp = epOutPut{
		ColumnName: "QUERY PLAN",
		Row: `[{
			"Plan": {
			  "Node Type": "Seq Scan",
			  "Parallel Aware": false,
			  "Async Capable": false,
			  "Scan Direction": "Forward",
			  "Index Name": "table1_pkey",
			  "Relation Name": "table1",
			  "Alias": "table1",
			  "Startup Cost": 0.15,
			  "Total Cost": 8.17,
			  "Plan Rows": 1,
			  "Plan Width": 222,
			  "Index Cond": "(id = 1)"
			}
		  }]`,
	}
	assertSqlCorrect(sqlSmt, mockEp)
}

func TestDriverImpl_Rule70(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId70].Rule
	rule.Params.SetParamValue("max_in_parameters_number", "5")
	// 走规则
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId70, t, sql, newTestResults().add(RuleId70, 5), pgContext, make([]string, 0), make([]string, 0))
	}
	// 不走规则
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId70, t, sql, newTestResults(), pgContext, make([]string, 0), make([]string, 0))
	}

	assertSqlIncorrect(`select id from test.test where id in(1,2,3,4,5,6)`)
	assertSqlCorrect(`select id from test.test where id in(1,2,3,4,5)`)

	assertSqlIncorrect(`select (select id from test.test where id in(1,2,3,4,5,6)) id from test.test where id > 0`)
	assertSqlCorrect(`select (select id from test.test where id in(1,2,3,4,5)) id from test.test where id > 0`)

	assertSqlIncorrect(`select t.id from (select id from test.test where id in(1,2,3,4,5,6)) t where t.id > 0`)
	assertSqlCorrect(`select t.id from (select id from test.test where id in(1,2,3,4,5)) t where t.id > 0`)

	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test where id in(1,2,3,4,5,6)`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test where id in(1,2,3,4,5)`)

	assertSqlIncorrect(`update test.test set name='test' where id in(1,2,3,4,5,6)`)
	assertSqlCorrect(`update test.test set name='test' where id in(1,2,3,4,5)`)

	assertSqlIncorrect(`delete from test.test where id in(1,2,3,4,5,6)`)
	assertSqlCorrect(`delete from test.test where id in(1,2,3,4,5)`)
}

func TestDriverImpl_Rule71(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId71, t, "select id from test.test where id > 0 union select id from test.test where id > 0", newTestResults().add(RuleId71), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId71, t, "select id from test.test where id > 0 union all select id from test.test where id > 0", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId71, t, "insert into test.test(id) (select id from test.test where id > 0 union select id from test.test where id > 0)", newTestResults().add(RuleId71), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId71, t, "insert into test.test(id) (select id from test.test where id > 0 union all select id from test.test where id > 0)", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId71, t, "update test.test set name = 'test' where id in (select id from test.test where id > 0 union select id from test.test where id > 0)", newTestResults().add(RuleId71), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId71, t, "update test.test set name = 'test' where id in (select id from test.test where id > 0 union all select id from test.test where id > 0)", newTestResults(), pgContext, make([]string, 0), []string{"test"})

	testSingleSqlAudit(RuleId71, t, "delete from test.test where id in (select id from test.test where id > 0 union select id from test.test where id > 0)", newTestResults().add(RuleId71), pgContext, make([]string, 0), []string{"test"})
	testSingleSqlAudit(RuleId71, t, "delete from test.test where id in (select id from test.test where id > 0 union all select id from test.test where id > 0)", newTestResults(), pgContext, make([]string, 0), []string{"test"})
}

func TestDriverImpl_Rule72(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId72].Rule
	rule.Params.SetParamValue("specified_functions_set", "sha1,sha256,sqrt,md5")
	// 走规则
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId72, t, sql, newTestResults().add(RuleId72, "sha1,sha256,sqrt,md5"), pgContext, make([]string, 0), make([]string, 0))
	}
	// 不走规则
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId72, t, sql, newTestResults(), pgContext, make([]string, 0), make([]string, 0))
	}

	assertSqlIncorrect(`select sqrt(id) from test.test;`)
	assertSqlCorrect(`select id from test.test;`)

	assertSqlIncorrect(`select sqrt(abs(id)) from test.test;`)
	assertSqlCorrect(`select id from test.test;`)

	assertSqlIncorrect(`select id from test.test where name = md5(name);`)
	assertSqlCorrect(`select id from test.test where name = 'test';`)

	assertSqlIncorrect(`select id from test.test where abs(sqrt(id)) > 0;`)
	assertSqlCorrect(`select id from test.test where id > 0;`)

	assertSqlIncorrect(`select id from test.test where name = 'test' group by id having sqrt(id) > 0;`)
	assertSqlCorrect(`select id from test.test where name = 'test' group by id having id > 0;`)

	assertSqlIncorrect(`select id from test.test where name = 'test' group by id having abs(sqrt(id)) > 0;`)
	assertSqlCorrect(`select id from test.test where name = 'test' group by id having abs(id) > 0;`)

	assertSqlIncorrect(`select id from test.test where name = 'test' group by id,name order by sqrt(id);`)
	assertSqlCorrect(`select id from test.test where name = 'test' group by id,name order by id;`)

	assertSqlIncorrect(`select id from test.test where name = 'test' group by id,name order by abs(sqrt(id));`)
	assertSqlCorrect(`select id from test.test where name = 'test' group by id,name order by abs(id);`)

	assertSqlIncorrect(`select t.id from (select id,name from test.test where name = md5(name)) t where t.name = 'test';`)
	assertSqlCorrect(`select t.id from (select id,name from test.test) t where t.name = 'test';`)

	assertSqlIncorrect(`insert into test.test(id) (select sqrt(id) from test.test);`)
	assertSqlCorrect(`insert into test.test(id) (select id from test.test);`)

	assertSqlIncorrect(`update test.test set name=md5('test');`)
	assertSqlCorrect(`update test.test set name='test';`)

	assertSqlIncorrect(`update test.test set name='test' where id in(select sqrt(id) from test.test);`)
	assertSqlCorrect(`update test.test set name='test' where id in(select id from test.test);`)

	assertSqlIncorrect(`delete from test.test where id in(select sqrt(id) from test.test);`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test);`)
}

func TestDriverImpl_Rule73(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId73].Rule
	rule.Params.SetParamValue("max_join_number", "2")
	// 走规则
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId73, t, sql, newTestResults().add(RuleId73, 2), pgContext, make([]string, 0), make([]string, 0))
	}
	// 不走规则
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId73, t, sql, newTestResults(), pgContext, make([]string, 0), make([]string, 0))
	}

	assertSqlIncorrect(`select x.id from test.test x join test.test y on x.id = y.id join test.test z on y.id = z.id;`)
	assertSqlCorrect(`select x.id from test.test x join test.test y on x.id = y.id;`)

	assertSqlIncorrect(`select t.id from test.test t where id in(select x.id from test.test x join test.test y on x.id = y.id join test.test z on y.id = z.id);`)
	assertSqlCorrect(`select t.id from test.test t where id in(select x.id from test.test x join test.test y on x.id = y.id);`)

	assertSqlIncorrect(`insert into test.test(id) (select x.id from test.test x join test.test y on x.id = y.id join test.test z on y.id = z.id);`)
	assertSqlCorrect(`insert into test.test(id) (select x.id from test.test x join test.test y on x.id = y.id);`)

	assertSqlIncorrect(`update test.test set name = 'test' where id in(select x.id from test.test x join test.test y on x.id = y.id join test.test z on y.id = z.id);`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select x.id from test.test x join test.test y on x.id = y.id);`)

	assertSqlIncorrect(`delete from test.test where id in(select x.id from test.test x join test.test y on x.id = y.id join test.test z on y.id = z.id);`)
	assertSqlCorrect(`delete from test.test where id in(select x.id from test.test x join test.test y on x.id = y.id);`)
}

func TestDriverImpl_Rule74(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "long_char_table",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "long_char_table",
			},
			{
				ColumnName:   "long_column",
				TableName:    "long_char_table",
				ColumnLength: 3000,
				ColumnType:   "character varying",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId74].Rule
	rule.Params.SetParamValue("expect_long_field_length", "2000")
	// 触发规则
	assertSqlIncorrectMock := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId74)
		testAuditWithDbQueryMockConn(RuleId74, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	mockData := []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "long_char_table",
				Valid:  true,
			},
		},
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockDbOperateObj := &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "table1",
				Valid:  true,
			},
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "32",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"table_name": sql.NullString{
				String: "table2",
				Valid:  true,
			},
			"column_name": sql.NullString{
				String: "long_column",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "3000",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select table_name, column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1",
		fields:   []string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = append(mockData, map[string]sql.NullString{
		"table_name": {
			String: "table2",
			Valid:  true,
		},
		"column_name": {
			String: "column1",
			Valid:  true,
		},
		"data_type": {
			String: "numeric",
			Valid:  true,
		},
		"character_set_name": {
			String: "",
			Valid:  true,
		},
		"is_nullable": {
			String: "YES",
			Valid:  true,
		},
		"column_default": {
			String: "",
			Valid:  true,
		},
	})
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.relname AS table_name, c.conname AS constraint_name, c.contype AS constraint_type, CASE WHEN c.contype IN ('p', 'u') THEN string_agg(a.attname, ',') WHEN c.contype = 'f' THEN string_agg(a.attname, ',') ELSE null END as column_names, CASE WHEN c.contype = 'f' THEN c.confrelid::regclass ELSE null END as referenced_table, CASE WHEN c.contype = 'f' THEN string_agg(ac.attname, ',') ELSE null END as referenced_columns, CASE WHEN c.contype = 'c' THEN pg_get_constraintdef(c.oid) ELSE null END as check_condition FROM pg_constraint c JOIN pg_class t ON c.conrelid = t.oid JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey) JOIN pg_namespace n ON t.relnamespace = n.oid LEFT JOIN pg_class rc ON c.confrelid = rc.oid LEFT JOIN pg_attribute ac ON ac.attrelid = rc.oid AND ac.attnum = ANY (c.confkey) WHERE n.nspname = $1 GROUP BY t.relname, c.conname, c.contype, c.confrelid, c.oid HAVING constraint_type IN('p', 'u', 'f', 'c')",
		fields:   []string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"tablename": {
				String: "table2",
				Valid:  true,
			},
			"indexname": sql.NullString{
				String: "",
				Valid:  true,
			},
			"indexdef": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT tablename,indexname,indexdef FROM pg_indexes where schemaname = $1",
		fields:   []string{"tablename", "indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "table1",
				Valid:  true,
			},
		},
		{
			"table_name": sql.NullString{
				String: "table2",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		args:     []string{"test"},
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "table1",
				Valid:  true,
			},
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"table_name": sql.NullString{
				String: "table1",
				Valid:  true,
			},
			"column_name": sql.NullString{
				String: "name",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"table_name": sql.NullString{
				String: "table2",
				Valid:  true,
			},
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "NO",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"table_name": sql.NullString{
				String: "table2",
				Valid:  true,
			},
			"column_name": sql.NullString{
				String: "name",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select table_name, column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1",
		fields:   []string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = append(mockData, map[string]sql.NullString{
		"table_name": {
			String: "table2",
			Valid:  true,
		},
		"column_name": {
			String: "column1",
			Valid:  true,
		},
		"data_type": {
			String: "numeric",
			Valid:  true,
		},
		"character_set_name": {
			String: "",
			Valid:  true,
		},
		"is_nullable": {
			String: "YES",
			Valid:  true,
		},
		"column_default": {
			String: "",
			Valid:  true,
		},
	})
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT t.relname AS table_name, c.conname AS constraint_name, c.contype AS constraint_type, CASE WHEN c.contype IN ('p', 'u') THEN string_agg(a.attname, ',') WHEN c.contype = 'f' THEN string_agg(a.attname, ',') ELSE null END as column_names, CASE WHEN c.contype = 'f' THEN c.confrelid::regclass ELSE null END as referenced_table, CASE WHEN c.contype = 'f' THEN string_agg(ac.attname, ',') ELSE null END as referenced_columns, CASE WHEN c.contype = 'c' THEN pg_get_constraintdef(c.oid) ELSE null END as check_condition FROM pg_constraint c JOIN pg_class t ON c.conrelid = t.oid JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey) JOIN pg_namespace n ON t.relnamespace = n.oid LEFT JOIN pg_class rc ON c.confrelid = rc.oid LEFT JOIN pg_attribute ac ON ac.attrelid = rc.oid AND ac.attnum = ANY (c.confkey) WHERE n.nspname = $1 GROUP BY t.relname, c.conname, c.contype, c.confrelid, c.oid HAVING constraint_type IN('p', 'u', 'f', 'c')",
		fields:   []string{"table_name", "column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"table_name": sql.NullString{
				String: "table1",
				Valid:  true,
			},
		},
		{
			"table_name": sql.NullString{
				String: "table2",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
		args:     []string{"test"},
		fields:   []string{"table_name"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = append(mockData, map[string]sql.NullString{
		"tablename": {
			String: "table2",
			Valid:  true,
		},
		"indexname": {
			String: "",
			Valid:  true,
		},
		"indexdef": {
			String: "",
			Valid:  true,
		},
	})
	mockDbOperateObj = &mockDbOperate{
		sql:      "SELECT tablename,indexname,indexdef FROM pg_indexes where schemaname = $1",
		args:     []string{"test"},
		fields:   []string{"tablename", "indexname", "indexdef"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	mockData = []map[string]sql.NullString{
		{
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "32",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "long_column",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "3000",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj = &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		args:     []string{"test", "table1"},
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockDbOperateObj = &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		args:     []string{"test", "table2"},
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrectMock("select distinct long_column from long_char_table;", &dbOperates)
	assertSqlIncorrectMock("select id, long_column from long_char_table group by id,long_column;", &dbOperates)
	assertSqlIncorrectMock("select id,long_column from long_char_table order by long_column;", &dbOperates)
	assertSqlIncorrectMock("(select id,long_column from long_char_table order by long_column) union (select id,long_column from long_char_table order by long_column);", &dbOperates)
	assertSqlIncorrectMock("insert into long_char_table(id) select t.id from ((select id from long_char_table order by id) union (select id from long_char_table order by long_column)) t;", &dbOperates)
	assertSqlIncorrectMock("update long_char_table set long_column = 'test' where id in (select id from long_char_table order by long_column);", &dbOperates)
	assertSqlIncorrectMock("delete from long_char_table where id in (select id from long_char_table order by long_column);", &dbOperates)
}

func TestDriverImpl_Rule75(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
			{
				ColumnName: "age",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId75].Rule
	rule.Params.SetParamValue("max_rows", "1000")
	// 走规则
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId75, t, sql, newTestResults().add(RuleId75, 1000), pgContext, make([]string, 0), make([]string, 0))
	}
	// 不走规则
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId75, t, sql, newTestResults(), pgContext, make([]string, 0), make([]string, 0))
	}

	assertSqlIncorrect(`select id from test.test`)
	assertSqlCorrect(`select id from test.test limit 10`)
	assertSqlCorrect(`select id from test.test FETCH FIRST 10 ROWS ONLY`)

	assertSqlIncorrect(`select id from test.test limit 1001`)
	assertSqlIncorrect(`select id from test.test FETCH FIRST 1001 ROWS ONLY`)
	assertSqlCorrect(`select id from test.test limit 1000`)
	assertSqlCorrect(`select id from test.test FETCH FIRST 1000 ROWS ONLY`)

	assertSqlIncorrect(`select id from test.test limit 1001 offset 10`)
	assertSqlIncorrect(`select id from test.test FETCH FIRST 1001 ROWS ONLY offset 10`)
	assertSqlCorrect(`select id from test.test limit 1000 offset 10`)
	assertSqlCorrect(`select id from test.test FETCH FIRST 1000 ROWS ONLY offset 10`)

	assertSqlIncorrect(`select id from test.test offset 10 limit 1001`)
	assertSqlIncorrect(`select id from test.test offset 10 FETCH FIRST 1001 ROWS ONLY`)
	assertSqlCorrect(`select id from test.test offset 10 limit 1000`)
	assertSqlCorrect(`select id from test.test offset 10 FETCH FIRST 1000 ROWS ONLY`)

	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test limit 10`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test FETCH FIRST 10 ROWS ONLY`)

	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test limit 1001`)
	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test FETCH FIRST 1001 ROWS ONLY`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test limit 1000`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test FETCH FIRST 1000 ROWS ONLY`)

	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test limit 1001 offset 10`)
	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test FETCH FIRST 1001 ROWS ONLY offset 10`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test limit 1000 offset 10`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test FETCH FIRST 1000 ROWS ONLY offset 10`)

	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test offset 10 limit 1001`)
	assertSqlIncorrect(`insert into test.test(id,name,age) select id,name,age from test.test offset 10 FETCH FIRST 1001 ROWS ONLY`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test offset 10 limit 1000`)
	assertSqlCorrect(`insert into test.test(id,name,age) select id,name,age from test.test offset 10 FETCH FIRST 1000 ROWS ONLY`)
	// 不触发规则
	assertSqlCorrect(`insert into test.test(id,name,age) values(1,'aa',100);`)

	assertSqlIncorrect(`update test.test set name = 'test' where id in(select id from test.test)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test limit 10)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test FETCH FIRST 10 ROWS ONLY)`)

	assertSqlIncorrect(`update test.test set name = 'test' where id in(select id from test.test limit 1001)`)
	assertSqlIncorrect(`update test.test set name = 'test' where id in(select id from test.test FETCH FIRST 1001 ROWS ONLY)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test limit 1000)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test FETCH FIRST 1000 ROWS ONLY)`)

	assertSqlIncorrect(`update test.test set name = 'test' where id in(select id from test.test limit 1001 offset 10)`)
	assertSqlIncorrect(`update test.test set name = 'test' where id in(select id from test.test FETCH FIRST 1001 ROWS ONLY offset 10)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test limit 1000 offset 10)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test FETCH FIRST 1000 ROWS ONLY offset 10)`)

	assertSqlIncorrect(`update test.test set name = 'test' where id in(select id from test.test offset 10 limit 1001)`)
	assertSqlIncorrect(`update test.test set name = 'test' where id in(select id from test.test offset 10 FETCH FIRST 1001 ROWS ONLY)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test offset 10 limit 1000)`)
	assertSqlCorrect(`update test.test set name = 'test' where id in(select id from test.test offset 10 FETCH FIRST 1000 ROWS ONLY)`)
	// 没有select子查询不走规则
	assertSqlCorrect(`update test.test set name = 'test'`)
	assertSqlCorrect(`update test.test set name = 'test' where id > 0`)

	assertSqlIncorrect(`delete from test.test where id in(select id from test.test)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test limit 10)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test FETCH FIRST 10 ROWS ONLY)`)

	assertSqlIncorrect(`delete from test.test where id in(select id from test.test limit 1001)`)
	assertSqlIncorrect(`delete from test.test where id in(select id from test.test FETCH FIRST 1001 ROWS ONLY)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test limit 1000)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test FETCH FIRST 1000 ROWS ONLY)`)

	assertSqlIncorrect(`delete from test.test where id in(select id from test.test limit 1001 offset 10)`)
	assertSqlIncorrect(`delete from test.test where id in(select id from test.test FETCH FIRST 1001 ROWS ONLY offset 10)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test limit 1000 offset 10)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test FETCH FIRST 1000 ROWS ONLY offset 10)`)

	assertSqlIncorrect(`delete from test.test where id in(select id from test.test offset 10 limit 1001)`)
	assertSqlIncorrect(`delete from test.test where id in(select id from test.test offset 10 FETCH FIRST 1001 ROWS ONLY)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test offset 10 limit 1000)`)
	assertSqlCorrect(`delete from test.test where id in(select id from test.test offset 10 FETCH FIRST 1000 ROWS ONLY)`)
	// 没有select子查询不走规则
	assertSqlCorrect(`delete from test.test`)
	assertSqlCorrect(`delete from test.test where id > 0`)
}

func TestDriverImpl_Rule76(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "long_char_table",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "long_char_table",
			},
			{
				ColumnName: "name",
				TableName:  "long_char_table",
			},
			{
				ColumnName: "lang_column",
				TableName:  "long_char_table",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId76, t, "select * from test.long_char_table order by id,name desc;", newTestResults().add(RuleId76), pgContext, nil, nil)
	testSingleSqlAudit(RuleId76, t, "select * from test.long_char_table order by id,name;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId76, t, "select * from test.long_char_table order by id asc,name desc;", newTestResults().add(RuleId76), pgContext, nil, nil)
	testSingleSqlAudit(RuleId76, t, "select * from test.long_char_table order by id asc,name asc;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId76, t, "insert into test.long_char_table(id,lang_column) select id,lang_column from test.long_char_table order by id,name desc;", newTestResults().add(RuleId76), pgContext, nil, nil)
	testSingleSqlAudit(RuleId76, t, "insert into test.long_char_table(id,lang_column) select id,lang_column from test.long_char_table order by id,name;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId76, t, "update test.long_char_table set lang_column = '' where id in(select id from test.long_char_table order by id,name desc);", newTestResults().add(RuleId76), pgContext, nil, nil)
	testSingleSqlAudit(RuleId76, t, "update test.long_char_table set lang_column = '' where id in(select id from test.long_char_table order by id,name);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId76, t, "delete from test.long_char_table where id in (select id from test.long_char_table order by id,name desc);", newTestResults().add(RuleId76), pgContext, nil, nil)
	testSingleSqlAudit(RuleId76, t, "delete from test.long_char_table where id in (select id from test.long_char_table order by id,name);", newTestResults(), pgContext, nil, nil)
}

func TestDriverImpl_Rule77(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "test",
			},
		},
		OwnerName: "test",
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["public"] = &SchemaInfo{
		SchemaName:    "public",
		TableInfoList: tableInfoList,
	}
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	rule := RuleHandlerMap[RuleId77].Rule
	rule.Params.SetParamValue("expected_subQuery_nesting_layers", "3")
	assertSqlCorrect := func(sql string) {
		testSingleSqlAudit(RuleId77, t, sql, newTestResults(), pgContext, nil, nil)
	}
	assertSqlIncorrect := func(sql string) {
		testSingleSqlAudit(RuleId77, t, sql, newTestResults().add(RuleId77, 3), pgContext, nil, nil)
	}

	assertSqlIncorrect("select (select (select (select (select id from test.test limit 1))) from test.test limit 1) from test.test")
	assertSqlCorrect("select (select(select (select id from test.test limit 1) from test.test limit 1)) from test.test")

	assertSqlIncorrect("select t.id from (select (select (select (select id from test.test limit 1)) from test.test limit 1) from test.test) t")
	assertSqlCorrect("select t.id from (select (select (select id from test.test limit 1) from test.test limit 1) from test.test) t")

	assertSqlIncorrect("select t.id from test.test t where t.id in (select (select (select (select id from test.test limit 1)) from test.test limit 1) from test.test)")
	assertSqlCorrect("select t.id from test.test t where t.id in (select (select (select id from test.test limit 1) from test.test limit 1) from test.test)")

	assertSqlIncorrect("insert into test.test(id) (select (select (select (select id from test.test limit 1)) from test.test limit 1) from test.test)")
	assertSqlCorrect("insert into test.test(id) (select (select (select id from test.test limit 1) from test.test limit 1) from test.test)")

	assertSqlIncorrect("update test.test set name = 'test' where id in(select (select (select (select id from test.test limit 1)) from test.test limit 1) from test.test)")
	assertSqlCorrect("update test.test set name = 'test' where id in(select (select (select id from test.test limit 1) from test.test limit 1) from test.test)")

	assertSqlIncorrect("delete from test.test where id in(select (select (select (select id from test.test limit 1)) from test.test limit 1) from test.test)")
	assertSqlCorrect("delete from test.test where id in(select (select (select id from test.test limit 1) from test.test limit 1) from test.test)")
}

func TestDriverImpl_Rule78(t *testing.T) {
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table1",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table1",
				OwnerName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "table1",
				OwnerName:  "test",
			},
		},
	})
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName: "table2",
		OwnerName: "test",
		ColumnInfoList: []*ColumnInfo{
			{
				ColumnName: "id",
				TableName:  "table2",
				OwnerName:  "test",
			},
			{
				ColumnName: "name",
				TableName:  "table2",
				OwnerName:  "test",
			},
		},
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "postgres",
		CurrentSchema: "public",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	// 触发规则
	assertSqlIncorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults().add(RuleId78)
		testAuditWithDbQueryMockConn(RuleId78, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}
	// 不触发规则
	assertSqlCorrect := func(sql string, dbOperates *[]*mockDbOperate) {
		results := newTestResults()
		testAuditWithDbQueryMockConn(RuleId78, t, []string{sql}, []*testResults{results}, dbOperates, pgContext)
	}

	dbOperates := make([]*mockDbOperate, 0)
	mockData := []map[string]sql.NullString{
		{
			"column_name": sql.NullString{
				String: "id",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "integer",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "32",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "",
				Valid:  true,
			},
		},
		{
			"column_name": sql.NullString{
				String: "long_column",
				Valid:  true,
			},
			"data_type": sql.NullString{
				String: "character varying",
				Valid:  true,
			},
			"character_set_name": sql.NullString{
				String: "",
				Valid:  true,
			},
			"is_nullable": sql.NullString{
				String: "YES",
				Valid:  true,
			},
			"column_default": sql.NullString{
				String: "",
				Valid:  true,
			},
			"numeric_precision": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"numeric_scale": sql.NullString{
				String: "0",
				Valid:  true,
			},
			"character_maximum_length": sql.NullString{
				String: "3000",
				Valid:  true,
			},
		},
	}
	mockDbOperateObj := &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		args:     []string{"test", "table1"},
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)
	mockDbOperateObj = &mockDbOperate{
		sql:      "select column_name, data_type, character_set_name, is_nullable, column_default, numeric_precision, numeric_scale, character_maximum_length from information_schema.columns where table_schema = $1 and table_name = $2",
		args:     []string{"test", "table2"},
		fields:   []string{"column_name", "data_type", "character_set_name", "is_nullable", "column_default", "numeric_precision", "numeric_scale", "character_maximum_length"},
		mockData: mockData,
	}
	dbOperates = append(dbOperates, mockDbOperateObj)

	assertSqlIncorrect("select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1,test.table2 t2 where t1.id = '1';", &dbOperates)
	assertSqlIncorrect("select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1,test.table2 t2 where t1.name = 1;", &dbOperates)
	assertSqlIncorrect("select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1,test.table2 t2 where t1.name = 1.0;", &dbOperates)
	assertSqlIncorrect("select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1,test.table2 t2 where t1.id = t2.name;", &dbOperates)
	assertSqlCorrect("select t1.id id1,t1.name name1,t2.id id2,t2.name name2 from test.table1 t1,test.table2 t2 where t1.id = t2.id;", &dbOperates)
}

func TestDriverImpl_Rule79(t *testing.T) {
	columnInfoList := make([]*ColumnInfo, 0)
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "id",
		ColumnType: "integer",
		OwnerName:  "test",
	})
	columnInfoList = append(columnInfoList, &ColumnInfo{
		ColumnName: "lang_column",
		ColumnType: "varchar",
		OwnerName:  "test",
	})
	tableInfoList := make([]*TableInfo, 0)
	tableInfoList = append(tableInfoList, &TableInfo{
		TableName:      "long_char_table",
		OwnerName:      "test",
		ColumnInfoList: columnInfoList,
	})
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap["test"] = &SchemaInfo{
		SchemaName:    "test",
		TableInfoList: tableInfoList,
	}
	pgContext := &PgContext{UsingType: UsingTypeOffline, DatabaseInfo: &DatabaseInfo{
		DatabaseName:  "test",
		CurrentSchema: "test",
		SchemaInfoMap: schemaInfoMap,
	}, DeletedSchemaMap: make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	testSingleSqlAudit(RuleId79, t, "select * from test.long_char_table limit 100 OFFSET 10;", newTestResults().add(RuleId79), pgContext, nil, nil)
	testSingleSqlAudit(RuleId79, t, "select * from test.long_char_table order by id limit 100 OFFSET 10;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId79, t, "select * from test.long_char_table OFFSET 10 limit 100;", newTestResults().add(RuleId79), pgContext, nil, nil)
	testSingleSqlAudit(RuleId79, t, "select * from test.long_char_table order by id OFFSET 10 limit 100;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId79, t, "select * from test.long_char_table limit 100;", newTestResults().add(RuleId79), pgContext, nil, nil)
	testSingleSqlAudit(RuleId79, t, "select * from test.long_char_table order by id limit 100;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId79, t, "insert into test.long_char_table(id,lang_column) select id,lang_column from test.long_char_table limit 100;", newTestResults().add(RuleId79), pgContext, nil, nil)
	testSingleSqlAudit(RuleId79, t, "insert into test.long_char_table(id,lang_column) select id,lang_column from test.long_char_table order by id limit 100;", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId79, t, "update test.long_char_table set lang_column = '' where id in(select id from test.long_char_table limit 100);", newTestResults().add(RuleId79), pgContext, nil, nil)
	testSingleSqlAudit(RuleId79, t, "update test.long_char_table set lang_column = '' where id in(select id from test.long_char_table order by id limit 100);", newTestResults(), pgContext, nil, nil)

	testSingleSqlAudit(RuleId79, t, "delete from test.long_char_table where id in (select id from test.long_char_table limit 100);", newTestResults().add(RuleId79), pgContext, nil, nil)
	testSingleSqlAudit(RuleId79, t, "delete from test.long_char_table where id in (select id from test.long_char_table order by id limit 100);", newTestResults(), pgContext, nil, nil)
}
