// Package inspector 是后续 Task-Dev-003+ 中 import-复用 sqle-pg-plugin 规则集
// 与 SQL 解析器的复用入口（参考 docs/spec/design.md §3.5）。
//
// 在 Task-Dev-002 骨架阶段，本文件仅保留 package 声明作为占位 —— 由于
// sqle-pg-plugin 的 `internal/inspector` 包在 Go 语言规范下不能被外部 module
// 直接 import（internal/ 包对外不可见），本期不在此处放 dummy import。
//
// 等 Task-Dev-003 在 sqle-pg-plugin 中暴露 public re-export（或通过 sqle-tbase-plugin
// 同款"共享 module path" 模式重组复用通道）后，再补齐 reuse 入口；调研结论与决策
// 参考 expertise_docs/semantic/sqle_plugin_module_path_shared_20260521.md。
package inspector
