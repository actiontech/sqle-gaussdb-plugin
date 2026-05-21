// Package dialector — minimal placeholder for Task-Dev-002.
//
// The real Dialector for GaussDB / openGauss must:
//   - return DriverName "opengauss" (registered by gitee.com/opengauss/openGauss-connector-go-pq)
//   - build libpq key=value DSN with branch-specific defaults (port 8000 / 5432, sslmode, etc.)
//   - expose connect-timeout fallback and additional-param hooks
//
// All of the above belongs to Task-Dev-003 (plan §3.1). This file only ensures
// driverPkg.NewDriverBuilder(&Dialector{...}) compiles — every method is the
// smallest zero-value form that satisfies the driverPkg.Dialector interface.
package dialector

import (
	"context"
	"database/sql"
	"errors"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	"github.com/actiontech/sqle/sqle/pkg/params"
)

// Dialector identifies which variant (GaussDB or openGauss) the current plugin
// process represents. Variant must be one of consts.PluginNameGaussDB /
// consts.PluginNameOpenGauss (set by the cmd/* main.go entrypoint).
type Dialector struct {
	driverPkg.BaseDialector
	Variant string
}

// Compile-time assertion: *Dialector satisfies driverPkg.Dialector.
var _ driverPkg.Dialector = (*Dialector)(nil)

// String returns the dialect name. driverPkg.NewDriverBuilder copies this into
// DriverMetas.PluginName at construction time; cmd/* main.go overrides
// builder.Meta.PluginName afterwards to the exact "GaussDB" / "openGauss"
// literal (design §4.4 strict-match contract).
func (d *Dialector) String() string {
	return d.Variant
}

// DatabaseAdditionalParam returns plugin-specific extra connection parameters.
// Task-Dev-002 returns an empty slice; real SSL / authtype params land in
// Task-Dev-003 per design §4.3.
func (d *Dialector) DatabaseAdditionalParam() params.Params {
	return params.Params{}
}

// Open is the minimum placeholder that satisfies the driverPkg.Dialector
// interface. It deliberately returns an error so that any code path that
// reaches it during the skeleton stage fails fast with a clear message. The
// real implementation (DSN construction + libpq driver name="opengauss") lands
// in Task-Dev-003.
func (d *Dialector) Open(dsn *driverV2.DSN) (*sql.DB, *sql.Conn, error) {
	_ = context.TODO
	return nil, nil, errors.New("sqle-gaussdb-plugin: Dialector.Open is not implemented yet (Task-Dev-003)")
}

// ShowDatabaseSQL returns the SQL used by sqle to enumerate databases on the
// data source. Placeholder for Task-Dev-002.
func (d *Dialector) ShowDatabaseSQL() string {
	return "SELECT datname FROM pg_database WHERE datname NOT IN ('template0', 'template1')"
}
