// Package main is the openGauss binary of sqle-gaussdb-plugin.
//
// Direction-A (physical isolation): this binary only registers PluginName
// "openGauss". The sister binary cmd/sqle-gaussdb-plugin registers "GaussDB".
// Together they form the two physically-isolated plugin processes that sqle's
// plugin_manager loads side-by-side — each process only ever Serve()s a single
// PluginName, structurally eliminating the "in-process multi-register order
// race" risk called out in impact_analysis.yml critical_boundaries[0].
//
// TODO(code_review): replace with driverPkg.ServeMulti once sqle main repo
// provides it (see design §3.4 line 199-203 and exploration_dev.md §3.1).
// When that lands, this file can be merged with cmd/sqle-gaussdb-plugin/main.go
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

	builder := driverPkg.NewDriverBuilder(&dialector.Dialector{Variant: consts.PluginNameOpenGauss})
	builder.Meta.PluginName = consts.PluginNameOpenGauss
	builder.Meta.DatabaseDefaultPort = consts.DefaultPortOpenGauss
	builder.Meta.Logo = logo.Bytes
	builder.SetEnableOptionalModule(
		driverV2.OptionalGetDatabaseObjectDDL,
		driverV2.OptionalGetDatabaseDiffModifySQL,
	)

	// Task-Dev-002 skeleton stage: driver.NewOpenGaussDriverImpl currently
	// delegates to sqle's built-in NewDriverImpl (see internal/driver/stub.go).
	// Task-Dev-003 replaces stub.go with the real driver_opengauss.go — same
	// connection path as the GaussDB binary minus the GaussDB-specific
	// sslmode=disable default.
	builder.Serve(driver.NewOpenGaussDriverImpl)
}
