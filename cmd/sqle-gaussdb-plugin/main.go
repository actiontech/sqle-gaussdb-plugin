// Package main is the GaussDB binary of sqle-gaussdb-plugin.
//
// Direction-A (physical isolation): this binary only registers PluginName
// "GaussDB". The sister binary cmd/sqle-opengauss-plugin registers
// "openGauss". sqle main repo's plugin_manager loads each binary as a separate
// process, so the strict-match contract (design §4.4) is structurally enforced
// — there is no in-process possibility of mis-routing a DriverType to the
// wrong PluginName.
//
// TODO(code_review): replace with driverPkg.ServeMulti once sqle main repo
// provides it (see design §3.4 line 199-203 and exploration_dev.md §3.1).
// When that lands, this file can be merged with cmd/sqle-opengauss-plugin/main.go
// into a single main.go calling driverPkg.ServeMulti(gaussBuilder, ogBuilder).
package main

import (
	"flag"
	"fmt"

	// Side-effect import: registers database/sql driver name="opengauss".
	// Hard constraint (design §1.4 line 31): GaussDB / openGauss plugins MUST
	// use this driver exclusively — no fallback to lib/pq or pgx is allowed.
	_ "gitee.com/opengauss/openGauss-connector-go-pq"

	"github.com/actiontech/sqle-gaussdb-plugin/internal/dialector"
	"github.com/actiontech/sqle-gaussdb-plugin/internal/driver"
	"github.com/actiontech/sqle-gaussdb-plugin/pkg/consts"
	"github.com/actiontech/sqle-gaussdb-plugin/pkg/logo"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
)

// version is injected at build time via -ldflags (see Makefile).
var version string

var printVersion = flag.Bool("version", false, "Print version & exit")

func main() {
	flag.Parse()
	if *printVersion {
		fmt.Println(version)
		return
	}

	builder := driverPkg.NewDriverBuilder(&dialector.Dialector{Variant: consts.PluginNameGaussDB})
	builder.Meta.PluginName = consts.PluginNameGaussDB
	builder.Meta.DatabaseDefaultPort = consts.DefaultPortGaussDB
	builder.Meta.Logo = logo.Bytes
	builder.SetEnableOptionalModule(
		driverV2.OptionalGetDatabaseObjectDDL,
		driverV2.OptionalGetDatabaseDiffModifySQL,
	)

	// Task-Dev-002 skeleton stage: driver.NewGaussDBDriverImpl is a thin stub
	// in internal/driver/stub.go that delegates to driverPkg.NewDriverImpl (a
	// compile-only placeholder). The real GaussDB Driver (with openGauss
	// driver-name="opengauss" connection + Ping / Connect / extractor wiring)
	// will replace stub.go in Task-Dev-003 (plan §3.1).
	builder.Serve(driver.NewGaussDBDriverImpl)
}
