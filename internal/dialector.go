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
)

// connInitTimeout bounds the time spent on db.Conn and conn.PingContext during
// connectivity test. lib/pq + pgx may block indefinitely on the
// startup-packet / SSL handshake when the remote endpoint stops responding
// mid-handshake. Without a deadline the plugin Init RPC never returns and the
// DMS UI hangs on "loading". 5s gives slow but reachable hosts enough headroom
// while still failing fast on unreachable ones.
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
// (DSN composition + pgx driver) but routes through d.GetConn so that we get
// the 5s timeout on db.Conn / conn.PingContext rather than context.TODO().
func (d *Dialector) Open(dsn *driverV2.DSN) (*sql.DB, *sql.Conn, error) {
	if dsn.DatabaseName == "" {
		dsn.DatabaseName = "postgres"
	}
	return d.GetConn("pgx", fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dsn.User, dsn.Password, dsn.Host, dsn.Port, dsn.DatabaseName))
}

func (d *Dialector) GetConn(driverName, dataSourceName string) (*sql.DB, *sql.Conn, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, nil, err
	}

	connCtx, connCancel := context.WithTimeout(context.Background(), connInitTimeout)
	defer connCancel()
	conn, err := db.Conn(connCtx)
	if err != nil {
		db.Close()
		return nil, nil, errors.Wrap(err, "get database connection failed when new driver")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), connInitTimeout)
	defer pingCancel()
	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		db.Close()
		return nil, nil, errors.Wrap(err, "ping database connection failed when new driver")
	}
	return db, conn, nil
}
