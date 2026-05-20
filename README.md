# sqle-gaussdb-plugin

SQLE driver plugin for GaussDB / openGauss.

本仓库是 SQLE 的 GaussDB（含 openGauss）驱动插件，负责对接 GaussDB 数据源、支持慢日志扫描任务等能力。参照 `sqle-tbase-plugin` / `sqle-pg-plugin` 的插件模式实现。

## 关联 Issue

- actiontech/sqle-ee#2892 - feat(sqle): GaussDB 数据源支持慢日志扫描任务（含 openGauss）
- https://github.com/actiontech/sqle-ee/issues/2892

## 当前状态

仓库初始化中，具体实现将在后续任务中按 `docs/spec/design.md` 推进，包含但不限于：

- 驱动主程序与插件注册（main.go）
- 元数据/连通性能力
- GaussDB 慢日志采集与解析
- 单元测试覆盖

## 后续步骤

- Task-D2：plugin 主程序与 go.mod 初始化
- Task-D3：慢日志采集与解析
- 后续任务详见工作空间根目录 `docs/spec/plan.md` 与 `docs/dev/todo.md`
