// Package consts 定义 sqle-gaussdb-plugin 仓库内的常量。
//
// 核心常量是 GaussDB / openGauss 两个 PluginName 与对应默认端口；
// PluginName 字面值必须严格匹配 sqle / sqle-ee 主仓
// sqle/sqle/driver/v2/util.go 中的 DriverType 常量（design §3.4 / §4.4）。
package consts

// PluginNameGaussDB 是 GaussDB 数据源在 sqle plugin_manager 中的注册名，
// 必须与 sqle/sqle/driver/v2/util.go 的 DriverTypeGaussDB 字面值完全一致
// （"GaussDB"，大小写敏感，禁止归一化）。详见 design.md §3.4 / §4.4。
const PluginNameGaussDB = "GaussDB"

// PluginNameOpenGauss 是 openGauss 数据源在 sqle plugin_manager 中的注册名，
// 必须与 sqle/sqle/driver/v2/util.go 的 DriverTypeOpenGauss 字面值完全一致
// （"openGauss"，大小写敏感，禁止归一化）。
// 本期由 Task §10.1 / §11.1 在 sqle 与 sqle-ee 主仓同步新增。
// 详见 design.md §3.4 / §4.4。
const PluginNameOpenGauss = "openGauss"

// DefaultPortGaussDB 是 GaussDB 商业版默认端口（design §4.3 表格 / exploration §1.2）。
const DefaultPortGaussDB = 8000

// DefaultPortOpenGauss 是 openGauss 6.0 默认端口（design §4.3 表格）。
const DefaultPortOpenGauss = 5432
