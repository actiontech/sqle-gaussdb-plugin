// Package driver provides the GaussDB and openGauss driverV2.Driver
// implementations for sqle-gaussdb-plugin.
//
// File layout (Task-Dev-004, plan §4.1):
//
//   - driver_common.go   shared Driver struct, connection-pool policy and the
//                        wrapPGError SQLSTATE passthrough helper used by both
//                        variants.
//   - driver_gaussdb.go  GaussDBDriver aggregation + NewGaussDBDriverImpl
//                        factory (PluginName="GaussDB").
//   - driver_opengauss.go OpenGaussDriver aggregation + NewOpenGaussDriverImpl
//                        factory (PluginName="openGauss").
//
// Both factories keep the same signature as the previous stub.go so that
// cmd/sqle-gaussdb-plugin/main.go and cmd/sqle-opengauss-plugin/main.go
// continue to compile without any change.
package driver

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"time"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	hclog "github.com/hashicorp/go-hclog"

	"github.com/actiontech/sqle-gaussdb-plugin/internal/differ"
	"github.com/actiontech/sqle-gaussdb-plugin/internal/extractor"
)

// stripObjectNameSignature 把 plugin gRPC 入参中的 ObjectName 字符串剥除参数
// 签名部分，返回 bare 名（pg_proc.proname 字面值，无括号 / 参数列表）。
//
// 背景（Task-Test-Fix-002 P2-B；
// expertise_docs/episodic/sqle_gaussdb_modify_sql_object_name_signature_mismatch_20260521.md）：
//
//   - sqle-ee 业务层 / dms-ui-ee UI 主路径在调 plugin
//     GetDatabaseDiffModifySQL / GetDatabaseObjectDDL 时，会把树中显示文本
//     （含参数签名，如 `fn_calc(p integer)` / `proc_alert(p TEXT)`）作为
//     ObjectName 透传给 plugin；
//   - plugin extractor 内部 extractFunction / extractProcedure 用 ObjectName
//     作为 `pg_proc.proname = $2` 的绑定参数查询；pg_proc.proname 字面值
//     永远是 bare name（PG 系数据源系统目录约定），含括号 / 参数签名的
//     字符串永远查不到任何行，被走"占位 ObjectDDL=空"分支；
//   - 后果是 base / compared 两侧都返回空 DDL 占位，differ 比对得到
//     modify_sqls=[]，UI 抽屉显示"暂无数据"（case-3-3.md / case-3-4.md）。
//
// 修复策略：plugin 入口前置剥签名 → 用 bare name 查 pg_proc → 命中 N 个重载
// → 每条 ObjectName 由 plugin extractor 重新拼为 `fn_calc(<arg_types>)` 规范
// 形态（pg_get_function_arguments 输出）。这样：
//
//   - base / compared 两侧 normalize 后 ObjectName 一致（规范签名 key），
//     differ map 比对正确；
//   - 重载场景天然分裂为多条 entry，不存在 ambiguous 问题（与 R-Q4-2 重载
//     语义对齐）；
//   - 兼容用户传 bare 名（不带括号）的 API 旁路调用（IndexByte 返回 -1 → 原
//     样返回）。
//
// 切分策略：以 `(` 为分界点，取前缀部分；不剥除 schema 前缀（plugin 入口
// 的 ObjectName 不带 schema，schema 由 SchemaName 字段单独承载）。
func stripObjectNameSignature(s string) string {
	idx := strings.IndexByte(s, '(')
	if idx < 0 {
		return s
	}
	// 用 TrimRight 容忍 `fn_calc (p int)` 这样在括号前带空格的形态（虽然
	// dms-ui-ee 实际不会这么传，但稳健性需要 defensive）。
	return strings.TrimRight(s[:idx], " \t")
}

// normalizeObjectsForExtractor 把入参对象列表中的 ObjectName 剥签名后返回
// 新的列表，原列表 / 原 *DatabaseObject 指针不修改（避免影响调用方持有的
// proto 消息状态）。
//
// 仅 FUNCTION / PROCEDURE 才需要剥签名；TABLE / VIEW 的 ObjectName 不含
// 括号，IndexByte 返回 -1，stripObjectNameSignature 短路返回原值，所以全量
// normalize 不影响 TABLE / VIEW 路径。
func normalizeObjectsForExtractor(objs []*driverV2.DatabaseObject) []*driverV2.DatabaseObject {
	if len(objs) == 0 {
		return objs
	}
	out := make([]*driverV2.DatabaseObject, 0, len(objs))
	for _, obj := range objs {
		if obj == nil {
			out = append(out, obj)
			continue
		}
		bare := stripObjectNameSignature(obj.ObjectName)
		if bare == obj.ObjectName {
			out = append(out, obj)
			continue
		}
		// 拷贝一份新值，不修改原 proto 消息。
		cp := *obj
		cp.ObjectName = bare
		out = append(out, &cp)
	}
	return out
}

// Connection-pool policy shared by the GaussDB and openGauss variants. The
// values match sqle-pg-plugin's runtime profile so the new plugin does not
// introduce a different pool-behaviour footprint (compat-RISK-2; design §5.1
// line 328).
const (
	maxOpenConns    = 10
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
)

// Driver is the shared backing struct embedded by both GaussDBDriver and
// OpenGaussDriver. By embedding *driverPkg.DriverImpl we inherit the default
// implementation of every driverV2.Driver method (Close / Ping / Exec / Audit /
// Parse / Query / ... — see sqle/sqle/pkg/driver/impl.go). The variants only
// override the methods that need custom behaviour, which keeps the surface
// area minimal and unblocks the capability matrix declared by cmd/* main.go.
//
// Variant is the human-readable PluginName ("GaussDB" / "openGauss") used by
// unit tests to assert that the two factory functions produce two structurally
// distinct Driver instances (design §17.1.5 — PluginNameRegistration).
type Driver struct {
	*driverPkg.DriverImpl
	Variant string
}

// Compile-time assertion: *Driver must continue to satisfy driverV2.Driver.
// If the upstream interface grows a new method, the embedded DriverImpl is
// expected to provide a default and this assertion catches the gap early.
var _ driverV2.Driver = (*Driver)(nil)

// newCommon constructs the shared Driver value used by both variants.
//
// It delegates to driverPkg.NewDriverImpl so that the sqle main repo's
// connection bootstrap path (Dialector.Open → *sql.DB + *sql.Conn) is reused
// verbatim. Once DriverImpl has the *sql.DB in hand we apply the plugin's
// connection-pool policy on top (design §5.1 line 328).
//
// variant is the consts.PluginName* string used for diagnostics and unit-test
// assertions; it does not influence connection behaviour because the libpq
// DSN built by Dialector.BuildDSN already encodes the variant-specific defaults.
func newCommon(
	l hclog.Logger,
	dt driverPkg.Dialector,
	ah *driverPkg.AuditHandler,
	cfg *driverV2.Config,
	variant string,
) (*Driver, error) {
	rawDriver, err := driverPkg.NewDriverImpl(l, dt, ah, cfg)
	if err != nil {
		return nil, err
	}
	impl, ok := rawDriver.(*driverPkg.DriverImpl)
	if !ok {
		// Defensive guard: sqle main repo always returns *DriverImpl here, but
		// asserting it explicitly keeps the type contract local to this file.
		return nil, errors.New(
			"sqle-gaussdb-plugin: NewDriverImpl did not return *driverPkg.DriverImpl")
	}
	applyPoolPolicy(impl.DB)
	return &Driver{DriverImpl: impl, Variant: variant}, nil
}

// applyPoolPolicy sets the shared connection-pool limits on db. It is a no-op
// when db is nil so that the cfg.DSN==nil path (driverPkg.NewDriverImpl
// returns a Driver with no underlying *sql.DB — see impl.go:37-39) does not
// panic.
func applyPoolPolicy(db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
}

// Ping overrides the embedded DriverImpl.Ping so that the plugin always pings
// through *sql.DB rather than *sql.Conn. design §5.1 关键片段 mandates this
// shape; the DB path also lets the connect_timeout DSN parameter (set by
// dialector.BuildDSN, exploration §1.4 R-Q1-1) actually bound the wait.
//
// When the underlying *sql.DB has not been initialised (cfg.DSN == nil at
// driver construction time, e.g. during unit tests that exercise only the
// capability matrix), we surface a clear error instead of dereferencing a nil
// pointer.
func (d *Driver) Ping(ctx context.Context) error {
	if d == nil || d.DriverImpl == nil || d.DriverImpl.DB == nil {
		return errors.New("sqle-gaussdb-plugin: Ping called before database connection initialised")
	}
	return wrapPGError(d.DriverImpl.DB.PingContext(ctx))
}

// wrapPGError surfaces openGauss / GaussDB SQLSTATE codes verbatim to upper
// layers (design §5.3). The strategy is:
//
//   - If err is nil, return nil.
//   - If err is a *pq.Error from gitee.com/opengauss/openGauss-connector-go-pq
//     (the only sql driver this plugin ever registers), return it unchanged.
//     The lib/pq-compatible Error type already implements `error` with a
//     "pq: <Message>" string and the .Code field exposes the raw 5-character
//     SQLSTATE for callers that want to branch on it.
//   - For any other error type (network failure, context cancellation, etc.)
//     return the error unchanged. We deliberately do not classify or
//     translate non-pq errors so that the original cause survives the trip
//     through plugin gRPC.
//
// Implementation note: we type-detect the pq.Error via reflect on the type
// name so this file does not need a direct import of
// gitee.com/opengauss/openGauss-connector-go-pq (that import lives in cmd/*
// for the side-effect sql.Register and in internal/dialector for the same
// reason). Keeping the driver/ package free of the connector dependency keeps
// the unit-test surface small — sqlmock-backed tests do not need to vendor
// the openGauss connector to satisfy the wrapPGError code path.
func wrapPGError(err error) error {
	if err == nil {
		return nil
	}
	// Walk the wrap chain looking for an *openGauss pq.Error (or the
	// lib/pq.Error fork that shares the same exported type name and field
	// shape — design §5.3 explicitly relies on this structural compatibility).
	cur := err
	for cur != nil {
		t := reflect.TypeOf(cur)
		if t != nil && t.Kind() == reflect.Ptr {
			elem := t.Elem()
			if elem.Kind() == reflect.Struct && elem.Name() == "Error" {
				// Best-effort check: the openGauss / lib-pq Error type is
				// identified by package path + type name. We only need it to
				// pass through unchanged, so once we recognise it we return
				// the original error verbatim.
				pkg := elem.PkgPath()
				if pkg == "gitee.com/opengauss/openGauss-connector-go-pq" ||
					pkg == "github.com/lib/pq" {
					return err
				}
			}
		}
		next := errors.Unwrap(cur)
		if next == nil {
			break
		}
		cur = next
	}
	return err
}

// GetDatabaseObjectDDL delegates to internal/extractor (Task-Dev-005 / 006).
// See plan §4.2 / §6.1: the per-schema iteration constructs a single shared
// *extractor.Extractor (one *sql.DB scope per Driver instance) and calls
// Extract once per *driverV2.DatabaseSchemaInfo, appending each result into
// the returned slice.
//
// Defensive guard: when the embedded *sql.DB has not been initialised (e.g.
// cfg.DSN==nil capability-probe path), surface a clear error rather than
// dereferencing nil — same form as Ping above.
//
// Error propagation: a single extract failure returns (nil, err) immediately;
// callers receive the first failure as the deterministic outcome and the
// plugin gRPC layer surfaces it to dms-ui-ee.
//
// Defining this method on *Driver (rather than overriding on
// GaussDBDriver / OpenGaussDriver) means both variants share one body via
// embedding. The previous per-variant overrides that returned the
// errNotImplemented sentinel have been removed by Task-Dev-008.
func (d *Driver) GetDatabaseObjectDDL(
	ctx context.Context,
	objInfos []*driverV2.DatabaseSchemaInfo,
) ([]*driverV2.DatabaseSchemaObjectResult, error) {
	if d == nil || d.DriverImpl == nil || d.DriverImpl.DB == nil {
		return nil, errors.New(
			"sqle-gaussdb-plugin: GetDatabaseObjectDDL called before database connection initialised")
	}

	// One Extractor per call site is fine — construction is cheap and the
	// receiver only retains *sql.DB; reusing it across the loop keeps the
	// single-instance semantic explicit.
	ex := extractor.NewExtractor(d.DriverImpl.DB)

	results := make([]*driverV2.DatabaseSchemaObjectResult, 0, len(objInfos))
	for _, info := range objInfos {
		if info == nil {
			continue
		}
		// Task-Test-Fix-002 P2-B: 入口剥签名，让带签名的 FUNCTION/PROCEDURE
		// ObjectName（如 `fn_calc(p integer)`）也能命中 pg_proc.proname。
		// 该 normalize 仅作用于 extractor 入参，不影响 result.SchemaName /
		// 返回的 ObjectName（由 extractor 内部按 pg_get_function_arguments
		// 输出重新拼接）。
		normInfo := &driverV2.DatabaseSchemaInfo{
			SchemaName:      info.SchemaName,
			DatabaseObjects: normalizeObjectsForExtractor(info.DatabaseObjects),
		}
		result, err := ex.Extract(ctx, normInfo)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// GetDatabaseDiffModifySQL delegates to internal/extractor + internal/differ
// (Task-Dev-005 / 006 / 007). See plan §4.3 / §6.1.
//
// For each *driverV2.DatabasCompareSchemaInfo we run extractor.Extract twice
// (base side then compared side) over the SAME DatabaseObjects list — the
// real chain ships base/compared schema names alongside one shared object
// list, so the driver layer destructures that into two extractor calls that
// only differ in SchemaName. The two results feed differ.GenerateModifySQLs
// (Task-Dev-007 4-arg signature) to produce the ModifySQLs string slice.
//
// SchemaName on the returned DatabaseDiffModifySQLResult is set to
// info.BaseSchemaName per design §7 / impact_analysis.yml line 226 — business
// layer organises the comparison tree by base schema. The compared schema
// name is already encoded into DROP statements' schema qualifier inside
// differ output, so we deliberately do not surface it on the outer result.
//
// calibratedDSN: not used this period. design §5 / plan §4 does not require
// rebuilding the database connection at differ time — d.DriverImpl.DB already
// holds the target-instance connection established at construction. The
// parameter is retained on the signature (interface contract) and explicitly
// discarded with `_ = calibratedDSN` so go vet's unused-parameter check stays
// silent; future cross-instance compare scenarios may consume it.
//
// Defensive guard mirrors GetDatabaseObjectDDL: nil DB → explicit error.
//
// Error propagation: any extract failure (base or compared side) returns
// (nil, err) immediately. We do not partially fail / partially succeed —
// callers see a deterministic single-error outcome.
func (d *Driver) GetDatabaseDiffModifySQL(
	ctx context.Context,
	calibratedDSN *driverV2.DSN,
	objInfos []*driverV2.DatabasCompareSchemaInfo,
) ([]*driverV2.DatabaseDiffModifySQLResult, error) {
	if d == nil || d.DriverImpl == nil || d.DriverImpl.DB == nil {
		return nil, errors.New(
			"sqle-gaussdb-plugin: GetDatabaseDiffModifySQL called before database connection initialised")
	}
	// reserved for future cross-instance compare; this period uses the
	// embedded *sql.DB exclusively (single-instance both-side compare).
	_ = calibratedDSN

	ex := extractor.NewExtractor(d.DriverImpl.DB)

	results := make([]*driverV2.DatabaseDiffModifySQLResult, 0, len(objInfos))
	for _, info := range objInfos {
		if info == nil {
			continue
		}

		// Task-Test-Fix-002 P2-B: 入口剥签名（同 GetDatabaseObjectDDL）。
		// UI 主路径会把树中显示文本 `fn_calc(p integer)` 作为 ObjectName
		// 透传给 plugin，必须在调 extractor 之前剥签名，否则
		// pg_proc.proname 查询命中 0 行，走"占位空 DDL"分支，differ 比对
		// 输出 modify_sqls=[]（case-3-3.md / case-3-4.md 缺陷溯源）。
		normObjs := normalizeObjectsForExtractor(info.DatabaseObjects)

		baseInfo := &driverV2.DatabaseSchemaInfo{
			SchemaName:      info.BaseSchemaName,
			DatabaseObjects: normObjs,
		}
		baseResult, err := ex.Extract(ctx, baseInfo)
		if err != nil {
			return nil, err
		}

		comparedInfo := &driverV2.DatabaseSchemaInfo{
			SchemaName:      info.ComparedSchemaName,
			DatabaseObjects: normObjs,
		}
		comparedResult, err := ex.Extract(ctx, comparedInfo)
		if err != nil {
			return nil, err
		}

		sqls := differ.GenerateModifySQLs(
			baseResult.SchemaName,
			baseResult.DatabaseObjectDDLs,
			comparedResult.SchemaName,
			comparedResult.DatabaseObjectDDLs,
		)

		results = append(results, &driverV2.DatabaseDiffModifySQLResult{
			SchemaName: info.BaseSchemaName,
			ModifySQLs: sqls,
		})
	}
	return results, nil
}
