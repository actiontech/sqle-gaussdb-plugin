// driver_opengauss.go implements the PluginName="openGauss" variant of the
// sqle-gaussdb-plugin Driver. See driver_gaussdb.go for the structural
// rationale — this file mirrors that shape with only the variant tag /
// PluginName differing. Variant-specific connection behaviour (default port,
// default sslmode, libpq DSN layout) lives in internal/dialector and never
// in this layer; design §3.2 / §4.3 keeps the split deliberate.
package driver

import (
	"context"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	hclog "github.com/hashicorp/go-hclog"

	"github.com/actiontech/sqle-gaussdb-plugin/pkg/consts"
)

// OpenGaussDriver is the driverV2.Driver implementation that
// cmd/sqle-opengauss-plugin/main.go serves under PluginName="openGauss".
type OpenGaussDriver struct {
	*Driver
}

// Compile-time assertion: *OpenGaussDriver satisfies driverV2.Driver.
var _ driverV2.Driver = (*OpenGaussDriver)(nil)

// NewOpenGaussDriverImpl is the Driver factory consumed by
// cmd/sqle-opengauss-plugin/main.go via builder.Serve. Signature matches the
// previous stub.go exactly so the cmd entrypoint never has to change.
func NewOpenGaussDriverImpl(
	l hclog.Logger,
	dt driverPkg.Dialector,
	ah *driverPkg.AuditHandler,
	cfg *driverV2.Config,
) (driverV2.Driver, error) {
	base, err := newCommon(l, dt, ah, cfg, consts.PluginNameOpenGauss)
	if err != nil {
		return nil, err
	}
	return &OpenGaussDriver{Driver: base}, nil
}

// GetDatabaseObjectDDL — see driver_gaussdb.go for the rationale on returning
// errNotImplemented rather than relying on the embedded DriverImpl default.
// openGauss and GaussDB share the same upcoming extractor (internal/extractor
// is variant-agnostic per design §6.1), so this method's implementation will
// eventually delegate to the same code path as GaussDBDriver.
func (d *OpenGaussDriver) GetDatabaseObjectDDL(
	ctx context.Context,
	objInfos []*driverV2.DatabaseSchemaInfo,
) ([]*driverV2.DatabaseSchemaObjectResult, error) {
	return nil, errNotImplemented
}

// GetDatabaseDiffModifySQL — see driver_gaussdb.go. The differ is also
// variant-agnostic (design §7.1) so the eventual delegation matches its
// GaussDB sibling.
func (d *OpenGaussDriver) GetDatabaseDiffModifySQL(
	ctx context.Context,
	calibratedDSN *driverV2.DSN,
	objInfos []*driverV2.DatabasCompareSchemaInfo,
) ([]*driverV2.DatabaseDiffModifySQLResult, error) {
	return nil, errNotImplemented
}
