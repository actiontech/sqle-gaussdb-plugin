package internal

import (
	"context"
	"database/sql"

	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	"github.com/pkg/errors"
)

type Dialector struct {
	driverPkg.PostgresDialector
}

func (d *Dialector) String() string {
	return "TBase"
}

func (d *Dialector) ShowDatabaseSQL() string {
	return "SELECT datname FROM pg_database WHERE datname NOT IN ('template1', 'template0');"
}

func (d *Dialector) GetConn(driverName, dataSourceName string) (*sql.DB, *sql.Conn, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, nil, err
	}
	conn, err := db.Conn(context.TODO())
	if err != nil {
		db.Close()
		return nil, nil, errors.Wrap(err, "get database connection failed when new driver")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := conn.PingContext(context.TODO()); err != nil {
		conn.Close()
		db.Close()
		return nil, nil, errors.Wrap(err, "ping database connection failed when new driver")
	}
	return db, conn, nil
}
