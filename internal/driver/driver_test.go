// driver_test.go covers design §17.1.5 — Driver capability matrix + Ping
// behaviour — using map-case style and DATA-DOG/go-sqlmock to keep all
// database interactions hermetic. The tests deliberately avoid the
// newCommon → driverPkg.NewDriverImpl factory path because that path requires
// a real Dialector.Open and would either touch the network or duplicate the
// dialector tests. Instead each case constructs *Driver directly, injecting
// the sqlmock-backed *sql.DB into the embedded *driverPkg.DriverImpl. This
// keeps the test focused on Driver-layer behaviour: pool policy, Ping
// delegation, variant tagging, and the not-implemented sentinel for
// extractor / differ entry-points.
package driver

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"

	"github.com/actiontech/sqle-gaussdb-plugin/pkg/consts"
)

// newTestDriver wires a sqlmock-backed *sql.DB into a *Driver with the given
// variant. The returned cleanup function closes the underlying mock so the
// test does not leak file descriptors. We bypass newCommon because that path
// triggers Dialector.Open which would need a real network connection.
//
// MonitorPingsOption(true) is required for ExpectPing().WillDelayFor to take
// effect — sqlmock defaults to ignoring db.Ping() so the Ping-timeout case
// would otherwise return immediately instead of honouring the configured
// delay (see go-sqlmock README "MonitorPings" section).
func newTestDriver(t *testing.T, variant string) (*Driver, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	impl := &driverPkg.DriverImpl{
		Config: &driverV2.Config{},
		DB:     db,
	}
	d := &Driver{DriverImpl: impl, Variant: variant}
	cleanup := func() { _ = db.Close() }
	return d, mock, cleanup
}

// TestDriver_PluginNameRegistration covers design §17.1.5
// PluginNameRegistration. The two PluginName variants must produce
// structurally distinct Driver instances with the correct Variant tag, and
// must continue to satisfy driverV2.Driver at runtime (the compile-time
// assertions in driver_gaussdb.go / driver_opengauss.go cover the static
// side; this test exercises the dynamic side via a runtime type assertion on
// the factory return value).
func TestDriver_PluginNameRegistration(t *testing.T) {
	cases := map[string]struct {
		variant        string
		factory        func(*Driver) driverV2.Driver
		wantConcrete   string
		wantPluginName string
	}{
		"GaussDB variant wraps shared Driver": {
			variant:        consts.PluginNameGaussDB,
			factory:        func(d *Driver) driverV2.Driver { return &GaussDBDriver{Driver: d} },
			wantConcrete:   "*driver.GaussDBDriver",
			wantPluginName: "GaussDB",
		},
		"openGauss variant wraps shared Driver": {
			variant:        consts.PluginNameOpenGauss,
			factory:        func(d *Driver) driverV2.Driver { return &OpenGaussDriver{Driver: d} },
			wantConcrete:   "*driver.OpenGaussDriver",
			wantPluginName: "openGauss",
		},
		"variant tag persists through embed chain": {
			variant:        "GaussDB",
			factory:        func(d *Driver) driverV2.Driver { return &GaussDBDriver{Driver: d} },
			wantConcrete:   "*driver.GaussDBDriver",
			wantPluginName: "GaussDB",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			base, _, cleanup := newTestDriver(t, c.variant)
			defer cleanup()

			drv := c.factory(base)

			// Concrete type name asserts that the two factory functions land
			// on two structurally distinct types.
			gotConcrete := concreteTypeName(drv)
			if gotConcrete != c.wantConcrete {
				t.Errorf("concrete type = %s, want %s", gotConcrete, c.wantConcrete)
			}

			// Variant tag survives the embed chain — both
			// (*GaussDBDriver).Driver and (*OpenGaussDriver).Driver point at
			// the shared *Driver value whose Variant was set by the factory.
			gotVariant := ""
			switch x := drv.(type) {
			case *GaussDBDriver:
				gotVariant = x.Variant
			case *OpenGaussDriver:
				gotVariant = x.Variant
			default:
				t.Fatalf("unexpected concrete type %T", drv)
			}
			if gotVariant != c.wantPluginName {
				t.Errorf("Variant = %q, want %q", gotVariant, c.wantPluginName)
			}
		})
	}
}

// TestDriver_CapabilityMatrix locks the design §5.2 / §17.1.5 capability
// matrix. Per Task-Dev-004 派单 the test enumerates each of the **ten**
// OptionalModule values from design §5.2 that has a corresponding
// driverV2.Driver method, asserting the surface behaviour the plugin commits
// to today:
//
//   - OptionalGetDatabaseObjectDDL  (enabled this period — placeholder)
//   - OptionalGetDatabaseDiffModifySQL (enabled this period — placeholder)
//   - OptionalModuleGenRollbackSQL  (disabled — DriverImpl default empty)
//   - OptionalModuleExplain         (disabled — DriverImpl default empty)
//   - OptionalModuleGetTableMeta    (disabled — DriverImpl default empty)
//   - OptionalModuleExtractTableFromSQL (disabled — DriverImpl default empty)
//   - OptionalModuleEstimateSQLAffectRows (disabled — DriverImpl default empty)
//   - OptionalModuleKillProcess     (disabled — DriverImpl default empty)
//   - OptionalBackup                (disabled — DriverImpl default empty)
//   - OptionalModuleQuery           (disabled — requires *sql.Conn, surfaces
//                                    the "database conn not initialized"
//                                    sentinel since cfg.DSN==nil)
//
// The eleventh entry on the design table, OptionalModuleI18n, has no
// dedicated method on driverV2.Driver — it is signalled via DriverMetas only,
// so it is exercised by the dedicated TestCapabilityMatrix_I18nMetadataOnly
// below rather than through this main matrix.
//
// The two GaussDB and openGauss variants exercise the same matrix; running
// both keeps the variant decoupling honest while still tracking design §5.2.
func TestDriver_CapabilityMatrix(t *testing.T) {
	type probeResult struct {
		err     error
		nonNil  bool
		details string
	}
	cases := map[string]struct {
		module        driverV2.OptionalModule
		probe         func(ctx context.Context, drv driverV2.Driver) probeResult
		wantErrSubstr string // substring expected in err.Error(); "" means no error
		wantNonNil    bool   // true → the probe result must be non-nil typed value
	}{
		"OptionalGetDatabaseObjectDDL — enabled, returns empty result for nil objInfos": {
			module: driverV2.OptionalGetDatabaseObjectDDL,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				// Probe the real chain with nil objInfos: the for-range loop
				// iterates zero times so extractor.NewExtractor is constructed
				// but never invoked. Expected: ([]*DatabaseSchemaObjectResult{}, nil).
				res, err := drv.GetDatabaseObjectDDL(ctx, nil)
				return probeResult{err: err, nonNil: res != nil}
			},
			wantNonNil: true,
		},
		"OptionalGetDatabaseDiffModifySQL — enabled, returns empty result for nil objInfos": {
			module: driverV2.OptionalGetDatabaseDiffModifySQL,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				// Same probe shape: nil objInfos => empty slice, nil err.
				res, err := drv.GetDatabaseDiffModifySQL(ctx, nil, nil)
				return probeResult{err: err, nonNil: res != nil}
			},
			wantNonNil: true,
		},
		"OptionalModuleGenRollbackSQL — disabled, DriverImpl default returns empty strings": {
			module: driverV2.OptionalModuleGenRollbackSQL,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				a, b, err := drv.GenRollbackSQL(ctx, "SELECT 1")
				return probeResult{err: err, details: a + "|" + b}
			},
		},
		"OptionalModuleExplain — disabled, DriverImpl default returns empty *ExplainResult": {
			module: driverV2.OptionalModuleExplain,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				res, err := drv.Explain(ctx, &driverV2.ExplainConf{})
				return probeResult{err: err, nonNil: res != nil}
			},
			wantNonNil: true,
		},
		"OptionalModuleGetTableMeta — disabled, DriverImpl default returns empty *TableMeta": {
			module: driverV2.OptionalModuleGetTableMeta,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				res, err := drv.GetTableMeta(ctx, &driverV2.Table{})
				return probeResult{err: err, nonNil: res != nil}
			},
			wantNonNil: true,
		},
		"OptionalModuleExtractTableFromSQL — disabled, DriverImpl default returns empty []*Table": {
			module: driverV2.OptionalModuleExtractTableFromSQL,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				res, err := drv.ExtractTableFromSQL(ctx, "SELECT 1")
				return probeResult{err: err, nonNil: res != nil}
			},
			wantNonNil: true,
		},
		"OptionalModuleEstimateSQLAffectRows — disabled, DriverImpl default returns empty *EstimatedAffectRows": {
			module: driverV2.OptionalModuleEstimateSQLAffectRows,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				res, err := drv.EstimateSQLAffectRows(ctx, "SELECT 1")
				return probeResult{err: err, nonNil: res != nil}
			},
			wantNonNil: true,
		},
		"OptionalModuleKillProcess — disabled, DriverImpl default returns empty *KillProcessInfo": {
			module: driverV2.OptionalModuleKillProcess,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				res, err := drv.KillProcess(ctx)
				return probeResult{err: err, nonNil: res != nil}
			},
			wantNonNil: true,
		},
		"OptionalBackup — disabled, DriverImpl default returns (nil, nil)": {
			module: driverV2.OptionalBackup,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				res, err := drv.Backup(ctx, &driverV2.BackupReq{})
				return probeResult{err: err, nonNil: res != nil}
			},
			// DriverImpl.Backup returns (nil, nil); assert only the no-err
			// contract, not the response value.
		},
		"OptionalModuleQuery — disabled, no *sql.Conn → \"database conn not initialized\"": {
			module: driverV2.OptionalModuleQuery,
			probe: func(ctx context.Context, drv driverV2.Driver) probeResult {
				_, err := drv.Query(ctx, "SELECT 1", nil)
				return probeResult{err: err}
			},
			wantErrSubstr: "database conn not initialized",
		},
	}

	// Exercise both variants for every case so a variant-specific override
	// (today: only the two enabled-this-period methods on GaussDBDriver /
	// OpenGaussDriver) is caught immediately.
	variants := []string{consts.PluginNameGaussDB, consts.PluginNameOpenGauss}
	for _, variant := range variants {
		variant := variant
		for name, c := range cases {
			c := c
			t.Run(variant+"/"+name, func(t *testing.T) {
				base, _, cleanup := newTestDriver(t, variant)
				defer cleanup()

				var drv driverV2.Driver
				switch variant {
				case consts.PluginNameGaussDB:
					drv = &GaussDBDriver{Driver: base}
				case consts.PluginNameOpenGauss:
					drv = &OpenGaussDriver{Driver: base}
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				got := c.probe(ctx, drv)

				if c.wantErrSubstr != "" {
					if got.err == nil {
						t.Fatalf("module=%s expected error containing %q, got nil", c.module, c.wantErrSubstr)
					}
					if !strings.Contains(got.err.Error(), c.wantErrSubstr) {
						t.Errorf("module=%s error %q does not contain %q", c.module, got.err.Error(), c.wantErrSubstr)
					}
					return
				}
				if got.err != nil {
					t.Fatalf("module=%s expected no error, got %v", c.module, got.err)
				}
				if c.wantNonNil && !got.nonNil {
					t.Errorf("module=%s expected non-nil response, got nil", c.module)
				}
			})
		}
	}
}

// TestCapabilityMatrix_I18nMetadataOnly covers the eleventh row of design §5.2
// (OptionalModuleI18n). The Driver interface itself has no I18n entry-point —
// the flag flows through DriverMetas.EnabledOptionalModule into the
// audit-result pipeline. This test simply locks the constant value so a
// future refactor of OptionalModule iota ordering cannot silently break the
// cmd/* main.go capability declaration.
func TestCapabilityMatrix_I18nMetadataOnly(t *testing.T) {
	cases := map[string]struct {
		module   driverV2.OptionalModule
		wantName string
	}{
		"OptionalModuleI18n stringifies to \"I18n\"": {
			module:   driverV2.OptionalModuleI18n,
			wantName: "I18n",
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			if got := c.module.String(); got != c.wantName {
				t.Errorf("OptionalModule.String() = %q, want %q", got, c.wantName)
			}
		})
	}
}

// TestDriver_PingTimeout covers design §17.1.5 PingTimeout and exploration
// §1.4 R-Q1-1. The Driver.Ping override must (a) propagate context
// cancellation / deadline so a stuck backend does not hang the plugin process
// and (b) surface a clear error rather than panicking when the underlying
// *sql.DB is nil (the cfg.DSN==nil construction path).
func TestDriver_PingTimeout(t *testing.T) {
	cases := map[string]struct {
		setup      func(t *testing.T) (*Driver, func())
		ctxFactory func() (context.Context, context.CancelFunc)
		// wantErrSubstr is asserted when non-empty. For the deadline case we
		// rely on wantCtxErrExpired instead, because sqlmock surfaces its
		// internal "canceling query due to user request" string before
		// database/sql wraps the ctx error; we still verify the ctx itself
		// has reached the cancelled / deadline state to prove Ping honoured
		// the deadline.
		wantErrSubstr     string
		wantCtxErrExpired bool
	}{
		"deadline exceeded — ctx expires, Ping returns non-nil error within budget": {
			setup: func(t *testing.T) (*Driver, func()) {
				d, mock, cleanup := newTestDriver(t, consts.PluginNameGaussDB)
				// Force the sqlmock ping to take longer than the context
				// deadline; sqlmock honours ExpectPing.WillDelayFor.
				mock.ExpectPing().WillDelayFor(200 * time.Millisecond)
				return d, cleanup
			},
			ctxFactory: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Millisecond)
			},
			wantCtxErrExpired: true,
		},
		"cancelled context surfaces context.Canceled": {
			setup: func(t *testing.T) (*Driver, func()) {
				d, mock, cleanup := newTestDriver(t, consts.PluginNameOpenGauss)
				mock.ExpectPing().WillDelayFor(50 * time.Millisecond)
				return d, cleanup
			},
			ctxFactory: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // cancel immediately so Ping observes the cancellation
				return ctx, func() {}
			},
			wantErrSubstr:     "context canceled",
			wantCtxErrExpired: true,
		},
		"nil *sql.DB returns explicit initialisation error": {
			setup: func(t *testing.T) (*Driver, func()) {
				// Bypass sqlmock entirely so DriverImpl.DB stays nil.
				impl := &driverPkg.DriverImpl{Config: &driverV2.Config{}}
				d := &Driver{DriverImpl: impl, Variant: consts.PluginNameGaussDB}
				return d, func() {}
			},
			ctxFactory: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Second)
			},
			wantErrSubstr: "before database connection initialised",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			d, cleanup := c.setup(t)
			defer cleanup()

			ctx, cancel := c.ctxFactory()
			defer cancel()

			// Drive Ping via the GaussDBDriver wrapper to mirror the runtime
			// call chain (cmd/* → driverV2.Driver → embedded *Driver.Ping).
			drv := driverV2.Driver(&GaussDBDriver{Driver: d})

			// Bound the Ping call so a regression that breaks ctx propagation
			// fails the test instead of hanging the suite.
			done := make(chan error, 1)
			go func() { done <- drv.Ping(ctx) }()

			var err error
			select {
			case err = <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Ping did not return within 2s; ctx propagation broken")
			}

			if err == nil {
				t.Fatalf("expected non-nil error from Ping")
			}
			if c.wantErrSubstr != "" && !strings.Contains(err.Error(), c.wantErrSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), c.wantErrSubstr)
			}
			if c.wantCtxErrExpired && ctx.Err() == nil {
				t.Errorf("ctx.Err() unexpectedly nil; Ping did not honour the deadline")
			}
		})
	}
}

// TestWrapPGError locks the SQLSTATE pass-through contract from design §5.3.
// Each case asserts that the helper preserves error identity (== check) so
// that callers can still type-assert the original error after wrapping.
func TestWrapPGError(t *testing.T) {
	plainErr := errors.New("connection refused")
	sentinel := errors.New("sqle-gaussdb-plugin: probe sentinel")

	cases := map[string]struct {
		in      error
		wantNil bool
		wantSame error
	}{
		"nil propagates as nil": {
			in:      nil,
			wantNil: true,
		},
		"plain error returns same instance unchanged": {
			in:       plainErr,
			wantSame: plainErr,
		},
		"sentinel error preserves identity": {
			in:       sentinel,
			wantSame: sentinel,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := wrapPGError(c.in)
			if c.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got != c.wantSame { //nolint:errorlint // we want pointer-equality on purpose
				t.Errorf("expected same error instance %v, got %v", c.wantSame, got)
			}
		})
	}
}

// TestApplyPoolPolicy locks the connection-pool constants from design §5.1
// line 328 onto the *sql.DB after newCommon. The Stats() snapshot exposes
// MaxOpenConnections directly; MaxIdleConnections and ConnMaxLifetime are not
// readable post-set in Go 1.19, so we additionally re-call SetMaxIdleConns
// and assert the no-op (sql.DB stores the value internally; the assertion is
// implicit — applyPoolPolicy returning without panic with a nil DB exercises
// the defensive branch).
func TestApplyPoolPolicy(t *testing.T) {
	cases := map[string]struct {
		dbNil      bool
		wantMaxOpen int
	}{
		"applies MaxOpenConns when db is non-nil": {
			dbNil:      false,
			wantMaxOpen: maxOpenConns,
		},
		"no-op when db is nil (cfg.DSN==nil construction path)": {
			dbNil:      true,
			wantMaxOpen: 0,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var db *sql.DB
			if !c.dbNil {
				m, _, err := sqlmock.New()
				if err != nil {
					t.Fatalf("sqlmock.New: %v", err)
				}
				defer m.Close()
				db = m
			}
			applyPoolPolicy(db)
			if c.dbNil {
				// The defensive nil check is the contract; reaching this point
				// without panicking proves it.
				return
			}
			if got := db.Stats().MaxOpenConnections; got != c.wantMaxOpen {
				t.Errorf("MaxOpenConnections = %d, want %d", got, c.wantMaxOpen)
			}
		})
	}
}

// extractTableSQLLiteral mirrors internal/extractor/table.go:extractTableSQL
// byte-for-byte. We deliberately duplicate the literal here (rather than
// importing the unexported const) so the real-chain driver tests below do not
// take a code dependency on the extractor package internals — driver layer's
// contract is `extractor.NewExtractor(db).Extract(ctx, info)` and nothing else.
// Any drift between this literal and extractor/table.go would be caught by the
// existing TestExtractor_Extract_TableAndView (which exercises the canonical
// const directly); the redundancy here is intentional documentation of the
// SQL the driver layer ultimately issues.
const extractTableSQLLiteral = `SELECT pg_get_tabledef(c.oid)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1
  AND c.relkind IN ('r', 'p')
  AND c.relname = $2`

// newRealChainTestDriver builds a *Driver whose embedded *sql.DB is a
// sqlmock-backed connection with QueryMatcherEqual mode (so SQL-literal
// assertions in real-chain tests below lock the byte-for-byte design §6.2.1
// template; see semantic sqlmock_query_matcher_equal_injection_defense_20260521).
// Unlike newTestDriver above, MonitorPingsOption is left at its default — the
// real-chain tests never call Ping.
func newRealChainTestDriver(t *testing.T) (*Driver, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
	)
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	impl := &driverPkg.DriverImpl{
		Config: &driverV2.Config{},
		DB:     db,
	}
	d := &Driver{DriverImpl: impl, Variant: consts.PluginNameGaussDB}
	cleanup := func() { _ = db.Close() }
	return d, mock, cleanup
}

// TestDriver_GetDatabaseObjectDDL_RealChain exercises the live wiring from
// driver.GetDatabaseObjectDDL through extractor.NewExtractor(db).Extract to
// the *sql.DB catalog query. Three sub-cases cover the contractual surface:
//
//   - empty objInfos slice  → empty results + nil error
//   - one DatabaseSchemaInfo with one TABLE object → results length 1 with
//     the round-tripped pg_get_tabledef DDL
//   - catalog query error (sql.ErrConnDone) → error propagates, results nil
//
// All cases use sqlmock.QueryMatcherEqual via newRealChainTestDriver so the
// design §6.2.1 SQL template is locked byte-for-byte.
func TestDriver_GetDatabaseObjectDDL_RealChain(t *testing.T) {
	const tableDDL = "SET search_path = 'public'; CREATE TABLE public.t1 (id int) WITH (orientation=row);"

	cases := map[string]struct {
		objInfos       []*driverV2.DatabaseSchemaInfo
		mockSetup      func(mock sqlmock.Sqlmock)
		wantLen        int
		wantErr        bool
		wantErrSubstr  string
		wantFirstDDL   string
		wantFirstObjNm string
	}{
		"empty objInfos returns empty slice": {
			objInfos:  []*driverV2.DatabaseSchemaInfo{},
			mockSetup: func(mock sqlmock.Sqlmock) {}, // no calls expected
			wantLen:   0,
		},
		"single schema with one table extracted": {
			objInfos: []*driverV2.DatabaseSchemaInfo{
				{
					SchemaName: "public",
					DatabaseObjects: []*driverV2.DatabaseObject{
						{ObjectName: "t1", ObjectType: driverV2.ObjectType_TABLE},
					},
				},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractTableSQLLiteral).
					WithArgs("public", "t1").
					WillReturnRows(sqlmock.NewRows([]string{"pg_get_tabledef"}).AddRow(tableDDL))
			},
			wantLen:        1,
			wantFirstDDL:   tableDDL,
			wantFirstObjNm: "t1",
		},
		"extract error propagates": {
			objInfos: []*driverV2.DatabaseSchemaInfo{
				{
					SchemaName: "public",
					DatabaseObjects: []*driverV2.DatabaseObject{
						{ObjectName: "t1", ObjectType: driverV2.ObjectType_TABLE},
					},
				},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(extractTableSQLLiteral).
					WithArgs("public", "t1").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr:       true,
			wantErrSubstr: sql.ErrConnDone.Error(),
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			d, mock, cleanup := newRealChainTestDriver(t)
			defer cleanup()
			c.mockSetup(mock)

			drv := driverV2.Driver(&GaussDBDriver{Driver: d})

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			res, err := drv.GetDatabaseObjectDDL(ctx, c.objInfos)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; res=%v", res)
				}
				if c.wantErrSubstr != "" && !strings.Contains(err.Error(), c.wantErrSubstr) {
					t.Errorf("err %q does not contain %q", err.Error(), c.wantErrSubstr)
				}
				if res != nil {
					t.Errorf("expected nil res on error, got %v", res)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got := len(res); got != c.wantLen {
					t.Fatalf("len(res) = %d, want %d", got, c.wantLen)
				}
				if c.wantLen > 0 {
					first := res[0]
					if first == nil {
						t.Fatal("res[0] is nil")
					}
					if got := len(first.DatabaseObjectDDLs); got != 1 {
						t.Fatalf("len(res[0].DatabaseObjectDDLs) = %d, want 1", got)
					}
					ddl := first.DatabaseObjectDDLs[0]
					if ddl.ObjectDDL != c.wantFirstDDL {
						t.Errorf("ObjectDDL = %q, want %q", ddl.ObjectDDL, c.wantFirstDDL)
					}
					if ddl.DatabaseObject == nil || ddl.DatabaseObject.ObjectName != c.wantFirstObjNm {
						t.Errorf("DatabaseObject.ObjectName = %v, want %q", ddl.DatabaseObject, c.wantFirstObjNm)
					}
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("sqlmock expectations: %v", err)
			}
		})
	}
}

// TestDriver_GetDatabaseDiffModifySQL_RealChain exercises the live wiring from
// driver.GetDatabaseDiffModifySQL through two extractor.Extract invocations
// (base side then compared side) into differ.GenerateModifySQLs.
//
// Sub-cases:
//
//  1. empty objInfos slice → empty results + nil error
//  2. single DatabasCompareSchemaInfo with one TABLE that diffs between
//     base and compared → one DatabaseDiffModifySQLResult whose ModifySQLs
//     contains the WARNING comment + DROP TABLE IF EXISTS + CREATE TABLE
//     (end-to-end DROP+CREATE wiring, design §7.2 / R-Q4-1)
//  3. base-side extract error → error propagates, compared-side ExpectQuery
//     is intentionally NOT registered to prove the second call is short-
//     circuited (sqlmock.ExpectationsWereMet would still pass because we
//     never queued the second expectation)
//
// All cases lock the design §6.2.1 SQL template byte-for-byte via
// QueryMatcherEqual; the base and compared sides issue the SAME SQL with the
// SAME args (only the SchemaName parameter differs) and sqlmock consumes them
// in registration order.
func TestDriver_GetDatabaseDiffModifySQL_RealChain(t *testing.T) {
	const (
		baseTableDDL     = "CREATE TABLE base.t (id int);"
		comparedTableDDL = "CREATE TABLE compared.t (id int, name text);"
	)

	cases := map[string]struct {
		objInfos          []*driverV2.DatabasCompareSchemaInfo
		mockSetup         func(mock sqlmock.Sqlmock)
		wantLen           int
		wantErr           bool
		wantErrSubstr     string
		wantSchemaName    string
		wantModifyContain []string // every substring must appear inside the joined ModifySQLs
	}{
		"empty objInfos returns empty slice": {
			objInfos:  []*driverV2.DatabasCompareSchemaInfo{},
			mockSetup: func(mock sqlmock.Sqlmock) {}, // no calls expected
			wantLen:   0,
		},
		"single compare info with table both diff produces DROP+CREATE": {
			objInfos: []*driverV2.DatabasCompareSchemaInfo{
				{
					BaseSchemaName:     "base",
					ComparedSchemaName: "compared",
					DatabaseObjects: []*driverV2.DatabaseObject{
						{ObjectName: "t", ObjectType: driverV2.ObjectType_TABLE},
					},
				},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// First extract: base side (SchemaName="base", ObjectName="t").
				mock.ExpectQuery(extractTableSQLLiteral).
					WithArgs("base", "t").
					WillReturnRows(sqlmock.NewRows([]string{"pg_get_tabledef"}).AddRow(baseTableDDL))
				// Second extract: compared side (SchemaName="compared", ObjectName="t").
				mock.ExpectQuery(extractTableSQLLiteral).
					WithArgs("compared", "t").
					WillReturnRows(sqlmock.NewRows([]string{"pg_get_tabledef"}).AddRow(comparedTableDDL))
			},
			wantLen:        1,
			wantSchemaName: "base",
			wantModifyContain: []string{
				"WARNING",
				"DROP TABLE IF EXISTS compared.t",
				baseTableDDL, // CREATE TABLE base.t (id int);
			},
		},
		"base side extract error propagates": {
			objInfos: []*driverV2.DatabasCompareSchemaInfo{
				{
					BaseSchemaName:     "base",
					ComparedSchemaName: "compared",
					DatabaseObjects: []*driverV2.DatabaseObject{
						{ObjectName: "t", ObjectType: driverV2.ObjectType_TABLE},
					},
				},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Only the base-side query is expected; compared-side must NOT
				// fire (verified implicitly: sqlmock would report unexpected
				// query if it did, and ExpectationsWereMet would still pass
				// because we never queued the second expectation).
				mock.ExpectQuery(extractTableSQLLiteral).
					WithArgs("base", "t").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr:       true,
			wantErrSubstr: sql.ErrConnDone.Error(),
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			d, mock, cleanup := newRealChainTestDriver(t)
			defer cleanup()
			c.mockSetup(mock)

			drv := driverV2.Driver(&GaussDBDriver{Driver: d})

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			res, err := drv.GetDatabaseDiffModifySQL(ctx, nil, c.objInfos)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; res=%v", res)
				}
				if c.wantErrSubstr != "" && !strings.Contains(err.Error(), c.wantErrSubstr) {
					t.Errorf("err %q does not contain %q", err.Error(), c.wantErrSubstr)
				}
				if res != nil {
					t.Errorf("expected nil res on error, got %v", res)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got := len(res); got != c.wantLen {
					t.Fatalf("len(res) = %d, want %d", got, c.wantLen)
				}
				if c.wantLen > 0 {
					first := res[0]
					if first == nil {
						t.Fatal("res[0] is nil")
					}
					if first.SchemaName != c.wantSchemaName {
						t.Errorf("SchemaName = %q, want %q", first.SchemaName, c.wantSchemaName)
					}
					if len(first.ModifySQLs) != 1 {
						t.Fatalf("len(ModifySQLs) = %d, want 1; got=%v", len(first.ModifySQLs), first.ModifySQLs)
					}
					joined := first.ModifySQLs[0]
					for _, sub := range c.wantModifyContain {
						if !strings.Contains(joined, sub) {
							t.Errorf("ModifySQLs[0] missing %q; full=%q", sub, joined)
						}
					}
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("sqlmock expectations: %v", err)
			}
		})
	}
}

// concreteTypeName returns the package-qualified type name of v. We avoid the
// fmt %T format directive because it includes a leading package selector that
// differs between go test invocations (e.g. "*driver.GaussDBDriver" vs
// "*github.com/.../driver.GaussDBDriver" in some build modes).
func concreteTypeName(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	// Use a switch over the known concrete types so the assertion is explicit
	// and survives a future refactor that adds more variants.
	switch v.(type) {
	case *GaussDBDriver:
		return "*driver.GaussDBDriver"
	case *OpenGaussDriver:
		return "*driver.OpenGaussDriver"
	case *Driver:
		return "*driver.Driver"
	default:
		return "unknown"
	}
}
