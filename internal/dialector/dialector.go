// Package dialector implements the Dialector for GaussDB / openGauss data
// sources used by sqle-gaussdb-plugin.
//
// Two responsibilities:
//
//  1. Satisfy the driverPkg.Dialector interface (String / DatabaseAdditionalParam /
//     Open / ShowDatabaseSQL) so that sqle's pkg/driver.DriverBuilder can use this
//     dialect end-to-end (see sqle/sqle/pkg/driver/dialector.go).
//  2. Expose three plugin-internal helpers — DriverName / DefaultPort / BuildDSN —
//     that encapsulate the GaussDB vs openGauss branch logic (default port,
//     default sslmode, libpq key=value DSN layout). These helpers are pulled out
//     of Open so that unit tests can assert the DSN purely as a string, without
//     touching a real database (design §4.3 + §17.1.1).
//
// design §4.3 伪代码 uses `*driverV2.Config` as BuildDSN's parameter but accesses
// fields such as `cfg.Host` that live on `*driverV2.DSN` (the inner struct of
// Config). Real sqle code paths always pass `*driverV2.DSN` to Dialector.Open,
// so we keep BuildDSN's signature as `*driverV2.DSN`. This is semantically
// equivalent to the design pseudocode and avoids a redundant `cfg.DSN.Host`
// access in callers.
package dialector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"

	"github.com/actiontech/sqle-gaussdb-plugin/pkg/consts"

	// Register the database/sql driver name "opengauss" via the openGauss
	// official Go driver (gitee.com/opengauss/openGauss-connector-go-pq).
	// design §3.4 / plan §3.1 mandate this registration.
	_ "gitee.com/opengauss/openGauss-connector-go-pq"
)

// driverNameOpenGauss is the database/sql driver name registered by the blank
// import above. Both GaussDB (port 8000) and openGauss (port 5432) connect
// through the same libpq-compatible driver — only the DSN differs.
const driverNameOpenGauss = "opengauss"

// defaultDatabaseName is the libpq fallback database when DSN.DatabaseName is
// empty. design §4.3 伪代码 fixes this to "postgres" for both variants.
const defaultDatabaseName = "postgres"

// connectTimeoutSeconds is the libpq connect_timeout DSN parameter value used
// as a sane fallback. exploration_dev.md §1.4 R-Q1-1 mandated this so that
// hung TCP handshakes fail fast within plugin Ping budget.
const connectTimeoutSeconds = 5

// Dialector identifies which variant (GaussDB or openGauss) the current
// plugin process represents. Variant must be one of consts.PluginNameGaussDB
// or consts.PluginNameOpenGauss (set by the cmd/* main.go entrypoint).
//
// Embedding driverPkg.BaseDialector inherits the default ShowDatabaseSQL /
// DatabaseAdditionalParam / GetConn implementations from sqle; we override
// the ones that need GaussDB / openGauss specific behaviour below.
type Dialector struct {
	driverPkg.BaseDialector
	Variant string
}

// Compile-time assertion: *Dialector satisfies driverPkg.Dialector.
var _ driverPkg.Dialector = (*Dialector)(nil)

// String returns the dialect name. driverPkg.NewDriverBuilder copies this into
// DriverMetas.PluginName at construction time; cmd/* main.go then overrides
// builder.Meta.PluginName to the strict "GaussDB" / "openGauss" literal
// required by design §4.4 (PluginName strict-match contract).
func (d *Dialector) String() string {
	return d.Variant
}

// DatabaseAdditionalParam returns plugin-level extra connection parameters
// surfaced to the DMS data-source UI. Currently empty: SSL / authtype are
// supplied via DSN.AdditionalParams at runtime (design §4.3 表格 + §4.3 DSN
// 拼装规则). Future per-variant additions land here.
func (d *Dialector) DatabaseAdditionalParam() params.Params {
	return params.Params{}
}

// ShowDatabaseSQL returns the SQL used by sqle to enumerate databases on the
// data source. openGauss / GaussDB share the same pg_database catalog as
// upstream PostgreSQL, so the statement matches sqle-pg-plugin's variant.
func (d *Dialector) ShowDatabaseSQL() string {
	return "SELECT datname FROM pg_database WHERE datname NOT IN ('template0', 'template1');"
}

// DriverName returns the database/sql driver name registered by the openGauss
// official Go driver (see the blank import at the top of this file). Both
// variants share the same driver — the variant only changes DSN content.
func (d *Dialector) DriverName() string {
	return driverNameOpenGauss
}

// DefaultPort returns the default TCP port for the current variant. design
// §4.3 表格:
//
//	GaussDB   → 8000  (commercial GaussDB default)
//	openGauss → 5432  (openGauss 6.0 default)
//
// Any unrecognised Variant returns 0. The empty Variant case is exercised by
// TestDialector_DefaultPort to lock the fallback contract.
func (d *Dialector) DefaultPort() int {
	switch d.Variant {
	case consts.PluginNameGaussDB:
		return consts.DefaultPortGaussDB
	case consts.PluginNameOpenGauss:
		return consts.DefaultPortOpenGauss
	default:
		return 0
	}
}

// BuildDSN constructs a libpq key=value DSN string for the openGauss driver.
// design §4.3 关键片段 specifies the layout:
//
//   - 5 baseline fields: host / port / user / password / dbname
//   - connect_timeout fallback (5 s)
//   - GaussDB defaults sslmode=disable; openGauss does not declare sslmode
//     (leaves the driver default "prefer" untouched)
//   - User-supplied AdditionalParams override defaults — they are appended last,
//     and libpq's parser keeps the last occurrence of a duplicate key
//
// Values are escaped per libpq single-quoted form: backslashes are doubled and
// single quotes are escaped as `\'` so that passwords like `it's` survive
// transit to the driver. This matches the escape strategy used by
// github.com/lib/pq's url.go (the openGauss Go driver inherits the same logic).
func (d *Dialector) BuildDSN(dsn *driverV2.DSN) string {
	if dsn == nil {
		return ""
	}

	dbName := dsn.DatabaseName
	if dbName == "" {
		dbName = defaultDatabaseName
	}

	parts := []string{
		fmt.Sprintf("host=%s", escape(dsn.Host)),
		fmt.Sprintf("port=%s", escape(dsn.Port)),
		fmt.Sprintf("user=%s", escape(dsn.User)),
		fmt.Sprintf("password=%s", escape(dsn.Password)),
		fmt.Sprintf("dbname=%s", escape(dbName)),
		fmt.Sprintf("connect_timeout=%d", connectTimeoutSeconds),
	}

	// SSL: GaussDB commercial 默认 sslmode=disable 即可握手 (exploration §1.2
	// 实测 OK)。openGauss 默认走驱动行为 (sslmode=prefer)，无需显式声明。
	if d.Variant == consts.PluginNameGaussDB {
		parts = append(parts, "sslmode=disable")
	}

	// AdditionalParams 由 DBA 在 DMS 数据源 UI 显式填写。它们追加在末尾，
	// 使得 libpq 解析时同 key 会以**最后一个**为准，从而覆盖前面的默认值
	// (e.g. 用户填 sslmode=verify-full 会覆盖 GaussDB 的 sslmode=disable)。
	for _, p := range dsn.AdditionalParams {
		if p == nil || p.Key == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", p.Key, escape(p.Value)))
	}

	return strings.Join(parts, " ")
}

// Open implements the driverPkg.Dialector interface. It composes BuildDSN with
// BaseDialector.GetConn so that the same DSN logic flows through both unit
// tests (which call BuildDSN directly) and runtime connections.
func (d *Dialector) Open(dsn *driverV2.DSN) (*sql.DB, *sql.Conn, error) {
	_ = context.TODO // retained for parity with design §5.1 reference
	if dsn == nil {
		return nil, nil, errors.New("sqle-gaussdb-plugin: Dialector.Open requires non-nil DSN")
	}
	dataSourceName := d.BuildDSN(dsn)
	db, conn, err := d.BaseDialector.GetConn(d.DriverName(), dataSourceName)
	if err != nil {
		return nil, nil, errors.Wrap(err, "sqle-gaussdb-plugin: open connection")
	}
	return db, conn, nil
}

// escape returns v wrapped in single quotes with embedded backslashes and
// single quotes escaped per libpq's connection-string syntax. The wrapping
// is unconditional so that empty values still surface as `''`, which libpq
// accepts as "use default / unset".
func escape(v string) string {
	// Order matters: replace backslashes first so the escapes we introduce
	// for single quotes are not double-escaped.
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `'`, `\'`)
	return "'" + v + "'"
}
