// Package driver 提供 sqle-gaussdb-plugin 的 driverV2.Driver 实现。
//
// 本文件 stub.go 为 Task-Dev-002 阶段的最小可编译占位：
//   - 通过嵌入 driverPkg.DriverImpl 自动继承 driverV2.Driver 接口的全部方法实现
//     （Close / Ping / Exec / Audit / Parse / GetDatabaseObjectDDL / GetDatabaseDiffModifySQL 等）；
//   - 暴露 NewGaussDBDriverImpl / NewOpenGaussDriverImpl 两个工厂函数，
//     供 cmd/sqle-gaussdb-plugin/main.go 与 cmd/sqle-opengauss-plugin/main.go 各自
//     调用 builder.Serve(...) 时使用；
//   - 真正连接 GaussDB / openGauss 的逻辑（基于 dialector.BuildDSN + sql.Open 的
//     "opengauss" driver name）见 Task-Dev-003（plan §4.1 driver_common.go）。
//
// 提示：本 stub 不再使用 `//go:build !real_driver` 等 build tag —— Task-Dev-003
// 起会**原地替换**本文件为真实实现（driver_common.go / driver_gaussdb.go /
// driver_opengauss.go 三文件），见 plan §4.1 / §4.2 / §4.3。
package driver

import (
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	hclog "github.com/hashicorp/go-hclog"
)

// NewGaussDBDriverImpl 是 PluginName=GaussDB 二进制（cmd/sqle-gaussdb-plugin/main.go）
// 调用 builder.Serve 时使用的 Driver 工厂函数。
//
// 与 PluginName=openGauss 共用同一段 stub 实现 —— 真正的差异化逻辑（DSN 默认端口、
// sslmode 等）在 Dialector 层做分支，Driver 层保持统一（design §3.2 / §4.3）。
//
// TODO(Task-Dev-003): 替换为 driver_gaussdb.go 中的真实实现。
func NewGaussDBDriverImpl(l hclog.Logger, dt driverPkg.Dialector, ah *driverPkg.AuditHandler, cfg *driverV2.Config) (driverV2.Driver, error) {
	return newStub(l, dt, ah, cfg)
}

// NewOpenGaussDriverImpl 是 PluginName=openGauss 二进制（cmd/sqle-opengauss-plugin/main.go）
// 调用 builder.Serve 时使用的 Driver 工厂函数。
//
// TODO(Task-Dev-003): 替换为 driver_opengauss.go 中的真实实现。
func NewOpenGaussDriverImpl(l hclog.Logger, dt driverPkg.Dialector, ah *driverPkg.AuditHandler, cfg *driverV2.Config) (driverV2.Driver, error) {
	return newStub(l, dt, ah, cfg)
}

// newStub 是 GaussDB / openGauss 两个工厂函数共用的内部构造函数。
// 委托给 driverPkg.NewDriverImpl —— 该工厂会按 cfg.DSN 是否为 nil 决定是否调用
// Dialector.Open；本 stub 阶段 Open 返回 "not implemented yet" 错误，
// 真正连库的代码在 Task-Dev-003 落地。
func newStub(l hclog.Logger, dt driverPkg.Dialector, ah *driverPkg.AuditHandler, cfg *driverV2.Config) (driverV2.Driver, error) {
	return driverPkg.NewDriverImpl(l, dt, ah, cfg)
}
