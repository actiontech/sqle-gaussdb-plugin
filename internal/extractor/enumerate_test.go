// enumerate_test.go — 覆盖 Task-Test-Fix-001 P0 修复：
//
//   - listTablesSQL / listViewsSQL / listFunctionsSQL / listProceduresSQL 四条
//     枚举 SQL 字面值锁定（sqlmock.QueryMatcherEqual）；
//   - Extract 顶部"空 DatabaseObjects → 全枚举 + switch 分发"端到端路径；
//   - 多类型混合 + 顺序无关断言（sort.Strings；参考 expertise_docs/semantic/
//     sqlmock_rows_iteration_empty_and_order_20260521.md）；
//   - 任一枚举 SQL 失败 → 不 partial 返回，错误透传。
//
// 关注矩阵：
//
//   - TABLE 全枚举 → 走 extractTable 拿 DDL；
//   - VIEW 全枚举 → 走 extractView 拿 DDL；
//   - FUNCTION / PROCEDURE 全枚举 GROUP BY proname 去重，重载分裂留在 helper。

package extractor

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// TestExtract_EmptyObjects_FullEnumeration 端到端覆盖空 DatabaseObjects →
// 全枚举 → 4 类对象 DDL 抽取的完整链路。
func TestExtract_EmptyObjects_FullEnumeration(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	// 期望 enumerateSchemaObjects 内 4 条枚举 SQL 顺序触发。
	mock.ExpectQuery(listTablesSQL).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}).
			AddRow("t_a").
			AddRow("t_b"))

	mock.ExpectQuery(listViewsSQL).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}).
			AddRow("v_a"))

	mock.ExpectQuery(listFunctionsSQL).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"proname"}).
			AddRow("fn_a"))

	mock.ExpectQuery(listProceduresSQL).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"proname"}).
			AddRow("proc_a"))

	// 期望随后按 4 类对象顺序触发 extractTable / extractView / extractFunction /
	// extractProcedure 的 SQL。
	mock.ExpectQuery(extractTableSQL).
		WithArgs("public", "t_a").
		WillReturnRows(sqlmock.NewRows([]string{"pg_get_tabledef"}).
			AddRow("CREATE TABLE public.t_a (id int);"))
	mock.ExpectQuery(extractTableSQL).
		WithArgs("public", "t_b").
		WillReturnRows(sqlmock.NewRows([]string{"pg_get_tabledef"}).
			AddRow("CREATE TABLE public.t_b (id int);"))
	mock.ExpectQuery(extractViewSQL).
		WithArgs("public", "v_a").
		WillReturnRows(sqlmock.NewRows([]string{"ddl"}).
			AddRow("CREATE OR REPLACE VIEW public.v_a AS SELECT 1;"))
	// extractFunction / extractProcedure 在阶段 2 改成两次单列查询，此处 mock
	// 两组单列 SQL：先 defs，再 args。
	mock.ExpectQuery(extractFunctionDefSQL).
		WithArgs("public", "fn_a").
		WillReturnRows(sqlmock.NewRows([]string{"functiondef"}).
			AddRow("CREATE OR REPLACE FUNCTION public.fn_a() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql"))
	mock.ExpectQuery(extractFunctionArgsSQL).
		WithArgs("public", "fn_a").
		WillReturnRows(sqlmock.NewRows([]string{"arguments"}).
			AddRow(""))
	mock.ExpectQuery(extractProcedureDefSQL).
		WithArgs("public", "proc_a").
		WillReturnRows(sqlmock.NewRows([]string{"procdef"}).
			AddRow("CREATE OR REPLACE PROCEDURE public.proc_a() AS BEGIN NULL; END;"))
	mock.ExpectQuery(extractProcedureArgsSQL).
		WithArgs("public", "proc_a").
		WillReturnRows(sqlmock.NewRows([]string{"arguments"}).
			AddRow(""))

	ex := NewExtractor(db)
	result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
		SchemaName:      "public",
		DatabaseObjects: nil, // 空：触发全枚举兜底
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result == nil {
		t.Fatal("Extract returned nil result")
	}

	if result.SchemaName != "public" {
		t.Errorf("SchemaName = %q, want %q", result.SchemaName, "public")
	}

	// 顺序无关断言（map 迭代顺序 Go runtime 不保证；本测试是按 ORDER BY 顺序
	// + helper 串行迭代，理论上稳定但仍 defensive sort）。
	var gotNames []string
	for _, ddl := range result.DatabaseObjectDDLs {
		gotNames = append(gotNames, ddl.DatabaseObject.ObjectType+":"+ddl.DatabaseObject.ObjectName)
	}
	sort.Strings(gotNames)
	wantNames := []string{
		driverV2.ObjectType_FUNCTION + ":fn_a()",
		driverV2.ObjectType_PROCEDURE + ":proc_a()",
		driverV2.ObjectType_TABLE + ":t_a",
		driverV2.ObjectType_TABLE + ":t_b",
		driverV2.ObjectType_VIEW + ":v_a",
	}
	sort.Strings(wantNames)
	if len(gotNames) != len(wantNames) {
		t.Fatalf("got names = %v, want %v", gotNames, wantNames)
	}
	for i := range gotNames {
		if gotNames[i] != wantNames[i] {
			t.Errorf("got[%d] = %q, want %q", i, gotNames[i], wantNames[i])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestExtract_EmptyObjects_EmptySchema 覆盖 schema 内 0 对象的边界（4 条枚举
// SQL 都返回 0 行 → DatabaseObjectDDLs 为空 slice，不报错）。
func TestExtract_EmptyObjects_EmptySchema(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	emptyRows := func(col string) *sqlmock.Rows { return sqlmock.NewRows([]string{col}) }
	mock.ExpectQuery(listTablesSQL).WithArgs("empty").WillReturnRows(emptyRows("relname"))
	mock.ExpectQuery(listViewsSQL).WithArgs("empty").WillReturnRows(emptyRows("relname"))
	mock.ExpectQuery(listFunctionsSQL).WithArgs("empty").WillReturnRows(emptyRows("proname"))
	mock.ExpectQuery(listProceduresSQL).WithArgs("empty").WillReturnRows(emptyRows("proname"))

	ex := NewExtractor(db)
	result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
		SchemaName: "empty",
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result == nil {
		t.Fatal("Extract returned nil result")
	}
	if len(result.DatabaseObjectDDLs) != 0 {
		t.Errorf("DatabaseObjectDDLs len = %d, want 0", len(result.DatabaseObjectDDLs))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestExtract_EmptyObjects_EnumerateError 断言任一枚举 SQL 失败时错误透传，
// 不做 partial 返回。
func TestExtract_EmptyObjects_EnumerateError(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	mock.ExpectQuery(listTablesSQL).
		WithArgs("restricted").
		WillReturnError(errors.New("permission denied for schema pg_catalog"))

	ex := NewExtractor(db)
	result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
		SchemaName: "restricted",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %q, expected substring 'permission denied'", err.Error())
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
