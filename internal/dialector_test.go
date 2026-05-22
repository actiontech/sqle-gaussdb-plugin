package internal

import (
	"database/sql"
	"testing"
	"time"
)

// TestGetConn_UsesOpenGaussDriver ensures the driver name constant used by
// dialector.go for sql.Open is "opengauss" (provided by
// gitee.com/opengauss/openGauss-connector-go-pq), NOT "pgx". jackc/pgx v4
// cannot complete the SHA256 SASL handshake against GaussDB Kernel 505.2.1
// private protocol, so any regression of this constant back to "pgx" would
// reintroduce the P-2 5s timeout on every plugin Init call.
//
// We assert two things:
//  1. The constant value matches the registered driver name.
//  2. The driver is actually registered with database/sql; we sql.Open with
//     this driver name and check that the open call itself does not error
//     because the driver is unknown ("sql: unknown driver \"opengauss\""
//     would be the error if registration was broken). We do NOT actually
//     connect to a real database here.
func TestGetConn_UsesOpenGaussDriver(t *testing.T) {
	if driverNameGaussDB != "opengauss" {
		t.Fatalf("driverNameGaussDB regression: want %q, got %q", "opengauss", driverNameGaussDB)
	}

	// Confirm the driver is registered with database/sql; sql.Open does
	// not establish a connection, so failures here mean the anonymous
	// import of openGauss-connector-go-pq is missing or the driver name
	// does not match what the package registers.
	db, err := sql.Open(driverNameGaussDB, "host=127.0.0.1 port=1 user=u password=p dbname=x sslmode=disable")
	if err != nil {
		t.Fatalf("sql.Open(%q) failed: %v (driver likely not registered; check anonymous import in dialector.go)", driverNameGaussDB, err)
	}
	defer db.Close()

	// sql.Open is lazy and does not dial; assert the *sql.DB instance is
	// non-nil so the test is not a no-op.
	if db == nil {
		t.Fatalf("sql.Open(%q) returned nil *sql.DB", driverNameGaussDB)
	}

	// Make sure no other driver named "pgx" sneaks in as the chosen name
	// for this plugin. The anonymous pgx imports kept in
	// internal/inspector and internal/executor are intentional (they
	// register pgx as a side-effect of historical PG plugin code) but
	// the plugin must NOT use it for actual sql.Open calls.
	for _, name := range sql.Drivers() {
		if name == driverNameGaussDB {
			return // OK
		}
	}
	t.Fatalf("driver %q not in sql.Drivers(): %v", driverNameGaussDB, sql.Drivers())
}

// TestGetConn_KeepsFiveSecondTimeout guards against regression of the 5s
// connInitTimeout introduced in fix-2912 (commit e2744ce). Even though the
// driver has been switched from jackc/pgx to openGauss-connector-go-pq, the
// goroutine + select wrapper around db.Conn / PingContext must stay: the
// underlying lib/pq path can still block in the startup-message phase on
// unreachable hosts, and the plugin Init RPC must never hang the DMS UI.
func TestGetConn_KeepsFiveSecondTimeout(t *testing.T) {
	if connInitTimeout != 5*time.Second {
		t.Fatalf("connInitTimeout regression: want 5s, got %v (P-1 fix from #2912 must not be reverted)", connInitTimeout)
	}

	// Sanity-bound: the timeout must be short enough to fail fast (<= 10s,
	// otherwise the DMS UI would feel stuck) and long enough to give slow
	// but reachable hosts headroom (>= 1s, otherwise we would false-alarm
	// on healthy connections).
	if connInitTimeout > 10*time.Second {
		t.Fatalf("connInitTimeout too large (%v); UI would feel unresponsive", connInitTimeout)
	}
	if connInitTimeout < 1*time.Second {
		t.Fatalf("connInitTimeout too small (%v); risks false timeouts on slow but reachable hosts", connInitTimeout)
	}
}
