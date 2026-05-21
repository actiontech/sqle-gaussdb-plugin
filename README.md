# sqle-gaussdb-plugin

> SQLE 的 GaussDB / openGauss 插件，提供同类型数据库结构对比（TABLE / VIEW / FUNCTION / PROCEDURE 四类对象）能力。
>
> 仓库定位：本仓库是 SQLE EE 专有的插件仓库，由 `sqle/sqle/driver/plugin_manager.go` 在主进程启动时通过 `hashicorp/go-plugin` 自动加载（参考 `docs/spec/design.md` §3.1）。

---

## 支持的 DBType

| PluginName | 数据源协议族 | 默认端口 | 说明 |
|-----------|------------|---------|------|
| `GaussDB` | PostgreSQL 协议（私有认证 RFC5802 SHA-256 / SM3） | 8000 | 华为 GaussDB 商业版（含集中式/分布式部署） |
| `openGauss` | PostgreSQL 协议 | 5432 | 开源 openGauss 6.0+ |

PluginName 字面值必须与 sqle 主仓 `sqle/sqle/driver/v2/util.go` 中的 `DriverTypeGaussDB` / `DriverTypeOpenGauss` 完全一致（大小写敏感，禁止归一化）。详见 `docs/spec/design.md` §4.4。

---

## 构建方式

```bash
# 使用 golang:1.19.6 镜像在容器内编译，与 sqle-pg-plugin / sqle-tbase-plugin 镜像一致
make docker_install
```

构建产物（位于 `./bin/`）：

```
bin/
├── sqle-gaussdb-plugin    # PluginName=GaussDB,   默认端口 8000
└── sqle-opengauss-plugin  # PluginName=openGauss, 默认端口 5432
```

如本地已有 Go 1.19.x 环境，也可直接 `make install`（不进容器）。

---

## 部署位置

将两个二进制都拷贝到 SQLE 主进程的插件目录：

```
${SQLE_BIN}/plugins/sqle-gaussdb-plugin
${SQLE_BIN}/plugins/sqle-opengauss-plugin
```

SQLE 主进程启动时 `plugin_manager.Start()` 会扫描 `${SQLE_BIN}/plugins/`，每个可执行文件起一个独立进程，分别注册 PluginName=`GaussDB` 与 `openGauss`。

### 启动日志校验

SQLE 主进程启动完成后应在日志中看到 **两条** 注册成功的记录：

```
... level=info ... msg="plugin loaded" PluginName=GaussDB
... level=info ... msg="plugin loaded" PluginName=openGauss
```

如果只看到其中一条，请确认两个二进制都已 `chmod +x` 并位于 `${SQLE_BIN}/plugins/` 目录。

---

## 仓库目录结构

```
sqle-gaussdb-plugin/
├── go.mod                                  # 4 个核心依赖锁定（sqle / openGauss-connector-go-pq / gmsm / sqle-pg-plugin）
├── Makefile                                # 双二进制 build target
├── README.md
├── cmd/
│   ├── sqle-gaussdb-plugin/main.go         # PluginName=GaussDB   入口
│   └── sqle-opengauss-plugin/main.go       # PluginName=openGauss 入口
├── internal/
│   ├── dialector/                          # 自定义 Dialector（driverName="opengauss"、DSN 构造）
│   ├── driver/                             # Driver 实现（Ping / Exec / Query + GetDatabaseObjectDDL + GetDatabaseDiffModifySQL）
│   ├── extractor/                          # DDL 抽取
│   ├── differ/                             # 变更 SQL 生成
│   └── inspector/                          # 复用 sqle-pg-plugin 规则集
├── pkg/
│   ├── consts/                             # PluginName / DefaultPort 常量
│   └── logo/                               # 插件 logo 字节流（与 sqle-tbase-plugin 同源占位）
├── plocale/                                # 国际化 toml（与 sqle 主仓 locale 独立）
└── _test/                                  # 单测固定 schema + map case 数据
```

详细分包职责与各 capability 实现矩阵见 `docs/spec/design.md` §3.2 / §5.2。

---

## 关于双独立二进制方案的临时性说明

当前 SQLE 主仓 `sqle/sqle/pkg/driver/builder.go` 仅提供 `Serve()` 单 PluginName 注册入口，**未提供 `ServeMulti()`**（见 `docs/dev/exploration_dev.md` §2 探针结论 B）。本仓库按 design §3.4 line 199–203 的**方向 A（双独立二进制）**落地：

- 每个二进制内只 `Serve()` 一个 PluginName，物理上消除"单进程多 PluginName 注册顺序错位"风险（critical_boundaries[0]）；
- 待 SQLE 主仓在 `code_review` 阶段补齐 `driverPkg.ServeMulti(gauss, og)` 后，可将两个 cmd/main.go 合并为单个、Makefile 删掉双 target，回切到 design §3.4 主路径。

切换时机与切换步骤由 manager 在 plan.md 中决策。

---

## 依赖版本说明

| 依赖 | 版本 | 说明 |
|------|------|------|
| `github.com/actiontech/sqle` | `v1.2210.1-0.20260506032938-299d6d6df8d1` | 与 sqle-pg-plugin / sqle-tbase-plugin 同步，main-ee 路径 2026-05-06 快照 |
| `gitee.com/opengauss/openGauss-connector-go-pq` | `v1.0.7` | openGauss 官方驱动，唯一可同时贯通 GaussDB 8000 / openGauss 5432 |
| `github.com/tjfoc/gmsm` | `v1.4.1` | SM3 国密包（openGauss 驱动间接传递），显式 require 锁定 vendor |
| `github.com/actiontech/sqle-pg-plugin` | `v4.2604.0` | 解析器 + 规则集复用，main 分支最新 release tag |

四个依赖版本与 dms-ee 现有 vendor 对齐，参见 `docs/dev/exploration_dev.md` §1。

---

## License & Maintainer

- 仓库归属：`actiontech` 私有，归 SQLE 企业版插件矩阵管理
- 维护：SQLE EE Team
