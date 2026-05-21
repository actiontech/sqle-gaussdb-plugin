// driver_gaussdb.go implements the PluginName="GaussDB" variant of the
// sqle-gaussdb-plugin Driver.
//
// Structurally it is a thin aggregation over the shared Driver type defined
// in driver_common.go. The only GaussDB-specific behaviour at the Driver
// layer (vs the Dialector layer) is the variant tag carried on the embedded
// Driver, which lets sqlmock-backed unit tests assert that the two factory
// functions produce two distinct instances (design §17.1.5
// PluginNameRegistration).
//
// GetDatabaseObjectDDL / GetDatabaseDiffModifySQL are NOT overridden here —
// since Task-Dev-008 the real implementations live on *Driver (driver_common.go)
// and both GaussDBDriver and OpenGaussDriver inherit them via embedding. The
// per-variant override that previously returned an errNotImplemented sentinel
// has been removed because internal/extractor (Task-Dev-005 / 006) and
// internal/differ (Task-Dev-007) have landed.
package driver

import (
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	hclog "github.com/hashicorp/go-hclog"

	"github.com/actiontech/sqle-gaussdb-plugin/pkg/consts"
)

// GaussDBDriver is the driverV2.Driver implementation that
// cmd/sqle-gaussdb-plugin/main.go serves under PluginName="GaussDB".
type GaussDBDriver struct {
	*Driver
}

// Compile-time assertion that *GaussDBDriver satisfies driverV2.Driver. This
// is structurally guaranteed by the embedded *Driver (which in turn embeds
// *driverPkg.DriverImpl), but locking it here surfaces refactor breakage
// immediately.
var _ driverV2.Driver = (*GaussDBDriver)(nil)

// NewGaussDBDriverImpl is the Driver factory consumed by
// cmd/sqle-gaussdb-plugin/main.go via builder.Serve. The function signature
// matches driverPkg.DriverBuilder.Serve's fn parameter shape and is identical
// to the previous stub.go signature on purpose — preserving it means the cmd
// entrypoint never has to change between Task-Dev-002 (stub) and Task-Dev-004
// (real Driver).
func NewGaussDBDriverImpl(
	l hclog.Logger,
	dt driverPkg.Dialector,
	ah *driverPkg.AuditHandler,
	cfg *driverV2.Config,
) (driverV2.Driver, error) {
	base, err := newCommon(l, dt, ah, cfg, consts.PluginNameGaussDB)
	if err != nil {
		return nil, err
	}
	return &GaussDBDriver{Driver: base}, nil
}
