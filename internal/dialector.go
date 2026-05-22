package internal

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"

	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	"github.com/pkg/errors"

	// driverNameGaussDB ("opengauss") must be registered before sql.Open is
	// called below. The jackc/pgx driver cannot complete the SHA256 SASL
	// handshake against GaussDB Kernel 505.2.1 private protocol; only
	// gitee.com/opengauss/openGauss-connector-go-pq implements it (already
	// proven by the dms-ee provision path).
	_ "gitee.com/opengauss/openGauss-connector-go-pq"
)

// driverNameGaussDB is the database/sql driver name registered by
// gitee.com/opengauss/openGauss-connector-go-pq. The plugin uses this driver
// (rather than jackc/pgx) because GaussDB Kernel 505.2.1.SPC0800 uses a
// private SHA256 SASL handshake that pgx v4 does not implement.
const driverNameGaussDB = "opengauss"

// connInitTimeout bounds the time spent on db.Conn and conn.PingContext during
// connectivity test. lib/pq + pgx may block indefinitely on the
// startup-packet / SSL handshake when the remote endpoint stops responding
// mid-handshake (observed against GaussDB CN/cm_agent on TCP port 8000).
// Without a deadline the plugin Init RPC never returns and the DMS UI hangs on
// "loading". 5s gives slow but reachable hosts enough headroom while still
// failing fast on unreachable ones.
const connInitTimeout = 5 * time.Second

// Dialector embeds PostgresDialector for SQL parsing and DSN shape, but
// overrides Open so that connectivity test goes through our timeout-aware
// GetConn instead of BaseDialector.GetConn (which uses context.TODO()).
type Dialector struct {
	driverPkg.PostgresDialector
}

func (d *Dialector) DatabaseAdditionalParam() params.Params {
	return params.Params{
		&params.Param{
			Key:   "default_schema",
			Value: "public",
			Desc:  "指定默认的 schema（默认为 public）",
			Type:  params.ParamTypeString,
		},
	}
}

func (d *Dialector) ShowDatabaseSQL() string {
	return "SELECT datname FROM pg_database WHERE datname NOT IN ('template1', 'template0');"
}

// Open implements driverPkg.Dialector.Open. It mirrors PostgresDialector.Open
// (DSN composition) but routes through d.GetConn using driverNameGaussDB so
// that we (a) speak the GaussDB private SHA256 handshake via
// openGauss-connector-go-pq, and (b) get the 5s timeout on db.Conn /
// conn.PingContext rather than context.TODO().
func (d *Dialector) Open(dsn *driverV2.DSN) (*sql.DB, *sql.Conn, error) {
	if dsn.DatabaseName == "" {
		dsn.DatabaseName = "postgres"
	}
	return d.GetConn(driverNameGaussDB, fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dsn.User, dsn.Password, dsn.Host, dsn.Port, dsn.DatabaseName))
}

// GetConn opens a database and verifies connectivity with a hard 5s deadline
// on both db.Conn and conn.PingContext. The deadline is enforced via a
// goroutine + select wrapper because, in practice, db.Conn(ctx) does NOT
// always honor its ctx when the remote endpoint completes a TCP handshake but
// then stalls during the PostgreSQL startup-message exchange (observed against
// GaussDB CN/cm_agent on TCP port 8000): the underlying pgx ConnectConfig
// installs a contextWatcher that arms net.Conn.SetDeadline only after the TLS
// negotiation has begun, leaving the very first read of the startup response
// effectively unbounded. Wrapping in a select with our own ctx.Done() gives a
// strict upper bound regardless of how the driver internally handles ctx.
func (d *Dialector) GetConn(driverName, dataSourceName string) (*sql.DB, *sql.Conn, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, nil, err
	}

	connCtx, connCancel := context.WithTimeout(context.Background(), connInitTimeout)
	defer connCancel()
	type connRes struct {
		c   *sql.Conn
		err error
	}
	ch := make(chan connRes, 1)
	go func() {
		c, err := db.Conn(connCtx)
		ch <- connRes{c: c, err: err}
	}()
	var conn *sql.Conn
	select {
	case r := <-ch:
		conn = r.c
		err = r.err
	case <-connCtx.Done():
		err = fmt.Errorf("db.Conn timed out after %s: %w", connInitTimeout, connCtx.Err())
		// drain the goroutine in the background and release any connection
		// that eventually arrives so the underlying socket is not leaked.
		go func() {
			if r := <-ch; r.c != nil {
				_ = r.c.Close()
			}
		}()
	}
	if err != nil {
		db.Close()
		return nil, nil, errors.Wrap(err, "get database connection failed when new driver")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), connInitTimeout)
	defer pingCancel()
	pingCh := make(chan error, 1)
	go func() { pingCh <- conn.PingContext(pingCtx) }()
	var pingErr error
	select {
	case pingErr = <-pingCh:
	case <-pingCtx.Done():
		pingErr = fmt.Errorf("PingContext timed out after %s: %w", connInitTimeout, pingCtx.Err())
	}
	if pingErr != nil {
		conn.Close()
		db.Close()
		return nil, nil, errors.Wrap(pingErr, "ping database connection failed when new driver")
	}
	return db, conn, nil
}
