// differ_test.go covers design §17.1.4 — Differ 单测矩阵：
//
//   - 4 类对象（TABLE / VIEW / FUNCTION / PROCEDURE）× 3 种差异分类
//     （仅 base / 仅 compared / 两侧 ObjectDDL 不同）= 12 个分支
//   - DROP TABLE WARNING 注释字面值（compat-RISK-5 / impact_analysis.yml R-Q4-1）
//     byte-for-byte 锁定
//   - DROP FUNCTION/PROCEDURE 带参数签名（R-Q4-2）+ 重载下两条独立 DROP
//   - 两侧 ObjectDDL 完全相同 → ModifySQLs 长度 0（无变更跳过）
//   - 不支持 ObjectType（INDEX / TRIGGER 等）→ 静默跳过，不 panic
//   - 混合 4 类对象的端到端集合断言（顺序无关，sort.Strings）
//
// 测试纯字符串断言：无 sqlmock / 无 sql.Open / 无网络 / 无文件 IO（differ 是纯
// 函数，符合 plan §6.2 / todo Task-Dev-007 子任务 D 的"纯字符串拼接断言"约束）。
//
// 顺序无关断言：参考 expertise_docs/semantic/sqle_plugin_overload_objectname_signature_20260521.md
// 与 sqlmock_rows_iteration_empty_and_order_20260521.md —— map 迭代顺序 Go runtime
// 不保证，所有多元素断言用 sort.Strings(got) == sort.Strings(want) 模式。

package differ

import (
	"sort"
	"strings"
	"testing"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// expectedWarning 是 DROP TABLE 前置注释字面值的测试副本；与 differ.go 包级
// 常量 dropTableWarningComment 字面值一致。
//
// 故意维护独立副本（而非 import 包级常量）—— 当 dropTableWarningComment 被
// 无意改写时，测试同步失效，让 code_review 能一眼看到字面值偏离
// （compat-RISK-5 兜底）。
const expectedWarning = "-- WARNING: DROP TABLE will drop data. Review before executing."

// ---- 工具构造器 ----

// makeDDL 构造单个 *driverV2.DatabaseObjectDDL 测试 fixture。
func makeDDL(name, objType, ddl string) *driverV2.DatabaseObjectDDL {
	return &driverV2.DatabaseObjectDDL{
		DatabaseObject: &driverV2.DatabaseObject{
			ObjectName: name,
			ObjectType: objType,
		},
		ObjectDDL: ddl,
	}
}

// ---- a) TestDiffer_TableOnlyBase ----

// TestDiffer_TableOnlyBase ：base 有 + compared 无 → 直接输出 base 侧 CREATE TABLE
// 原文（不含 WARNING / 不含 DROP）。
// design §7.2 表第 1 行（TABLE 列）。
func TestDiffer_TableOnlyBase(t *testing.T) {
	const tableDDL = "CREATE TABLE public.t1 (id int) WITH (orientation=row);"

	cases := map[string]struct {
		base []*driverV2.DatabaseObjectDDL
		want string
	}{
		"base_only_table": {
			base: []*driverV2.DatabaseObjectDDL{makeDDL("t1", driverV2.ObjectType_TABLE, tableDDL)},
			want: tableDDL,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("public", c.base, "public", nil)
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
			}
			if got[0] != c.want {
				t.Errorf("got[0] = %q, want %q", got[0], c.want)
			}
			if strings.Contains(got[0], expectedWarning) {
				t.Errorf("only-base TABLE must not contain WARNING comment; got=%q", got[0])
			}
			if strings.Contains(got[0], "DROP TABLE") {
				t.Errorf("only-base TABLE must not contain DROP TABLE; got=%q", got[0])
			}
		})
	}
}

// ---- b) TestDiffer_TableOnlyCompared ----

// TestDiffer_TableOnlyCompared ：仅 compared 有 → WARNING 前置 + DROP TABLE IF
// EXISTS schema.name（无引号 / 无分号）。
// design §7.2 表第 2 行（TABLE 列）+ §7.3 WARNING 注释格式。
func TestDiffer_TableOnlyCompared(t *testing.T) {
	cases := map[string]struct {
		comparedSchema string
		compared       []*driverV2.DatabaseObjectDDL
		wantSuffix     string // WARNING + "\n" 之后的 DROP 行字面值
	}{
		"compared_only_table": {
			comparedSchema: "public",
			compared: []*driverV2.DatabaseObjectDDL{
				makeDDL("t_dropme", driverV2.ObjectType_TABLE, "CREATE TABLE public.t_dropme (id int);"),
			},
			wantSuffix: "DROP TABLE IF EXISTS public.t_dropme",
		},
		"compared_only_table_chinese_schema": {
			comparedSchema: "中文模式",
			compared: []*driverV2.DatabaseObjectDDL{
				makeDDL("t1", driverV2.ObjectType_TABLE, "CREATE TABLE 中文模式.t1 (id int);"),
			},
			wantSuffix: "DROP TABLE IF EXISTS 中文模式.t1",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("base", nil, c.comparedSchema, c.compared)
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
			}
			want := expectedWarning + "\n" + c.wantSuffix
			if got[0] != want {
				t.Errorf("got = %q\nwant = %q", got[0], want)
			}
			// 二次校验首行严格等于 WARNING（compat-RISK-5 兜底）
			firstLine := strings.SplitN(got[0], "\n", 2)[0]
			if firstLine != expectedWarning {
				t.Errorf("first line = %q, want %q", firstLine, expectedWarning)
			}
		})
	}
}

// ---- c) TestDiffer_TableBothDiff ----

// TestDiffer_TableBothDiff ：两侧都有但 ObjectDDL 不同 → WARNING + DROP（带分号
// + 换行）+ base 侧 CREATE TABLE 原文。
// design §7.2 表第 3 行（TABLE 列）。
func TestDiffer_TableBothDiff(t *testing.T) {
	const (
		baseDDL     = "CREATE TABLE public.t1 (id int, name text);"
		comparedDDL = "CREATE TABLE public.t1 (id int);"
	)

	cases := map[string]struct {
		base, compared []*driverV2.DatabaseObjectDDL
	}{
		"table_both_diff": {
			base:     []*driverV2.DatabaseObjectDDL{makeDDL("t1", driverV2.ObjectType_TABLE, baseDDL)},
			compared: []*driverV2.DatabaseObjectDDL{makeDDL("t1", driverV2.ObjectType_TABLE, comparedDDL)},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("base_schema", c.base, "compared_schema", c.compared)
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
			}
			text := got[0]
			// 同时含 WARNING + DROP（带分号）+ base CREATE 原文，且顺序 DROP 在前 / CREATE 在后
			if !strings.Contains(text, expectedWarning) {
				t.Errorf("text missing WARNING; got=%q", text)
			}
			dropLine := "DROP TABLE IF EXISTS compared_schema.t1;"
			if !strings.Contains(text, dropLine) {
				t.Errorf("text missing %q; got=%q", dropLine, text)
			}
			if !strings.Contains(text, baseDDL) {
				t.Errorf("text missing base DDL %q; got=%q", baseDDL, text)
			}
			dropIdx := strings.Index(text, "DROP TABLE")
			createIdx := strings.Index(text, "CREATE TABLE")
			if dropIdx < 0 || createIdx < 0 || dropIdx > createIdx {
				t.Errorf("DROP must precede CREATE in both-diff TABLE; dropIdx=%d createIdx=%d text=%q", dropIdx, createIdx, text)
			}
		})
	}
}

// ---- d) TestDiffer_ViewBothDiff（含 view 3 种差异） ----

// TestDiffer_ViewBothDiff 覆盖 VIEW 三种差异分类（design §7.2 表 VIEW 列）：
//   - view_only_base: base 侧 ObjectDDL 原文（CREATE OR REPLACE VIEW，extractor
//     上游保证）
//   - view_only_compared: DROP VIEW IF EXISTS schema.viewName（无 WARNING）
//   - view_both_diff: base 侧 ObjectDDL（PG 系幂等，无 DROP）
func TestDiffer_ViewBothDiff(t *testing.T) {
	const (
		viewBaseDDL     = "CREATE OR REPLACE VIEW public.v1 AS SELECT 1 AS id;"
		viewComparedDDL = "CREATE OR REPLACE VIEW public.v1 AS SELECT 2 AS id;"
	)

	cases := map[string]struct {
		base, compared []*driverV2.DatabaseObjectDDL
		comparedSchema string
		want           string
	}{
		"view_only_base": {
			base:           []*driverV2.DatabaseObjectDDL{makeDDL("v1", driverV2.ObjectType_VIEW, viewBaseDDL)},
			compared:       nil,
			comparedSchema: "public",
			want:           viewBaseDDL,
		},
		"view_only_compared": {
			base:           nil,
			compared:       []*driverV2.DatabaseObjectDDL{makeDDL("v_drop", driverV2.ObjectType_VIEW, viewComparedDDL)},
			comparedSchema: "public",
			want:           "DROP VIEW IF EXISTS public.v_drop",
		},
		"view_both_diff": {
			base:           []*driverV2.DatabaseObjectDDL{makeDDL("v1", driverV2.ObjectType_VIEW, viewBaseDDL)},
			compared:       []*driverV2.DatabaseObjectDDL{makeDDL("v1", driverV2.ObjectType_VIEW, viewComparedDDL)},
			comparedSchema: "public",
			want:           viewBaseDDL, // PG 系幂等，无 DROP，直接 base 侧 CREATE OR REPLACE
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("base_schema", c.base, c.comparedSchema, c.compared)
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
			}
			if got[0] != c.want {
				t.Errorf("got = %q\nwant = %q", got[0], c.want)
			}
			// VIEW 的任一差异分类都**不应**包含 DROP TABLE WARNING
			if strings.Contains(got[0], expectedWarning) {
				t.Errorf("VIEW must not contain DROP TABLE WARNING; got=%q", got[0])
			}
			// VIEW only_base / both_diff 不应含 DROP；only_compared 则只含 DROP VIEW
			if name == "view_only_base" || name == "view_both_diff" {
				if strings.Contains(got[0], "DROP ") {
					t.Errorf("%s must not contain DROP; got=%q", name, got[0])
				}
			}
		})
	}
}

// ---- e) TestDiffer_FunctionBothDiff（含 function 3 种差异 + 参数签名） ----

// TestDiffer_FunctionBothDiff 覆盖 FUNCTION 三种差异分类（design §7.2 表 FUNCTION
// 列 + §7.4 参数签名 R-Q4-2）：
//   - function_only_base: base 侧 ObjectDDL 原文（CREATE OR REPLACE FUNCTION）
//   - function_only_compared_with_args: DROP FUNCTION IF EXISTS schema.fn(integer, text)
//   - function_both_diff: base 侧 ObjectDDL（无 DROP，PG 系幂等）
func TestDiffer_FunctionBothDiff(t *testing.T) {
	const (
		fnBaseDDL     = "CREATE OR REPLACE FUNCTION public.fn(integer, text) RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;"
		fnComparedDDL = "CREATE OR REPLACE FUNCTION public.fn(integer, text) RETURNS int AS $$ SELECT 2 $$ LANGUAGE SQL;"
	)

	cases := map[string]struct {
		base, compared []*driverV2.DatabaseObjectDDL
		comparedSchema string
		want           string
	}{
		"function_only_base": {
			base:           []*driverV2.DatabaseObjectDDL{makeDDL("fn(integer, text)", driverV2.ObjectType_FUNCTION, fnBaseDDL)},
			compared:       nil,
			comparedSchema: "public",
			want:           fnBaseDDL,
		},
		"function_only_compared_with_args": {
			base:           nil,
			compared:       []*driverV2.DatabaseObjectDDL{makeDDL("fn(integer, text)", driverV2.ObjectType_FUNCTION, fnComparedDDL)},
			comparedSchema: "public",
			want:           "DROP FUNCTION IF EXISTS public.fn(integer, text)",
		},
		"function_both_diff": {
			base:           []*driverV2.DatabaseObjectDDL{makeDDL("fn(integer, text)", driverV2.ObjectType_FUNCTION, fnBaseDDL)},
			compared:       []*driverV2.DatabaseObjectDDL{makeDDL("fn(integer, text)", driverV2.ObjectType_FUNCTION, fnComparedDDL)},
			comparedSchema: "public",
			want:           fnBaseDDL, // PG 系幂等
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("base_schema", c.base, c.comparedSchema, c.compared)
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
			}
			if got[0] != c.want {
				t.Errorf("got = %q\nwant = %q", got[0], c.want)
			}
		})
	}
}

// ---- f) TestDiffer_ProcedureBothDiff（含 procedure 3 种差异 + 参数签名） ----

// TestDiffer_ProcedureBothDiff 覆盖 PROCEDURE 三种差异分类（design §7.2 表
// PROCEDURE 列 + §7.4 参数签名 R-Q4-2）。
func TestDiffer_ProcedureBothDiff(t *testing.T) {
	const (
		procBaseDDL     = "CREATE OR REPLACE PROCEDURE public.proc(text) LANGUAGE plpgsql AS $$ BEGIN END; $$;"
		procComparedDDL = "CREATE OR REPLACE PROCEDURE public.proc(text) LANGUAGE plpgsql AS $$ BEGIN RAISE NOTICE 'x'; END; $$;"
	)

	cases := map[string]struct {
		base, compared []*driverV2.DatabaseObjectDDL
		comparedSchema string
		want           string
	}{
		"procedure_only_base": {
			base:           []*driverV2.DatabaseObjectDDL{makeDDL("proc(text)", driverV2.ObjectType_PROCEDURE, procBaseDDL)},
			compared:       nil,
			comparedSchema: "public",
			want:           procBaseDDL,
		},
		"procedure_only_compared_with_args": {
			base:           nil,
			compared:       []*driverV2.DatabaseObjectDDL{makeDDL("proc(text)", driverV2.ObjectType_PROCEDURE, procComparedDDL)},
			comparedSchema: "public",
			want:           "DROP PROCEDURE IF EXISTS public.proc(text)",
		},
		"procedure_both_diff": {
			base:           []*driverV2.DatabaseObjectDDL{makeDDL("proc(text)", driverV2.ObjectType_PROCEDURE, procBaseDDL)},
			compared:       []*driverV2.DatabaseObjectDDL{makeDDL("proc(text)", driverV2.ObjectType_PROCEDURE, procComparedDDL)},
			comparedSchema: "public",
			want:           procBaseDDL,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("base_schema", c.base, c.comparedSchema, c.compared)
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
			}
			if got[0] != c.want {
				t.Errorf("got = %q\nwant = %q", got[0], c.want)
			}
		})
	}
}

// ---- g) TestDiffer_FunctionDropWithArgs（design §17.1.4 line 880） ----

// TestDiffer_FunctionDropWithArgs 锁定 R-Q4-2：当 compared 侧出现两条同名不同
// 参数签名的 FUNCTION 重载时，differ 必须产出两条独立 DROP，每条 DROP 都带
// 完整参数签名（避免 PG 报 "function xxx is not unique"）。
//
// map 迭代顺序 Go runtime 不保证 → 用 sort.Strings 顺序无关断言。
func TestDiffer_FunctionDropWithArgs(t *testing.T) {
	t.Run("function_drop_with_args_two_overloads", func(t *testing.T) {
		compared := []*driverV2.DatabaseObjectDDL{
			makeDDL("fn(integer)", driverV2.ObjectType_FUNCTION,
				"CREATE OR REPLACE FUNCTION public.fn(integer) RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;"),
			makeDDL("fn(integer, text)", driverV2.ObjectType_FUNCTION,
				"CREATE OR REPLACE FUNCTION public.fn(integer, text) RETURNS int AS $$ SELECT 2 $$ LANGUAGE SQL;"),
		}

		got := GenerateModifySQLs("base_schema", nil, "public", compared)
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2; got=%v", len(got), got)
		}

		// 集合包含 + 顺序无关断言
		want := []string{
			"DROP FUNCTION IF EXISTS public.fn(integer)",
			"DROP FUNCTION IF EXISTS public.fn(integer, text)",
		}
		sortedGot := append([]string(nil), got...)
		sort.Strings(sortedGot)
		sortedWant := append([]string(nil), want...)
		sort.Strings(sortedWant)
		for i := range sortedWant {
			if sortedGot[i] != sortedWant[i] {
				t.Errorf("sortedGot[%d] = %q, want %q", i, sortedGot[i], sortedWant[i])
			}
		}

		// map key 唯一性兜底断言（避免重载被坍缩）
		uniq := make(map[string]bool)
		for _, s := range got {
			if uniq[s] {
				t.Errorf("duplicate ModifySQL %q; map key uniqueness broken", s)
			}
			uniq[s] = true
		}
	})
}

// ---- h) TestDiffer_WarningCommentInjection（design §17.1.4 line 881 / compat-RISK-5） ----

// TestDiffer_WarningCommentInjection 锁定 DROP TABLE WARNING 注释行字面值
// （byte-for-byte），任何标点 / 大小写 / 空格偏移都让测试失败。
//
// 兜底 impact_analysis.yml risk_points 第 4 条「DROP TABLE 兜底可能误删生产数据」。
func TestDiffer_WarningCommentInjection(t *testing.T) {
	t.Run("warning_comment_byte_for_byte", func(t *testing.T) {
		compared := []*driverV2.DatabaseObjectDDL{
			makeDDL("t1", driverV2.ObjectType_TABLE, "CREATE TABLE public.t1 (id int);"),
		}
		got := GenerateModifySQLs("base", nil, "public", compared)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
		}
		// 严格断言第一行字面值
		firstLine := strings.SplitN(got[0], "\n", 2)[0]
		if firstLine != expectedWarning {
			t.Errorf("first line = %q, want %q (byte-for-byte)", firstLine, expectedWarning)
		}
		// 拒绝尾随空白
		if strings.TrimRight(firstLine, " \t") != firstLine {
			t.Errorf("trailing whitespace in first line: %q", firstLine)
		}
		// 禁止 // 注释风格
		if strings.HasPrefix(firstLine, "//") {
			t.Errorf("first line must not use Go-style // comment; got %q", firstLine)
		}
		// 禁止 --! / -- ! 等带感叹号
		if strings.HasPrefix(firstLine, "--!") || strings.HasPrefix(firstLine, "-- !") {
			t.Errorf("first line must not contain extra punctuation; got %q", firstLine)
		}
	})
}

// ---- 增强覆盖（与 base 任务卡"额外建议"对齐） ----

// TestDiffer_TableBothSame_NoOutput ：两侧 ObjectDDL 完全相同 → 跳过，ModifySQLs
// 长度 0（design §7.2 矩阵「无变更不输出」语义）。
func TestDiffer_TableBothSame_NoOutput(t *testing.T) {
	const sameDDL = "CREATE TABLE public.t1 (id int);"
	cases := map[string]struct {
		base, compared []*driverV2.DatabaseObjectDDL
	}{
		"table_both_same": {
			base:     []*driverV2.DatabaseObjectDDL{makeDDL("t1", driverV2.ObjectType_TABLE, sameDDL)},
			compared: []*driverV2.DatabaseObjectDDL{makeDDL("t1", driverV2.ObjectType_TABLE, sameDDL)},
		},
		"view_both_same": {
			base:     []*driverV2.DatabaseObjectDDL{makeDDL("v1", driverV2.ObjectType_VIEW, "CREATE OR REPLACE VIEW public.v1 AS SELECT 1;")},
			compared: []*driverV2.DatabaseObjectDDL{makeDDL("v1", driverV2.ObjectType_VIEW, "CREATE OR REPLACE VIEW public.v1 AS SELECT 1;")},
		},
		"function_both_same": {
			base:     []*driverV2.DatabaseObjectDDL{makeDDL("fn()", driverV2.ObjectType_FUNCTION, "CREATE OR REPLACE FUNCTION public.fn() RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;")},
			compared: []*driverV2.DatabaseObjectDDL{makeDDL("fn()", driverV2.ObjectType_FUNCTION, "CREATE OR REPLACE FUNCTION public.fn() RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;")},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("public", c.base, "public", c.compared)
			if len(got) != 0 {
				t.Errorf("len(got) = %d, want 0; got=%v", len(got), got)
			}
		})
	}
}

// TestDiffer_MixedObjectTypes ：混合 4 类对象一次性输入，验证总数 + 集合包含。
// 用 sort.Strings 顺序无关断言。
func TestDiffer_MixedObjectTypes(t *testing.T) {
	t.Run("mixed_4_types", func(t *testing.T) {
		const (
			tableBase    = "CREATE TABLE public.t1 (id int);"
			viewBase     = "CREATE OR REPLACE VIEW public.v1 AS SELECT 1 AS id;"
			fnCompared   = "CREATE OR REPLACE FUNCTION public.fn(integer) RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;"
			procBase     = "CREATE OR REPLACE PROCEDURE public.proc(text) LANGUAGE plpgsql AS $$ BEGIN END; $$;"
			procCompared = "CREATE OR REPLACE PROCEDURE public.proc(text) LANGUAGE plpgsql AS $$ BEGIN RAISE NOTICE 'x'; END; $$;"
		)

		base := []*driverV2.DatabaseObjectDDL{
			makeDDL("t1", driverV2.ObjectType_TABLE, tableBase),
			makeDDL("v1", driverV2.ObjectType_VIEW, viewBase),
			makeDDL("proc(text)", driverV2.ObjectType_PROCEDURE, procBase),
		}
		compared := []*driverV2.DatabaseObjectDDL{
			makeDDL("fn(integer)", driverV2.ObjectType_FUNCTION, fnCompared),
			makeDDL("proc(text)", driverV2.ObjectType_PROCEDURE, procCompared),
		}

		got := GenerateModifySQLs("base_schema", base, "compared_schema", compared)
		// 期望 4 条 ModifySQL：
		//   - t1 仅 base → tableBase 原文
		//   - v1 仅 base → viewBase 原文
		//   - fn(integer) 仅 compared → DROP FUNCTION ...
		//   - proc(text) 两侧不同 → procBase（PG 幂等）
		if len(got) != 4 {
			t.Fatalf("len(got) = %d, want 4; got=%v", len(got), got)
		}

		wantContains := map[string]bool{
			tableBase: false,
			viewBase:  false,
			"DROP FUNCTION IF EXISTS compared_schema.fn(integer)": false,
			procBase: false,
		}
		for _, s := range got {
			if _, ok := wantContains[s]; ok {
				wantContains[s] = true
			}
		}
		for k, hit := range wantContains {
			if !hit {
				t.Errorf("expected ModifySQL %q not found in got=%v", k, got)
			}
		}
	})
}

// TestDiffer_UnsupportedObjectTypeIgnored ：differ 收到 INDEX / TRIGGER 等
// 本期不支持的 ObjectType 时静默跳过（design §7.5 兜底 + 防御性），不 panic、
// 不进入输出。
func TestDiffer_UnsupportedObjectTypeIgnored(t *testing.T) {
	cases := map[string]struct {
		base, compared []*driverV2.DatabaseObjectDDL
	}{
		"index_only_base": {
			base: []*driverV2.DatabaseObjectDDL{makeDDL("idx1", "INDEX", "CREATE INDEX idx1 ON public.t1(id);")},
		},
		"trigger_only_compared": {
			compared: []*driverV2.DatabaseObjectDDL{makeDDL("trg1", "TRIGGER", "CREATE TRIGGER trg1 ...")},
		},
		"index_both_diff": {
			base:     []*driverV2.DatabaseObjectDDL{makeDDL("idx1", "INDEX", "CREATE INDEX idx1 ON public.t1(id);")},
			compared: []*driverV2.DatabaseObjectDDL{makeDDL("idx1", "INDEX", "CREATE INDEX idx1 ON public.t1(id, name);")},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("public", c.base, "public", c.compared)
			if len(got) != 0 {
				t.Errorf("unsupported ObjectType must be ignored; got=%v", got)
			}
		})
	}
}

// TestDiffer_NilAndEmptyDefensive ：防御性测试 nil 边界。
//   - 双侧 nil / 空 slice → 空输出
//   - base/compared 含 nil 元素 → 跳过 nil，不 panic
//   - 含 nil DatabaseObject 字段 → 跳过，不 panic
//
// 注：differ 入参语义约定为"已过滤好的真实存在对象列表"，extractor 上游
// 兜底（design §6.5 「对象不存在 → 占位 ObjectDDL=""」）由 sqle-ee 业务层
// compareSchema 在调用 differ 之前过滤掉占位条目；differ 层不再二次识别
// ObjectDDL==""，避免双重语义入口（如果 compared 侧 ObjectDDL=="" 进入
// differ，会被当作真实对象生成 DROP，这是上游应该兜底的状态而非 differ
// 的失误）。
func TestDiffer_NilAndEmptyDefensive(t *testing.T) {
	cases := map[string]struct {
		base, compared []*driverV2.DatabaseObjectDDL
		wantLen        int
	}{
		"both_nil": {wantLen: 0},
		"both_empty_slice": {
			base:     []*driverV2.DatabaseObjectDDL{},
			compared: []*driverV2.DatabaseObjectDDL{},
			wantLen:  0,
		},
		"base_has_nil_element_skip": {
			base:    []*driverV2.DatabaseObjectDDL{nil},
			wantLen: 0,
		},
		"compared_has_nil_databaseobject_skip": {
			compared: []*driverV2.DatabaseObjectDDL{
				{DatabaseObject: nil, ObjectDDL: "junk"},
			},
			wantLen: 0,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateModifySQLs("public", c.base, "public", c.compared)
			if len(got) != c.wantLen {
				t.Errorf("len(got) = %d, want %d; got=%v", len(got), c.wantLen, got)
			}
		})
	}
}

// ---- i) TestDiffer_RewriteSchemaQualifier (Task-Test-Fix-001 P1.2) ----

// TestRewriteSchemaQualifier 直接断言 helper：baseSchema. 前缀被替换为
// comparedSchema.；自对比与空 baseSchema 走 no-op。
func TestRewriteSchemaQualifier(t *testing.T) {
	cases := map[string]struct {
		ddl, base, compared, want string
	}{
		"view_create_or_replace_qualified": {
			ddl:      "CREATE OR REPLACE VIEW test_2905_base.v_user_summary AS SELECT id FROM test_2905_base.t_user;",
			base:     "test_2905_base",
			compared: "test_2905_compared",
			want:     "CREATE OR REPLACE VIEW test_2905_compared.v_user_summary AS SELECT id FROM test_2905_compared.t_user;",
		},
		"function_qualified": {
			ddl:      "CREATE OR REPLACE FUNCTION test_2905_base.fn(integer) RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;",
			base:     "test_2905_base",
			compared: "test_2905_compared",
			want:     "CREATE OR REPLACE FUNCTION test_2905_compared.fn(integer) RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;",
		},
		"self_compare_noop": {
			ddl:      "CREATE OR REPLACE VIEW public.v AS SELECT 1;",
			base:     "public",
			compared: "public",
			want:     "CREATE OR REPLACE VIEW public.v AS SELECT 1;",
		},
		"empty_base_schema_noop": {
			ddl:      "CREATE TABLE x;",
			base:     "",
			compared: "compared",
			want:     "CREATE TABLE x;",
		},
		"no_match_noop": {
			ddl:      "CREATE OR REPLACE VIEW public.v AS SELECT 1;",
			base:     "other_schema",
			compared: "compared_schema",
			want:     "CREATE OR REPLACE VIEW public.v AS SELECT 1;",
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			got := rewriteSchemaQualifier(c.ddl, c.base, c.compared)
			if got != c.want {
				t.Errorf("got = %q\nwant = %q", got, c.want)
			}
		})
	}
}

// TestGenOnlyBaseSide_RewriteViewSchema 端到端验证 P1.2 修复：VIEW 仅 base 侧
// 有时，输出的 CREATE OR REPLACE VIEW schema 限定改写为 compared 侧。
func TestGenOnlyBaseSide_RewriteViewSchema(t *testing.T) {
	baseSchema := "test_2905_base"
	comparedSchema := "test_2905_compared"
	const viewDDL = "CREATE OR REPLACE VIEW test_2905_base.v_user_summary AS SELECT t_user.id, t_user.name FROM test_2905_base.t_user;"

	base := []*driverV2.DatabaseObjectDDL{
		makeDDL("v_user_summary", driverV2.ObjectType_VIEW, viewDDL),
	}
	got := GenerateModifySQLs(baseSchema, base, comparedSchema, nil)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
	}
	if strings.Contains(got[0], "test_2905_base.") {
		t.Errorf("base schema qualifier must be rewritten; got=%q", got[0])
	}
	if !strings.Contains(got[0], "test_2905_compared.v_user_summary") {
		t.Errorf("compared schema must appear in rewritten DDL; got=%q", got[0])
	}
	if !strings.Contains(got[0], "CREATE OR REPLACE VIEW") {
		t.Errorf("expected CREATE OR REPLACE VIEW; got=%q", got[0])
	}
}

// TestGenBothDiff_RewriteFunctionSchema 验证 P1.2 修复对 FUNCTION 两侧都有但
// DDL 不同分支的 schema 改写。
func TestGenBothDiff_RewriteFunctionSchema(t *testing.T) {
	baseSchema := "base_s"
	comparedSchema := "compared_s"
	const fnBase = "CREATE OR REPLACE FUNCTION base_s.fn(integer) RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;"
	const fnCompared = "CREATE OR REPLACE FUNCTION compared_s.fn(integer) RETURNS int AS $$ SELECT 2 $$ LANGUAGE SQL;"

	base := []*driverV2.DatabaseObjectDDL{
		makeDDL("fn(integer)", driverV2.ObjectType_FUNCTION, fnBase),
	}
	compared := []*driverV2.DatabaseObjectDDL{
		makeDDL("fn(integer)", driverV2.ObjectType_FUNCTION, fnCompared),
	}
	got := GenerateModifySQLs(baseSchema, base, comparedSchema, compared)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1; got=%v", len(got), got)
	}
	if strings.Contains(got[0], "base_s.fn") {
		t.Errorf("base schema qualifier must be rewritten; got=%q", got[0])
	}
	if !strings.Contains(got[0], "compared_s.fn(integer)") {
		t.Errorf("compared schema must appear in rewritten DDL; got=%q", got[0])
	}
}

// TestGenBothDiff_SameDDLOverload_NoOutput (P1.3 验证) 双侧同 ObjectName 且
// DDL 完全相同 → ModifySQLs 长度 0（短路过滤）。
func TestGenBothDiff_SameDDLOverload_NoOutput(t *testing.T) {
	const fnDDL = "CREATE OR REPLACE FUNCTION s.fn(integer) RETURNS int AS $$ SELECT 1 $$ LANGUAGE SQL;"

	base := []*driverV2.DatabaseObjectDDL{
		makeDDL("fn(integer)", driverV2.ObjectType_FUNCTION, fnDDL),
	}
	compared := []*driverV2.DatabaseObjectDDL{
		makeDDL("fn(integer)", driverV2.ObjectType_FUNCTION, fnDDL),
	}
	got := GenerateModifySQLs("s", base, "s", compared)
	if len(got) != 0 {
		t.Errorf("same DDL overload must be filtered; got=%v", got)
	}
}

// TestGenOnlyBaseSide_RewriteTableSearchPath 覆盖 Task-Test-Fix-002 P2-A：
// "仅 base 侧有"的 TABLE 输出必须把 pg_get_tabledef 首行
// `SET search_path = baseSchema` 改写为 `SET search_path = comparedSchema`，
// 否则后续裸表名 CREATE TABLE 会落到 base schema 撞已存在同名表
// （case-3-1.md 缺陷溯源）。
func TestGenOnlyBaseSide_RewriteTableSearchPath(t *testing.T) {
	const baseDDL = "SET search_path = test_2905_base;\n" +
		"CREATE TABLE t_order (\n" +
		"    id integer NOT NULL,\n" +
		"    amount numeric(10,2)\n" +
		")\n" +
		"WITH (orientation=row, compression=no);\n" +
		"ALTER TABLE t_order ADD CONSTRAINT t_order_pkey PRIMARY KEY (id);"

	base := []*driverV2.DatabaseObjectDDL{
		makeDDL("t_order", driverV2.ObjectType_TABLE, baseDDL),
	}
	got := GenerateModifySQLs("test_2905_base", base, "test_2905_compared", nil)
	if len(got) != 1 {
		t.Fatalf("expect 1 modify SQL, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "SET search_path = test_2905_compared") {
		t.Errorf("expect SET search_path rewritten to compared schema; got=%q", got[0])
	}
	// search_path 必须只有 compared，base 字面值不应出现在 SET 行（容忍其他
	// 位置如表名 / 字段不带 base schema 前缀，因为 pg_get_tabledef 输出 TABLE
	// 名是裸表名，不会含 base schema 字面）。
	for _, line := range strings.Split(got[0], "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "SET ") && strings.Contains(trim, "test_2905_base") {
			t.Errorf("SET line still references base schema: %q", trim)
		}
	}
}

// TestGenBothDiff_RewriteTableSearchPath 覆盖 Task-Test-Fix-002 P2-A：
// "两侧都有但 DDL 不同"的 TABLE 输出必须同样改写 SET search_path 到 compared，
// 并保留 WARNING + DROP IF EXISTS comparedSchema.name 前缀（case-2-1.md 双侧
// 都有 t_user 但 base 多列的场景）。
func TestGenBothDiff_RewriteTableSearchPath(t *testing.T) {
	const baseDDL = "SET search_path = test_2905_base;\n" +
		"CREATE TABLE t_user (\n" +
		"    id integer NOT NULL,\n" +
		"    name text,\n" +
		"    email text\n" +
		")\n" +
		"WITH (orientation=row, compression=no);"
	const comparedDDL = "SET search_path = test_2905_compared;\n" +
		"CREATE TABLE t_user (\n" +
		"    id integer NOT NULL,\n" +
		"    name text\n" +
		")\n" +
		"WITH (orientation=row, compression=no);"

	base := []*driverV2.DatabaseObjectDDL{
		makeDDL("t_user", driverV2.ObjectType_TABLE, baseDDL),
	}
	compared := []*driverV2.DatabaseObjectDDL{
		makeDDL("t_user", driverV2.ObjectType_TABLE, comparedDDL),
	}

	got := GenerateModifySQLs("test_2905_base", base, "test_2905_compared", compared)
	if len(got) != 1 {
		t.Fatalf("expect 1 modify SQL, got %d: %v", len(got), got)
	}
	out := got[0]
	if !strings.HasPrefix(out, expectedWarning) {
		t.Errorf("expect WARNING comment as first line; got=%q", out)
	}
	if !strings.Contains(out, "DROP TABLE IF EXISTS test_2905_compared.t_user;") {
		t.Errorf("expect DROP TABLE IF EXISTS compared schema; got=%q", out)
	}
	if !strings.Contains(out, "SET search_path = test_2905_compared") {
		t.Errorf("expect SET search_path rewritten to compared schema; got=%q", out)
	}
	// SET 行不能再含 base schema 字面（容忍 DROP TABLE 之前的"comparedSchema"
	// 出现，那是合法的限定）。
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "SET ") && strings.Contains(trim, "test_2905_base") {
			t.Errorf("SET line still references base schema: %q", trim)
		}
	}
}

// TestGenBothDiff_USTORENormalize_NoOutput 覆盖 Task-Test-Fix-002 P4：
// 两侧 DDL 仅 `storage_type=USTORE` / `storage_type=ustore` 大小写差异时，
// 短路跳过，不输出冗余 modify SQL（case-2-1.md 遗留次要缺陷溯源）。
func TestGenBothDiff_USTORENormalize_NoOutput(t *testing.T) {
	const baseDDL = "SET search_path = test_2905_base;\n" +
		"CREATE TABLE t (id integer)\n" +
		"WITH (orientation=row, storage_type=USTORE, compression=no);"
	const comparedDDL = "SET search_path = test_2905_compared;\n" +
		"CREATE TABLE t (id integer)\n" +
		"WITH (orientation=row, storage_type=ustore, compression=no);"

	base := []*driverV2.DatabaseObjectDDL{
		makeDDL("t", driverV2.ObjectType_TABLE, baseDDL),
	}
	compared := []*driverV2.DatabaseObjectDDL{
		makeDDL("t", driverV2.ObjectType_TABLE, comparedDDL),
	}
	got := GenerateModifySQLs("test_2905_base", base, "test_2905_compared", compared)
	if len(got) != 0 {
		t.Errorf("USTORE/ustore case-only differences must be filtered; got=%v", got)
	}
}

// TestGenBothDiff_SameDDLDifferentSchema_NoOutput 覆盖 Task-Test-Fix-002 P5：
// 两侧 DDL 仅 schema 限定不同（base 写 base.fn vs compared 写 compared.fn）、
// body 完全一致时，differ 在字符串短路前先做 schema 替换再比较 → 等价 →
// 短路跳过，不输出冗余 CREATE OR REPLACE（case-2-3.md 遗留次要缺陷溯源）。
func TestGenBothDiff_SameDDLDifferentSchema_NoOutput(t *testing.T) {
	const baseDDL = "CREATE OR REPLACE FUNCTION test_2905_base.fn_calc(p integer)\n" +
		"RETURNS integer LANGUAGE plpgsql AS $function$ BEGIN RETURN p * 2; END; $function$;"
	const comparedDDL = "CREATE OR REPLACE FUNCTION test_2905_compared.fn_calc(p integer)\n" +
		"RETURNS integer LANGUAGE plpgsql AS $function$ BEGIN RETURN p * 2; END; $function$;"

	base := []*driverV2.DatabaseObjectDDL{
		makeDDL("fn_calc(integer)", driverV2.ObjectType_FUNCTION, baseDDL),
	}
	compared := []*driverV2.DatabaseObjectDDL{
		makeDDL("fn_calc(integer)", driverV2.ObjectType_FUNCTION, comparedDDL),
	}
	got := GenerateModifySQLs("test_2905_base", base, "test_2905_compared", compared)
	if len(got) != 0 {
		t.Errorf("same DDL with only schema-qualifier difference must be filtered; got=%v", got)
	}
}

// TestRewriteSetSearchPath_GuardRails 覆盖 P2-A helper 的边界：
//
//   - baseSchema 为空 / 与 compared 相同 → no-op
//   - SET 行不匹配 baseSchema 字面值 → 保留原文（不应误改）
//   - 缺末尾分号 / 容忍空白
func TestRewriteSetSearchPath_GuardRails(t *testing.T) {
	cases := map[string]struct {
		in       string
		base     string
		compared string
		want     string
	}{
		"empty_base_short_circuit": {
			in:       "SET search_path = anyschema;\nCREATE TABLE x (id int);",
			base:     "",
			compared: "compared",
			want:     "SET search_path = anyschema;\nCREATE TABLE x (id int);",
		},
		"same_schema_short_circuit": {
			in:       "SET search_path = s;\nCREATE TABLE x (id int);",
			base:     "s",
			compared: "s",
			want:     "SET search_path = s;\nCREATE TABLE x (id int);",
		},
		"normal_rewrite_with_semicolon": {
			in:       "SET search_path = base;\nCREATE TABLE x (id int);",
			base:     "base",
			compared: "compared",
			want:     "SET search_path = compared;\nCREATE TABLE x (id int);",
		},
		"normal_rewrite_without_semicolon": {
			in:       "SET search_path = base\nCREATE TABLE x (id int);",
			base:     "base",
			compared: "compared",
			want:     "SET search_path = compared\nCREATE TABLE x (id int);",
		},
		"set_line_not_match_base_literal": {
			in:       "SET search_path = other;\nCREATE TABLE x (id int);",
			base:     "base",
			compared: "compared",
			want:     "SET search_path = other;\nCREATE TABLE x (id int);",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := rewriteSetSearchPath(c.in, c.base, c.compared)
			if got != c.want {
				t.Errorf("rewriteSetSearchPath(%q,%q,%q):\n got: %q\nwant: %q", c.in, c.base, c.compared, got, c.want)
			}
		})
	}
}
