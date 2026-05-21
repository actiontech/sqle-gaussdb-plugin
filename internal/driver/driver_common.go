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
	"time"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	hclog "github.com/hashicorp/go-hclog"
)

// Connection-pool policy shared by the GaussDB and openGauss variants. The
// values match sqle-pg-plugin's runtime profile so the new plugin does not
// introduce a different pool-behaviour footprint (compat-RISK-2; design §5.1
// line 328).
const (
	maxOpenConns    = 10
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
)

// errNotImplemented is returned by GetDatabaseObjectDDL /
// GetDatabaseDiffModifySQL while the extractor (Task-Dev-005) and differ
// (Task-Dev-006) implementations are pending. Keeping the placeholder text in
// one constant lets us assert on it from unit tests without literal duplication.
var errNotImplemented = errors.New(
	"not implemented yet: Task-Dev-005 (extractor) / Task-Dev-006 (differ)")

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
