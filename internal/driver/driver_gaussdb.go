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
// The two extractor / differ entry-points (GetDatabaseObjectDDL /
// GetDatabaseDiffModifySQL) are overridden with explicit "not implemented yet"
// errors so that callers cannot accidentally rely on the embedded DriverImpl
// default — which returns an empty slice with no error and would silently mask
// the missing Task-Dev-005 / Task-Dev-006 work.
package driver

import (
	"context"

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

// GetDatabaseObjectDDL is the entry-point for the DDL extractor. The
// implementation is deferred to Task-Dev-005 (plan §5); the current Driver
// layer surfaces a clear sentinel error so DMS sees a deterministic failure
// rather than the empty result the embedded DriverImpl default would return.
//
// Once Task-Dev-005 lands this method will delegate to internal/extractor.
func (d *GaussDBDriver) GetDatabaseObjectDDL(
	ctx context.Context,
	objInfos []*driverV2.DatabaseSchemaInfo,
) ([]*driverV2.DatabaseSchemaObjectResult, error) {
	return nil, errNotImplemented
}

// GetDatabaseDiffModifySQL is the entry-point for the structural differ. The
// implementation is deferred to Task-Dev-006 (plan §6); the current Driver
// layer surfaces a clear sentinel error so DMS sees a deterministic failure
// rather than the empty result the embedded DriverImpl default would return.
//
// Once Task-Dev-006 lands this method will delegate to internal/differ.
func (d *GaussDBDriver) GetDatabaseDiffModifySQL(
	ctx context.Context,
	calibratedDSN *driverV2.DSN,
	objInfos []*driverV2.DatabasCompareSchemaInfo,
) ([]*driverV2.DatabaseDiffModifySQLResult, error) {
	return nil, errNotImplemented
}
