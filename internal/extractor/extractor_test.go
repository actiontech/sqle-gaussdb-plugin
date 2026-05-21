// extractor_test.go covers design §17.1.2 — DDL Extractor 单测矩阵 第一轮：
//
//   - TABLE / VIEW 正常路径（SQL 模板字面匹配 + $1 / $2 参数绑定）
//   - 不支持 ObjectType 兜底（INDEX / TRIGGER / EVENT / SEQUENCE / FUNCTION /
//     PROCEDURE → default 分支，design §6.1 line 399 字面错误）
//   - 对象不存在 → sql.ErrNoRows 包装为空 DDL，不报错（design §6.5）
//   - catalog 权限错误 → 原样透传，不吞 SQLSTATE
//   - QuoteIdent 注入防御 → 空格 / 大小写 / 中文 / SQL 注入串 / 单引号 / 双引号
//     6 条 case，断言 sqlmock 收到的 $1 / $2 参数原值未被 Go 侧 escape
//
// FUNCTION / PROCEDURE 的 重载识别（design §6.4）+ 正常分发覆盖留给
// Task-Dev-006 (extractor_function_test.go / extractor_procedure_test.go)。
//
// 所有 sqlmock 用 QueryMatcherOption(QueryMatcherEqual) 模式锁定 SQL 文本，
// 避免正则模糊匹配掩盖 design 模板偏离。

package extractor

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// newMock 构造一个 sqlmock.QueryMatcherEqual 模式的 *sql.DB + sqlmock，并返回
// 清理函数；所有测试统一走该 helper 确保匹配策略一致。
func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
	)
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	return db, mock, func() { _ = db.Close() }
}

// TestExtractor_Extract_TableAndView 覆盖 Extract 入口的 TABLE / VIEW 分发与
// 对象不存在 / catalog 权限错误兜底（design §6.2.1 / §6.2.2 / §6.5）。
func TestExtractor_Extract_TableAndView(t *testing.T) {
	const (
		tableDDLNormal = "SET search_path = 'public'; CREATE TABLE public.t1 (id int) WITH (orientation=row);"
		viewDDLNormal  = "CREATE OR REPLACE VIEW public.v1 AS  SELECT 1 AS id;"
	)

	cases := map[string]struct {
		schema       string
		object       *driverV2.DatabaseObject
		expectSQL    string
		expectArgs   []driverV2.DatabaseObject // 仅用其字段做断言，不参与执行
		mockSetup    func(mock sqlmock.Sqlmock)
		wantErr      bool
		wantErrPart  string
		wantDDL      string
		wantObjType  string
		wantObjName  string
		wantDDLEmpty bool
	}{
		"table_normal": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "t1", ObjectType: driverV2.ObjectType_TABLE},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractTableSQL).
					WithArgs("public", "t1").
					WillReturnRows(sqlmock.NewRows([]string{"pg_get_tabledef"}).AddRow(tableDDLNormal))
			},
			wantDDL:     tableDDLNormal,
			wantObjType: driverV2.ObjectType_TABLE,
			wantObjName: "t1",
		},
		"view_normal": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "v1", ObjectType: driverV2.ObjectType_VIEW},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractViewSQL).
					WithArgs("public", "v1").
					WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow(viewDDLNormal))
			},
			wantDDL:     viewDDLNormal,
			wantObjType: driverV2.ObjectType_VIEW,
			wantObjName: "v1",
		},
		"table_not_found": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "missing_table", ObjectType: driverV2.ObjectType_TABLE},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractTableSQL).
					WithArgs("public", "missing_table").
					WillReturnError(sql.ErrNoRows)
			},
			wantDDLEmpty: true,
			wantObjType:  driverV2.ObjectType_TABLE,
			wantObjName:  "missing_table",
		},
		"view_not_found": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "missing_view", ObjectType: driverV2.ObjectType_VIEW},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractViewSQL).
					WithArgs("public", "missing_view").
					WillReturnError(sql.ErrNoRows)
			},
			wantDDLEmpty: true,
			wantObjType:  driverV2.ObjectType_VIEW,
			wantObjName:  "missing_view",
		},
		"table_permission_denied": {
			schema: "restricted",
			object: &driverV2.DatabaseObject{ObjectName: "t1", ObjectType: driverV2.ObjectType_TABLE},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractTableSQL).
					WithArgs("restricted", "t1").
					WillReturnError(errors.New("permission denied for schema pg_catalog"))
			},
			wantErr:     true,
			wantErrPart: "permission denied",
		},
		"view_permission_denied": {
			schema: "restricted",
			object: &driverV2.DatabaseObject{ObjectName: "v1", ObjectType: driverV2.ObjectType_VIEW},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractViewSQL).
					WithArgs("restricted", "v1").
					WillReturnError(errors.New("permission denied for relation v1"))
			},
			wantErr:     true,
			wantErrPart: "permission denied",
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()
			c.mockSetup(mock)

			ex := NewExtractor(db)
			result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
				SchemaName:      c.schema,
				DatabaseObjects: []*driverV2.DatabaseObject{c.object},
			})

			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", c.wantErrPart)
				}
				if !strings.Contains(err.Error(), c.wantErrPart) {
					t.Errorf("error %q does not contain %q", err.Error(), c.wantErrPart)
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %+v", result)
				}
				if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
					t.Errorf("unmet sqlmock expectations: %v", mockErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.SchemaName != c.schema {
				t.Errorf("SchemaName = %q, want %q", result.SchemaName, c.schema)
			}
			if len(result.DatabaseObjectDDLs) != 1 {
				t.Fatalf("DatabaseObjectDDLs len = %d, want 1", len(result.DatabaseObjectDDLs))
			}
			got := result.DatabaseObjectDDLs[0]
			if got.DatabaseObject == nil {
				t.Fatal("DatabaseObject metadata missing")
			}
			if got.DatabaseObject.ObjectName != c.wantObjName {
				t.Errorf("ObjectName = %q, want %q", got.DatabaseObject.ObjectName, c.wantObjName)
			}
			if got.DatabaseObject.ObjectType != c.wantObjType {
				t.Errorf("ObjectType = %q, want %q", got.DatabaseObject.ObjectType, c.wantObjType)
			}
			if c.wantDDLEmpty {
				if got.ObjectDDL != "" {
					t.Errorf("expected empty DDL on not-found, got %q", got.ObjectDDL)
				}
			} else if got.ObjectDDL != c.wantDDL {
				t.Errorf("ObjectDDL mismatch:\n  got  %q\n  want %q", got.ObjectDDL, c.wantDDL)
			}
			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Errorf("unmet sqlmock expectations: %v", mockErr)
			}
		})
	}
}

// TestExtractor_Extract_UnsupportedObjectType 覆盖 default 分支：本期不支持的
// ObjectType 一律返回 design §6.1 line 399 字面错误。
//
// FUNCTION / PROCEDURE 当前同样落入 default 分支（Task-Dev-006 会原地补充
// case 并修正本测试期望）。
func TestExtractor_Extract_UnsupportedObjectType(t *testing.T) {
	cases := map[string]struct {
		objectType string
	}{
		"INDEX":                              {objectType: "INDEX"},
		"TRIGGER":                            {objectType: driverV2.ObjectType_TRIGGER},
		"EVENT":                              {objectType: driverV2.ObjectType_EVENT},
		"SEQUENCE":                           {objectType: "SEQUENCE"},
		"TYPE":                               {objectType: "TYPE"},
		"PACKAGE":                            {objectType: "PACKAGE"},
		"FUNCTION_passthrough_to_Task006":    {objectType: driverV2.ObjectType_FUNCTION},
		"PROCEDURE_passthrough_to_Task006":   {objectType: driverV2.ObjectType_PROCEDURE},
		"empty_object_type_falls_to_default": {objectType: ""},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()
			// 不设任何 ExpectQuery：default 分支应在 dispatch 阶段直接返回错误，
			// 不会触达 DB。ExpectationsWereMet 兜底确保没有意外查询发生。

			ex := NewExtractor(db)
			result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
				SchemaName: "public",
				DatabaseObjects: []*driverV2.DatabaseObject{
					{ObjectName: "obj", ObjectType: c.objectType},
				},
			})

			if err == nil {
				t.Fatalf("expected not-supported error, got nil; result=%+v", result)
			}
			wantSubstr := "is not supported in this release"
			if !strings.Contains(err.Error(), wantSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), wantSubstr)
			}
			// design §6.1 line 399 要求把 ObjectType 字面值嵌入错误信息，便于运维定位。
			if !strings.Contains(err.Error(), c.objectType) {
				t.Errorf("error %q does not embed ObjectType literal %q", err.Error(), c.objectType)
			}
			if result != nil {
				t.Errorf("expected nil result, got %+v", result)
			}
			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Errorf("unmet sqlmock expectations: %v", mockErr)
			}
		})
	}
}

// TestExtractor_QuoteIdent_TableInjectionDefense 验证 schema / table 名含
// 特殊字符（空格 / 大小写 / 中文 / SQL 注入串 / 单双引号）时，Extractor
// 不在 Go 侧做任何 escape，原值通过 $1 / $2 透传到 sqlmock 参数断言。
//
// 这是 compat-RISK-4（SQL 注入）的兜底测试：转义责任在 PostgreSQL 服务端 +
// libpq 驱动；Go 侧任何对 schema / object 名做手工 escape 的代码都会让本组
// 用例的 WithArgs 断言失败。
func TestExtractor_QuoteIdent_TableInjectionDefense(t *testing.T) {
	cases := map[string]struct {
		schema string
		table  string
	}{
		"space_in_schema":          {schema: "My Schema", table: "t1"},
		"uppercase_schema":         {schema: "MySchema", table: "t1"},
		"chinese_schema":           {schema: "中文模式", table: "t1"},
		"sql_injection_in_object":  {schema: "public", table: "; DROP DATABASE postgres; --"},
		"single_quote_in_object":   {schema: "public", table: "it's"},
		"double_quote_in_object":   {schema: "public", table: `"quoted"`},
		"chinese_in_object_name":   {schema: "public", table: "表名"},
		"mixed_special_in_schema":  {schema: `s"a'b c中`, table: "t1"},
	}

	const ddlEcho = "<echo>"

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()

			// WithArgs(c.schema, c.table) 强约束：sqlmock 会对每个 driver.Value
			// 做 reflect.DeepEqual 比较，若 Extractor 在 Go 侧做任何 escape，
			// 实参就会与 c.schema / c.table 原值不等，匹配失败导致测试 fail。
			mock.ExpectQuery(extractTableSQL).
				WithArgs(c.schema, c.table).
				WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow(ddlEcho))

			ex := NewExtractor(db)
			result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
				SchemaName: c.schema,
				DatabaseObjects: []*driverV2.DatabaseObject{
					{ObjectName: c.table, ObjectType: driverV2.ObjectType_TABLE},
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil || len(result.DatabaseObjectDDLs) != 1 {
				t.Fatalf("unexpected result shape: %+v", result)
			}
			got := result.DatabaseObjectDDLs[0]
			if got.DatabaseObject.ObjectName != c.table {
				t.Errorf("ObjectName not preserved: got %q want %q", got.DatabaseObject.ObjectName, c.table)
			}
			if got.ObjectDDL != ddlEcho {
				t.Errorf("ObjectDDL = %q, want %q", got.ObjectDDL, ddlEcho)
			}
			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Errorf("unmet sqlmock expectations: %v", mockErr)
			}
		})
	}
}

// TestExtractor_QuoteIdent_ViewInjectionDefense 与 TestExtractor_QuoteIdent_
// TableInjectionDefense 同结构，但走 VIEW 抽取路径（view.go 的参数化绑定
// 同样需要防注入；design §17.1.2 要求 TABLE / VIEW 双路径都覆盖）。
func TestExtractor_QuoteIdent_ViewInjectionDefense(t *testing.T) {
	cases := map[string]struct {
		schema string
		view   string
	}{
		"space_in_schema":         {schema: "My Schema", view: "v1"},
		"chinese_schema":          {schema: "中文模式", view: "v1"},
		"sql_injection_in_view":   {schema: "public", view: "v1'; DROP VIEW x; --"},
		"single_quote_in_view":    {schema: "public", view: "it's_view"},
		"double_quote_in_view":    {schema: "public", view: `"quoted_view"`},
		"chinese_in_view_name":    {schema: "public", view: "视图名"},
	}

	const ddlEcho = "<echo-view>"

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()

			mock.ExpectQuery(extractViewSQL).
				WithArgs(c.schema, c.view).
				WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow(ddlEcho))

			ex := NewExtractor(db)
			result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
				SchemaName: c.schema,
				DatabaseObjects: []*driverV2.DatabaseObject{
					{ObjectName: c.view, ObjectType: driverV2.ObjectType_VIEW},
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil || len(result.DatabaseObjectDDLs) != 1 {
				t.Fatalf("unexpected result shape: %+v", result)
			}
			got := result.DatabaseObjectDDLs[0]
			if got.DatabaseObject.ObjectName != c.view {
				t.Errorf("ObjectName not preserved: got %q want %q", got.DatabaseObject.ObjectName, c.view)
			}
			if got.DatabaseObject.ObjectType != driverV2.ObjectType_VIEW {
				t.Errorf("ObjectType = %q, want %q", got.DatabaseObject.ObjectType, driverV2.ObjectType_VIEW)
			}
			if got.ObjectDDL != ddlEcho {
				t.Errorf("ObjectDDL = %q, want %q", got.ObjectDDL, ddlEcho)
			}
			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Errorf("unmet sqlmock expectations: %v", mockErr)
			}
		})
	}
}

// TestExtractor_Extract_NilInfoAndMultipleObjects 锁定 Extract 入口的两个
// 边界行为：
//
//   - info == nil 直接报错（不走任何 DB 调用）；
//   - 多对象 info 按顺序聚合到同一个 *DatabaseSchemaObjectResult；nil 对象
//     被跳过（不产生空 DDL 噪音）。
//
// 这两条不在 design §17.1.2 显式列出，但属于"入口契约"的健壮性兜底，避免
// Task-Dev-007 接入 driver.GetDatabaseObjectDDL 时撞到 nil-deref。
func TestExtractor_Extract_NilInfoAndMultipleObjects(t *testing.T) {
	t.Run("nil_info_returns_error", func(t *testing.T) {
		db, _, cleanup := newMock(t)
		defer cleanup()
		ex := NewExtractor(db)
		_, err := ex.Extract(context.Background(), nil)
		if err == nil {
			t.Fatal("expected error on nil info, got nil")
		}
		if !strings.Contains(err.Error(), "nil") {
			t.Errorf("error %q does not mention nil", err.Error())
		}
	})

	t.Run("multiple_objects_aggregate_in_order", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()

		mock.ExpectQuery(extractTableSQL).
			WithArgs("public", "t1").
			WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow("CREATE TABLE t1"))
		mock.ExpectQuery(extractViewSQL).
			WithArgs("public", "v1").
			WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow("CREATE OR REPLACE VIEW v1"))
		mock.ExpectQuery(extractTableSQL).
			WithArgs("public", "t2").
			WillReturnError(sql.ErrNoRows)

		ex := NewExtractor(db)
		result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
			SchemaName: "public",
			DatabaseObjects: []*driverV2.DatabaseObject{
				{ObjectName: "t1", ObjectType: driverV2.ObjectType_TABLE},
				nil, // 应被跳过
				{ObjectName: "v1", ObjectType: driverV2.ObjectType_VIEW},
				{ObjectName: "t2", ObjectType: driverV2.ObjectType_TABLE}, // not found → 空 DDL 占位
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.DatabaseObjectDDLs) != 3 {
			t.Fatalf("expected 3 DDL entries (nil skipped), got %d", len(result.DatabaseObjectDDLs))
		}
		wantOrder := []struct {
			name, typ, ddl string
		}{
			{"t1", driverV2.ObjectType_TABLE, "CREATE TABLE t1"},
			{"v1", driverV2.ObjectType_VIEW, "CREATE OR REPLACE VIEW v1"},
			{"t2", driverV2.ObjectType_TABLE, ""},
		}
		for i, want := range wantOrder {
			got := result.DatabaseObjectDDLs[i]
			if got.DatabaseObject.ObjectName != want.name {
				t.Errorf("[%d] ObjectName = %q, want %q", i, got.DatabaseObject.ObjectName, want.name)
			}
			if got.DatabaseObject.ObjectType != want.typ {
				t.Errorf("[%d] ObjectType = %q, want %q", i, got.DatabaseObject.ObjectType, want.typ)
			}
			if got.ObjectDDL != want.ddl {
				t.Errorf("[%d] ObjectDDL = %q, want %q", i, got.ObjectDDL, want.ddl)
			}
		}
		if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
			t.Errorf("unmet sqlmock expectations: %v", mockErr)
		}
	})
}
