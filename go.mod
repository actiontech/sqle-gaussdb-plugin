module github.com/actiontech/sqle-gaussdb-plugin

go 1.19

require (
	// openGauss 官方 Go 驱动：唯一可同时贯通 GaussDB 8000 / openGauss 5432 的驱动
	// （exploration_dev.md §1 表格第 2 行；与 dms-ee 现有 vendor 对齐）。
	gitee.com/opengauss/openGauss-connector-go-pq v1.0.7
	// sqle 主仓 driver SDK：与 sqle-pg-plugin / sqle-tbase-plugin 同一 pseudo-version
	// （main-ee 路径 commit 2026-05-06 快照；exploration_dev.md §1 表格第 1 行）。
	github.com/actiontech/sqle v1.2210.1-0.20260506032938-299d6d6df8d1

	// driverV2 Plugin / DriverBuilder.Serve 的 hclog.Logger 参数类型
	// （sqle 主仓 pkg/driver 公开签名直接依赖该包）。
	github.com/hashicorp/go-hclog v0.14.1

	// SM3 国密包：openGauss 驱动间接传递，显式 require 以便 vendor 锁定
	// （exploration_dev.md §1 表格第 4 行；design §3.3 line 152）。
	github.com/tjfoc/gmsm v1.4.1 // indirect
)

// indirect 依赖：严格锁定到 sqle 主仓 vendor 内实测使用的版本，避免 `go mod tidy`
// 把 golang.org/x/{crypto,sys,net,text,sync} 等子模块升到 Go 1.20+ 才有的新版（如
// golang.org/x/crypto v0.42.0 会引用 crypto/ecdh / crypto/mlkem，但本仓库 go 1.19）。
// 版本号来源：/home/dev/workspace/sqle-pg-plugin/go.sum 与 sqle main-ee 实测。
require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/hashicorp/go-plugin v1.4.2 // indirect
	github.com/hashicorp/yamux v0.0.0-20180604194846-3520598351bb // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-testing-interface v1.14.0 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto v0.0.0-20220414192740-2d67ff6cf2b4 // indirect
	google.golang.org/grpc v1.50.1 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

require github.com/DATA-DOG/go-sqlmock v1.5.0

require (
	github.com/360EntSecGroup-Skylar/excelize v1.4.1 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/actiontech/dms v0.0.0-20260520024857-5bc318ea23da // indirect
	github.com/bwmarrin/snowflake v0.3.0 // indirect
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/denisenkom/go-mssqldb v0.9.0 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-sql-driver/mysql v1.7.0 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.10.0 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.1.1 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.8.1 // indirect
	github.com/jackc/pgx/v4 v4.13.0 // indirect
	github.com/labstack/echo/v4 v4.10.2 // indirect
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.4.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/pingcap/errors v0.11.5-0.20201126102027-b0a155152ca3 // indirect
	github.com/pingcap/log v0.0.0-20210317133921-96f4fcab92a4 // indirect
	github.com/pingcap/parser v3.0.12+incompatible // indirect
	github.com/pingcap/tidb v1.1.0-beta.0.20200630082100-328b6d0a955c // indirect
	github.com/pingcap/tipb v0.0.0-20200522051215-f31a15d98fce // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20190728182440-6a916e37a237 // indirect
	github.com/shirou/gopsutil v2.19.10+incompatible // indirect
	github.com/sijms/go-ora/v2 v2.2.15 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.17.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/telemetry v0.0.0-20260519152614-eab6ae52b5e2 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gorm.io/gorm v1.24.3 // indirect
	vitess.io/vitess v0.12.0 // indirect
)

// 本地路径 replace + 关键 indirect 降级：sqle 主仓与 sqle-pg-plugin 均为 actiontech
// 内网 GitLab 私有仓库，dev 沙箱无法访问内网 GOPROXY。本 replace 指向 workspace 同级
// 目录下的本地 checkout，让 `go build` / `go vet` / `go mod tidy` 在无外网环境也能
// 完成验证。CI 阶段与正式发布会从内网 GOPROXY 拉相应 tag，届时由 ce-ee-split 阶段
// 统一处理（vendor 落盘后即可移除本 replace 块）。
//
// 同时把 golang.org/x/{crypto,sys,net,text,sync,telemetry} 与 actiontech/dms 降级到
// Go 1.19 兼容版本，避免 tidy MVS 选到需要 Go 1.20+ 才有 crypto/ecdh / crypto/mlkem /
// slices 标准包的更新版本（典型崩溃版本：crypto v0.42.0 / sys v0.44.0）。版本号来自
// sqle-pg-plugin/go.sum 实测。
replace (
	github.com/actiontech/dms => github.com/actiontech/dms v0.0.0-20251027081421-309bc24335ca
	github.com/actiontech/sqle => ../sqle
	github.com/actiontech/sqle-pg-plugin => ../sqle-pg-plugin

	// sqle 主仓 vendor 内已锁定 sjjian/parser fork（含 parser.Token 类型扩展），
	// 本仓库 module 必须同步该 replace 才能让 ../sqle 内部的 mysql/splitter 包编译过。
	// 版本号取自 sqle-pg-plugin/go.mod line 79。
	github.com/pingcap/parser => github.com/sjjian/parser v0.0.0-20240704052347-b6199b7bccae

	golang.org/x/crypto => golang.org/x/crypto v0.14.0
	golang.org/x/net => golang.org/x/net v0.17.0
	golang.org/x/sync => golang.org/x/sync v0.1.0
	golang.org/x/sys => golang.org/x/sys v0.15.0
	golang.org/x/text => golang.org/x/text v0.14.0
)
