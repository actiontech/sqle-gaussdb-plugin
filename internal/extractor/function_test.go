// function_test.go covers design §17.1.3 — FUNCTION / PROCEDURE 重载单测矩阵：
//
//   - FUNCTION / PROCEDURE 正常分发（Extract 入口 → extractFunction /
//     extractProcedure → 单参 / 多参 / 无参 / 对象不存在 / 权限错误透传）
//   - FUNCTION 重载（无参 / 单参 / 同名两参 / 同名三重载）：ObjectName 携带
//     `funcname(arg_type_list)`，map key 天然唯一（design §6.4 + R-Q4-2）
//   - PROCEDURE 重载（同上结构，prokind = 'p'）
//   - 注入防御（FUNCTION + PROCEDURE 各 6 类输入串，与 table/view 注入测试对称）
//   - Task-Test-Fix-001 P1.1：双列 SELECT 已拆成两次单列查询；单测同步
//     更新为 mock 两条独立 query（extract*DefSQL → defs 列、extract*ArgsSQL →
//     arguments 列），断言两次结果按 ORDER BY oid 顺序对齐。
//
// 所有 sqlmock 用 QueryMatcherOption(QueryMatcherEqual) 模式锁定 SQL 文本，
// 避免正则模糊匹配掩盖 design 模板偏离；WithArgs 用原值锁定 Go 侧未 escape。
//
// 共用 helper newMock 来自 extractor_test.go（同包）。

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

// expectFuncDefsAndArgs 是 FUNCTION 路径双查询的 mock helper，rows 形如
// [{def0, args0}, {def1, args1}, ...]；defs 列 + args 列分别由两次 query 返回。
func expectFuncDefsAndArgs(mock sqlmock.Sqlmock, schema, name string, rows [][2]string) {
	defRows := sqlmock.NewRows([]string{"functiondef"})
	argRows := sqlmock.NewRows([]string{"arguments"})
	for _, r := range rows {
		defRows.AddRow(r[0])
		argRows.AddRow(r[1])
	}
	mock.ExpectQuery(extractFunctionDefSQL).
		WithArgs(schema, name).
		WillReturnRows(defRows)
	mock.ExpectQuery(extractFunctionArgsSQL).
		WithArgs(schema, name).
		WillReturnRows(argRows)
}

// expectProcDefsAndArgs 是 PROCEDURE 路径双查询的 mock helper（结构同上）。
func expectProcDefsAndArgs(mock sqlmock.Sqlmock, schema, name string, rows [][2]string) {
	defRows := sqlmock.NewRows([]string{"procdef"})
	argRows := sqlmock.NewRows([]string{"arguments"})
	for _, r := range rows {
		defRows.AddRow(r[0])
		argRows.AddRow(r[1])
	}
	mock.ExpectQuery(extractProcedureDefSQL).
		WithArgs(schema, name).
		WillReturnRows(defRows)
	mock.ExpectQuery(extractProcedureArgsSQL).
		WithArgs(schema, name).
		WillReturnRows(argRows)
}

// TestExtractor_Extract_FunctionAndProcedure 覆盖 Extract 入口的 FUNCTION /
// PROCEDURE 分发 + 对象不存在占位 + 权限错误透传（design §6.2.3 / §6.2.4 /
// §6.5）。
func TestExtractor_Extract_FunctionAndProcedure(t *testing.T) {
	const (
		funcDDLSingleArg = "CREATE OR REPLACE FUNCTION public.fn(p1 integer) RETURNS integer LANGUAGE plpgsql AS $function$ BEGIN RETURN p1; END $function$"
		procDDLMultiArgs = "CREATE OR REPLACE PROCEDURE public.proc(p1 integer, p2 text) LANGUAGE plpgsql AS $procedure$ BEGIN NULL; END $procedure$"
	)

	cases := map[string]struct {
		schema       string
		object       *driverV2.DatabaseObject
		mockSetup    func(mock sqlmock.Sqlmock)
		wantErr      bool
		wantErrPart  string
		wantLen      int
		wantObjName  string
		wantObjType  string
		wantDDL      string
		wantDDLEmpty bool
	}{
		"function_normal_single_arg": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "fn", ObjectType: driverV2.ObjectType_FUNCTION},
			mockSetup: func(mock sqlmock.Sqlmock) {
				expectFuncDefsAndArgs(mock, "public", "fn", [][2]string{
					{funcDDLSingleArg, "integer"},
				})
			},
			wantLen:     1,
			wantObjName: "fn(integer)",
			wantObjType: driverV2.ObjectType_FUNCTION,
			wantDDL:     funcDDLSingleArg,
		},
		"procedure_normal_multi_args": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "proc", ObjectType: driverV2.ObjectType_PROCEDURE},
			mockSetup: func(mock sqlmock.Sqlmock) {
				expectProcDefsAndArgs(mock, "public", "proc", [][2]string{
					{procDDLMultiArgs, "integer, text"},
				})
			},
			wantLen:     1,
			wantObjName: "proc(integer, text)",
			wantObjType: driverV2.ObjectType_PROCEDURE,
			wantDDL:     procDDLMultiArgs,
		},
		"function_not_found_returns_empty_placeholder": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "missing_fn", ObjectType: driverV2.ObjectType_FUNCTION},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// 两次 query 都返回空行集 → 触发占位兜底。
				mock.ExpectQuery(extractFunctionDefSQL).
					WithArgs("public", "missing_fn").
					WillReturnRows(sqlmock.NewRows([]string{"functiondef"}))
				mock.ExpectQuery(extractFunctionArgsSQL).
					WithArgs("public", "missing_fn").
					WillReturnRows(sqlmock.NewRows([]string{"arguments"}))
			},
			wantLen:      1,
			wantObjName:  "missing_fn", // 不带参数签名，与"该侧不存在"语义一致
			wantObjType:  driverV2.ObjectType_FUNCTION,
			wantDDLEmpty: true,
		},
		"procedure_not_found_returns_empty_placeholder": {
			schema: "public",
			object: &driverV2.DatabaseObject{ObjectName: "missing_proc", ObjectType: driverV2.ObjectType_PROCEDURE},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractProcedureDefSQL).
					WithArgs("public", "missing_proc").
					WillReturnRows(sqlmock.NewRows([]string{"procdef"}))
				mock.ExpectQuery(extractProcedureArgsSQL).
					WithArgs("public", "missing_proc").
					WillReturnRows(sqlmock.NewRows([]string{"arguments"}))
			},
			wantLen:      1,
			wantObjName:  "missing_proc",
			wantObjType:  driverV2.ObjectType_PROCEDURE,
			wantDDLEmpty: true,
		},
		"function_permission_denied_propagates": {
			schema: "restricted",
			object: &driverV2.DatabaseObject{ObjectName: "fn", ObjectType: driverV2.ObjectType_FUNCTION},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// 第一条 defs query 失败即返回，不会再发起 args query。
				mock.ExpectQuery(extractFunctionDefSQL).
					WithArgs("restricted", "fn").
					WillReturnError(errors.New("permission denied for schema pg_catalog"))
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
			if len(result.DatabaseObjectDDLs) != c.wantLen {
				t.Fatalf("DatabaseObjectDDLs len = %d, want %d", len(result.DatabaseObjectDDLs), c.wantLen)
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

// TestExtractor_FunctionOverload 覆盖 design §17.1.3 + §6.4 重载识别核心矩阵：
// 无参 / 单参 / 同名两重载 / 同名三重载 四类签名分支。
//
// 每条 case 直接调用 extractFunction（绕过 Extract 入口）以便精确断言 slice
// 长度与 ObjectName 集合；这与 TestExtractor_Extract_FunctionAndProcedure 形成
// 互补——后者覆盖入口分发，前者覆盖 helper 自身契约。
func TestExtractor_FunctionOverload(t *testing.T) {
	cases := map[string]struct {
		schema    string
		funcName  string
		mockRows  [][2]string // [functiondef, arguments] 序列；按 oid 顺序对齐
		wantNames []string    // 期望的 ObjectName 集合（顺序无关）
		wantDDLs  []string    // 期望的 DDL 集合（顺序无关）
	}{
		"function_no_arg": {
			schema:   "public",
			funcName: "fn",
			mockRows: [][2]string{
				{"CREATE OR REPLACE FUNCTION public.fn() RETURNS void AS $$ BEGIN NULL; END $$ LANGUAGE plpgsql", ""},
			},
			wantNames: []string{"fn()"},
			wantDDLs:  []string{"CREATE OR REPLACE FUNCTION public.fn() RETURNS void AS $$ BEGIN NULL; END $$ LANGUAGE plpgsql"},
		},
		"function_single_arg": {
			schema:   "public",
			funcName: "fn",
			mockRows: [][2]string{
				{"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql", "integer"},
			},
			wantNames: []string{"fn(integer)"},
			wantDDLs:  []string{"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql"},
		},
		"function_two_overloads": {
			schema:   "public",
			funcName: "fn",
			mockRows: [][2]string{
				{"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql", "integer"},
				{"CREATE FUNCTION public.fn(p1 text) RETURNS text AS $$ ... $$ LANGUAGE plpgsql", "text"},
			},
			wantNames: []string{"fn(integer)", "fn(text)"},
			wantDDLs: []string{
				"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql",
				"CREATE FUNCTION public.fn(p1 text) RETURNS text AS $$ ... $$ LANGUAGE plpgsql",
			},
		},
		"function_three_overloads": {
			schema:   "public",
			funcName: "fn",
			mockRows: [][2]string{
				{"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql", "integer"},
				{"CREATE FUNCTION public.fn(p1 text) RETURNS text AS $$ ... $$ LANGUAGE plpgsql", "text"},
				{"CREATE FUNCTION public.fn(p1 integer, p2 text) RETURNS text AS $$ ... $$ LANGUAGE plpgsql", "integer, text"},
			},
			wantNames: []string{"fn(integer)", "fn(text)", "fn(integer, text)"},
			wantDDLs: []string{
				"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql",
				"CREATE FUNCTION public.fn(p1 text) RETURNS text AS $$ ... $$ LANGUAGE plpgsql",
				"CREATE FUNCTION public.fn(p1 integer, p2 text) RETURNS text AS $$ ... $$ LANGUAGE plpgsql",
			},
		},
		// 「无参 + 单参」双重载场景：补充 design §6.4 中 "fn()" 与 "fn(integer)"
		// 同时存在时上层 map key 不冲突的回归验证（无参函数 ObjectName 与
		// 任意带参签名永远不同，但单测必须显式断言 key 集合唯一性）。
		"function_no_arg_and_single_arg_overload": {
			schema:   "public",
			funcName: "fn",
			mockRows: [][2]string{
				{"CREATE FUNCTION public.fn() RETURNS void AS $$ BEGIN NULL; END $$ LANGUAGE plpgsql", ""},
				{"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql", "integer"},
			},
			wantNames: []string{"fn()", "fn(integer)"},
			wantDDLs: []string{
				"CREATE FUNCTION public.fn() RETURNS void AS $$ BEGIN NULL; END $$ LANGUAGE plpgsql",
				"CREATE FUNCTION public.fn(p1 integer) RETURNS integer AS $$ ... $$ LANGUAGE plpgsql",
			},
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()

			expectFuncDefsAndArgs(mock, c.schema, c.funcName, c.mockRows)

			ex := NewExtractor(db)
			results, err := ex.extractFunction(context.Background(), c.schema, c.funcName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != len(c.wantNames) {
				t.Fatalf("results len = %d, want %d", len(results), len(c.wantNames))
			}

			gotNames := make([]string, 0, len(results))
			gotDDLs := make([]string, 0, len(results))
			gotTypes := make(map[string]bool)
			for _, r := range results {
				gotNames = append(gotNames, r.DatabaseObject.ObjectName)
				gotDDLs = append(gotDDLs, r.ObjectDDL)
				gotTypes[r.DatabaseObject.ObjectType] = true
			}
			sort.Strings(gotNames)
			sort.Strings(gotDDLs)
			wantNamesSorted := append([]string(nil), c.wantNames...)
			wantDDLsSorted := append([]string(nil), c.wantDDLs...)
			sort.Strings(wantNamesSorted)
			sort.Strings(wantDDLsSorted)

			for i := range wantNamesSorted {
				if gotNames[i] != wantNamesSorted[i] {
					t.Errorf("ObjectName[%d] = %q, want %q", i, gotNames[i], wantNamesSorted[i])
				}
				if gotDDLs[i] != wantDDLsSorted[i] {
					t.Errorf("ObjectDDL[%d] = %q, want %q", i, gotDDLs[i], wantDDLsSorted[i])
				}
			}

			if len(gotTypes) != 1 || !gotTypes[driverV2.ObjectType_FUNCTION] {
				t.Errorf("ObjectType set = %v, want only {FUNCTION}", gotTypes)
			}

			uniq := make(map[string]bool)
			for _, n := range gotNames {
				if uniq[n] {
					t.Errorf("duplicate ObjectName %q in overload result; map key uniqueness broken", n)
				}
				uniq[n] = true
			}

			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Errorf("unmet sqlmock expectations: %v", mockErr)
			}
		})
	}
}

// TestExtractor_ProcedureOverload 与 TestExtractor_FunctionOverload 同结构，
// 仅走 extractProcedure 路径（prokind = 'p'）。
func TestExtractor_ProcedureOverload(t *testing.T) {
	cases := map[string]struct {
		schema    string
		procName  string
		mockRows  [][2]string
		wantNames []string
		wantDDLs  []string
	}{
		"procedure_no_arg": {
			schema:   "public",
			procName: "proc",
			mockRows: [][2]string{
				{"CREATE OR REPLACE PROCEDURE public.proc() LANGUAGE plpgsql AS $$ BEGIN NULL; END $$", ""},
			},
			wantNames: []string{"proc()"},
			wantDDLs:  []string{"CREATE OR REPLACE PROCEDURE public.proc() LANGUAGE plpgsql AS $$ BEGIN NULL; END $$"},
		},
		"procedure_single_arg": {
			schema:   "public",
			procName: "proc",
			mockRows: [][2]string{
				{"CREATE PROCEDURE public.proc(p1 integer) LANGUAGE plpgsql AS $$ ... $$", "integer"},
			},
			wantNames: []string{"proc(integer)"},
			wantDDLs:  []string{"CREATE PROCEDURE public.proc(p1 integer) LANGUAGE plpgsql AS $$ ... $$"},
		},
		"procedure_two_overloads": {
			schema:   "public",
			procName: "proc",
			mockRows: [][2]string{
				{"CREATE PROCEDURE public.proc(p1 integer) LANGUAGE plpgsql AS $$ ... $$", "integer"},
				{"CREATE PROCEDURE public.proc(p1 text) LANGUAGE plpgsql AS $$ ... $$", "text"},
			},
			wantNames: []string{"proc(integer)", "proc(text)"},
			wantDDLs: []string{
				"CREATE PROCEDURE public.proc(p1 integer) LANGUAGE plpgsql AS $$ ... $$",
				"CREATE PROCEDURE public.proc(p1 text) LANGUAGE plpgsql AS $$ ... $$",
			},
		},
		"procedure_three_overloads": {
			schema:   "public",
			procName: "proc",
			mockRows: [][2]string{
				{"CREATE PROCEDURE public.proc(p1 integer) LANGUAGE plpgsql AS $$ ... $$", "integer"},
				{"CREATE PROCEDURE public.proc(p1 text) LANGUAGE plpgsql AS $$ ... $$", "text"},
				{"CREATE PROCEDURE public.proc(p1 integer, p2 text) LANGUAGE plpgsql AS $$ ... $$", "integer, text"},
			},
			wantNames: []string{"proc(integer)", "proc(text)", "proc(integer, text)"},
			wantDDLs: []string{
				"CREATE PROCEDURE public.proc(p1 integer) LANGUAGE plpgsql AS $$ ... $$",
				"CREATE PROCEDURE public.proc(p1 text) LANGUAGE plpgsql AS $$ ... $$",
				"CREATE PROCEDURE public.proc(p1 integer, p2 text) LANGUAGE plpgsql AS $$ ... $$",
			},
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()

			expectProcDefsAndArgs(mock, c.schema, c.procName, c.mockRows)

			ex := NewExtractor(db)
			results, err := ex.extractProcedure(context.Background(), c.schema, c.procName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != len(c.wantNames) {
				t.Fatalf("results len = %d, want %d", len(results), len(c.wantNames))
			}

			gotNames := make([]string, 0, len(results))
			gotDDLs := make([]string, 0, len(results))
			gotTypes := make(map[string]bool)
			for _, r := range results {
				gotNames = append(gotNames, r.DatabaseObject.ObjectName)
				gotDDLs = append(gotDDLs, r.ObjectDDL)
				gotTypes[r.DatabaseObject.ObjectType] = true
			}
			sort.Strings(gotNames)
			sort.Strings(gotDDLs)
			wantNamesSorted := append([]string(nil), c.wantNames...)
			wantDDLsSorted := append([]string(nil), c.wantDDLs...)
			sort.Strings(wantNamesSorted)
			sort.Strings(wantDDLsSorted)

			for i := range wantNamesSorted {
				if gotNames[i] != wantNamesSorted[i] {
					t.Errorf("ObjectName[%d] = %q, want %q", i, gotNames[i], wantNamesSorted[i])
				}
				if gotDDLs[i] != wantDDLsSorted[i] {
					t.Errorf("ObjectDDL[%d] = %q, want %q", i, gotDDLs[i], wantDDLsSorted[i])
				}
			}

			if len(gotTypes) != 1 || !gotTypes[driverV2.ObjectType_PROCEDURE] {
				t.Errorf("ObjectType set = %v, want only {PROCEDURE}", gotTypes)
			}

			uniq := make(map[string]bool)
			for _, n := range gotNames {
				if uniq[n] {
					t.Errorf("duplicate ObjectName %q in overload result; map key uniqueness broken", n)
				}
				uniq[n] = true
			}

			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Errorf("unmet sqlmock expectations: %v", mockErr)
			}
		})
	}
}

// TestExtractor_QuoteIdent_FunctionInjectionDefense 验证 FUNCTION 路径的注入
// 防御：schema / function 名含特殊字符（空格 / 大小写 / 中文 / SQL 注入串 /
// 单双引号）时，Extractor 不在 Go 侧做任何 escape，原值通过 $1 / $2 透传到
// sqlmock 参数断言。
//
// 与 TABLE / VIEW 注入测试形成全路径覆盖（compat-RISK-4 SQL 注入兜底）。
func TestExtractor_QuoteIdent_FunctionInjectionDefense(t *testing.T) {
	cases := map[string]struct {
		schema   string
		funcName string
	}{
		"space_in_schema":            {schema: "My Schema", funcName: "fn"},
		"uppercase_schema":           {schema: "MySchema", funcName: "fn"},
		"chinese_schema":             {schema: "中文模式", funcName: "fn"},
		"sql_injection_in_func_name": {schema: "public", funcName: "fn; DROP FUNCTION x; --"},
		"single_quote_in_func_name":  {schema: "public", funcName: "it's_fn"},
		"double_quote_in_func_name":  {schema: "public", funcName: `"quoted_fn"`},
	}

	const ddlEcho = "<echo-function>"

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()

			// WithArgs(c.schema, c.funcName) 强约束：sqlmock 对每个 driver.Value
			// 做 reflect.DeepEqual 比较；若 Extractor 在 Go 侧做任何 escape，
			// 实参就会与原值不等，匹配失败导致测试 fail。
			expectFuncDefsAndArgs(mock, c.schema, c.funcName, [][2]string{
				{ddlEcho, "integer"},
			})

			ex := NewExtractor(db)
			result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
				SchemaName: c.schema,
				DatabaseObjects: []*driverV2.DatabaseObject{
					{ObjectName: c.funcName, ObjectType: driverV2.ObjectType_FUNCTION},
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil || len(result.DatabaseObjectDDLs) != 1 {
				t.Fatalf("unexpected result shape: %+v", result)
			}
			got := result.DatabaseObjectDDLs[0]
			// ObjectName 在重载场景拼接 "(arguments)"；funcName 原值必须为前缀，
			// 表明用户输入未被 Go 侧 escape。
			wantObjName := c.funcName + "(integer)"
			if got.DatabaseObject.ObjectName != wantObjName {
				t.Errorf("ObjectName = %q, want %q", got.DatabaseObject.ObjectName, wantObjName)
			}
			if got.DatabaseObject.ObjectType != driverV2.ObjectType_FUNCTION {
				t.Errorf("ObjectType = %q, want %q", got.DatabaseObject.ObjectType, driverV2.ObjectType_FUNCTION)
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

// TestExtractor_QuoteIdent_ProcedureInjectionDefense 与上一组同结构，但走
// PROCEDURE 抽取路径，确保 procedure.go 的参数化绑定同样防注入。
func TestExtractor_QuoteIdent_ProcedureInjectionDefense(t *testing.T) {
	cases := map[string]struct {
		schema   string
		procName string
	}{
		"space_in_schema":            {schema: "My Schema", procName: "proc"},
		"uppercase_schema":           {schema: "MySchema", procName: "proc"},
		"chinese_schema":             {schema: "中文模式", procName: "proc"},
		"sql_injection_in_proc_name": {schema: "public", procName: "proc; DROP PROCEDURE x; --"},
		"single_quote_in_proc_name":  {schema: "public", procName: "it's_proc"},
		"double_quote_in_proc_name":  {schema: "public", procName: `"quoted_proc"`},
	}

	const ddlEcho = "<echo-procedure>"

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()

			expectProcDefsAndArgs(mock, c.schema, c.procName, [][2]string{
				{ddlEcho, "integer"},
			})

			ex := NewExtractor(db)
			result, err := ex.Extract(context.Background(), &driverV2.DatabaseSchemaInfo{
				SchemaName: c.schema,
				DatabaseObjects: []*driverV2.DatabaseObject{
					{ObjectName: c.procName, ObjectType: driverV2.ObjectType_PROCEDURE},
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil || len(result.DatabaseObjectDDLs) != 1 {
				t.Fatalf("unexpected result shape: %+v", result)
			}
			got := result.DatabaseObjectDDLs[0]
			wantObjName := c.procName + "(integer)"
			if got.DatabaseObject.ObjectName != wantObjName {
				t.Errorf("ObjectName = %q, want %q", got.DatabaseObject.ObjectName, wantObjName)
			}
			if got.DatabaseObject.ObjectType != driverV2.ObjectType_PROCEDURE {
				t.Errorf("ObjectType = %q, want %q", got.DatabaseObject.ObjectType, driverV2.ObjectType_PROCEDURE)
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

// TestExtractFunction_NoTupleWrapping 是 Task-Test-Fix-001 P1.1 修复的回归
// 验证：双列 SELECT 拆成两次单列查询后，extractor 不再在 ObjectDDL 中输出
// `(N,"...")` 这种 Postgres tuple/record 字面值。
//
// 验证策略：mock 单列 SQL 返回纯 DDL 文本 → 断言 ObjectDDL 等于该原值（无
// 前导 `(N,"` 与尾部 `")"`）。同时验证 ObjectName 拼接结果不含 tuple 字面值。
func TestExtractFunction_NoTupleWrapping(t *testing.T) {
	const rawDDL = "CREATE OR REPLACE FUNCTION test_2905_base.fn(p1 integer) RETURNS integer LANGUAGE plpgsql AS $$ BEGIN RETURN p1; END $$"

	db, mock, cleanup := newMock(t)
	defer cleanup()

	expectFuncDefsAndArgs(mock, "test_2905_base", "fn", [][2]string{
		{rawDDL, "integer"},
	})

	ex := NewExtractor(db)
	results, err := ex.extractFunction(context.Background(), "test_2905_base", "fn")
	if err != nil {
		t.Fatalf("extractFunction: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	got := results[0]
	if got.ObjectDDL != rawDDL {
		t.Errorf("ObjectDDL = %q, want %q", got.ObjectDDL, rawDDL)
	}
	// 反向断言：tuple 字面值前缀 `(N,"` 不应出现。
	if strings.HasPrefix(got.ObjectDDL, "(") && strings.Contains(got.ObjectDDL, `,"`) {
		t.Errorf("ObjectDDL appears to be Postgres tuple literal: %q", got.ObjectDDL)
	}
	if got.DatabaseObject.ObjectName != "fn(integer)" {
		t.Errorf("ObjectName = %q, want %q", got.DatabaseObject.ObjectName, "fn(integer)")
	}
	if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
		t.Errorf("unmet sqlmock expectations: %v", mockErr)
	}
}

// TestExtractProcedure_NoTupleWrapping 同 TestExtractFunction_NoTupleWrapping，
// 仅走 PROCEDURE 路径（case-2-4 报告的 (1,"CREATE OR REPLACE PROCEDURE ...")
// 场景）。
func TestExtractProcedure_NoTupleWrapping(t *testing.T) {
	const rawDDL = "CREATE OR REPLACE PROCEDURE test_2905_compared.proc_log_action(p_action text) LANGUAGE plpgsql AS $$ BEGIN RAISE NOTICE 'action: %', p_action; END; $$"

	db, mock, cleanup := newMock(t)
	defer cleanup()

	expectProcDefsAndArgs(mock, "test_2905_compared", "proc_log_action", [][2]string{
		{rawDDL, "p_action text"},
	})

	ex := NewExtractor(db)
	results, err := ex.extractProcedure(context.Background(), "test_2905_compared", "proc_log_action")
	if err != nil {
		t.Fatalf("extractProcedure: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	got := results[0]
	if got.ObjectDDL != rawDDL {
		t.Errorf("ObjectDDL = %q, want %q", got.ObjectDDL, rawDDL)
	}
	if strings.HasPrefix(got.ObjectDDL, "(") && strings.Contains(got.ObjectDDL, `,"`) {
		t.Errorf("ObjectDDL appears to be Postgres tuple literal: %q", got.ObjectDDL)
	}
	if got.DatabaseObject.ObjectName != "proc_log_action(p_action text)" {
		t.Errorf("ObjectName = %q, want %q", got.DatabaseObject.ObjectName, "proc_log_action(p_action text)")
	}
	if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
		t.Errorf("unmet sqlmock expectations: %v", mockErr)
	}
}

// TestExtractProcedure_TrimTrailingSlash 覆盖 Task-Test-Fix-002 P3：
//
// openGauss pg_get_functiondef 对 PROCEDURE 返回值末尾会带一行独立的 `/`
// （PL/SQL 终止符），Go database/sql + openGauss-connector-go-pq 驱动不识别
// `/` 作为语句终止符，会撞 `syntax error at or near "/"`。extractor 层必须
// 剥除该末尾独立行 `/`（保留中间内嵌内容），让下游 modify_sql_statements 输出
// 能在 Go/JDBC 驱动里直接 EXEC_OK（case-3-4.md / semantic
// opengauss_procedure_slash_terminator_driver_incompat_20260521.md 缺陷溯源）。
func TestExtractProcedure_TrimTrailingSlash(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want string
	}{
		"slash_with_trailing_newline": {
			raw: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;\n" +
				"/\n",
			want: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;",
		},
		"slash_no_trailing_newline": {
			raw: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;\n" +
				"/",
			want: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;",
		},
		"slash_with_extra_blank_line_before": {
			raw: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;\n" +
				"\n/\n",
			want: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;",
		},
		"no_slash_unchanged": {
			raw: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;",
			want: "CREATE OR REPLACE PROCEDURE s.p()\n" +
				"AS DECLARE BEGIN NULL; END;",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := newMock(t)
			defer cleanup()

			expectProcDefsAndArgs(mock, "s", "p", [][2]string{
				{c.raw, ""},
			})

			ex := NewExtractor(db)
			results, err := ex.extractProcedure(context.Background(), "s", "p")
			if err != nil {
				t.Fatalf("extractProcedure: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("results len = %d, want 1", len(results))
			}
			if results[0].ObjectDDL != c.want {
				t.Errorf("ObjectDDL mismatch:\n got: %q\nwant: %q", results[0].ObjectDDL, c.want)
			}
			if strings.HasSuffix(results[0].ObjectDDL, "/") {
				t.Errorf("ObjectDDL still ends with `/`: %q", results[0].ObjectDDL)
			}
			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Errorf("unmet sqlmock expectations: %v", mockErr)
			}
		})
	}
}
