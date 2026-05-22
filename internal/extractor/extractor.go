// extractor.go — Extract 入口 + ObjectType switch 分发（Task-Dev-005）。
//
// 注：package extractor 的 godoc 包注释统一写在 doc.go；本文件顶部使用普通
// 注释块以避免 `go vet` 报 duplicate package documentation。
//
// 类型签名漂移说明（semantic/design_pseudocode_param_type_drift_20260521）：
// design §6.1 关键片段把 Extract 写成"对每个对象返回一个独立的
// *driverV2.DatabaseSchemaObjectResult"，但 real 链路
// (sqle/sqle/driver/v2/driver_interface.go:431-463) 中
// DatabaseSchemaObjectResult 是 **per-schema** 聚合的：
//
//	type DatabaseSchemaObjectResult struct {
//	    SchemaName         string
//	    SchemaDDL          string
//	    DatabaseObjectDDLs []*DatabaseObjectDDL
//	}
//	type DatabaseObjectDDL struct {
//	    DatabaseObject *DatabaseObject  // 含 ObjectName + ObjectType
//	    ObjectDDL      string
//	}
//
// 按"业务字段访问语义优先 + 文件头注释 + commit 标注"准则，本实现采用 real
// 链路签名：
//
//   - Extract(ctx, info) → 单个 *driverV2.DatabaseSchemaObjectResult，
//     SchemaName 取 info.SchemaName，DatabaseObjectDDLs 收集每个对象的 DDL；
//   - extractTable / extractView 内部 helper 返回 *driverV2.DatabaseObjectDDL；
//   - default 分支按 design §6.1 line 399 字面值返回错误。
//
// 红线（design §6.3）：所有 user-controlled 字符串（schema / object 名）必须
// 通过 `$1` / `$2` 参数化绑定；禁止 Go 侧手工 escape，转义责任在
// PostgreSQL 服务端 + libpq 驱动。

package extractor

import (
	"context"
	"database/sql"
	"fmt"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// Extractor 抽取 GaussDB / openGauss 的 schema 对象 DDL。
//
// 字段保持 unexported 与 design §6.1 伪代码一致；构造统一走 NewExtractor，
// 便于单测从外注入 sqlmock-backed *sql.DB 而无须真连 GaussDB / openGauss。
type Extractor struct {
	db *sql.DB
}

// NewExtractor 构造 Extractor，db 由调用方负责生命周期（通常是
// driverPkg.DriverImpl.DB，已套上连接池策略；driver.Extractor 接入留给
// Task-Dev-007）。
func NewExtractor(db *sql.DB) *Extractor {
	return &Extractor{db: db}
}

// Extract 按 design §6.1 入口分发 info.DatabaseObjects 中的每个对象到具体的
// extract* helper。
//
// 返回单个 *driverV2.DatabaseSchemaObjectResult：
//
//   - SchemaName 直接取 info.SchemaName；
//   - SchemaDDL 本期保持为空（design §6 未要求 schema 自身 DDL）；
//   - DatabaseObjectDDLs 按 info.DatabaseObjects 顺序收集，对象不存在时
//     保留一个 ObjectDDL=="" 的占位条目（design §6.5：让业务层
//     compareSchema 把它当作"该侧不存在"处理，**不报错**）。
//
// 分发覆盖 TABLE / VIEW / FUNCTION / PROCEDURE 四类；其他 ObjectType（INDEX /
// TRIGGER / EVENT / SEQUENCE / TYPE / PACKAGE 等）落入 default 分支返回
// design §6.1 line 399 字面错误 "object type %s is not supported in this release"。
//
// FUNCTION / PROCEDURE 与 TABLE / VIEW 的关键差异：helper 返回 slice 而非
// 单个 DDL（同名重载分裂为多条独立结果，design §6.4），故需用
// `append(..., rs...)` 展开。
func (e *Extractor) Extract(
	ctx context.Context,
	info *driverV2.DatabaseSchemaInfo,
) (*driverV2.DatabaseSchemaObjectResult, error) {
	if info == nil {
		return nil, fmt.Errorf("extractor: DatabaseSchemaInfo is nil")
	}

	result := &driverV2.DatabaseSchemaObjectResult{
		SchemaName: info.SchemaName,
	}

	// Task-Test-Fix-001 P0：sqle-ee 主路径 execute_comparison 传入空
	// DatabaseObjects 期待 plugin 按 SchemaName 全枚举（参考
	// docs/test/case-2-1.md 与 expertise_docs/episodic/
	// sqle_gaussdb_compare_main_path_empty_objects_bug_20260521.md）。
	// 在显式 switch 分发前做一次"空 → 全枚举"兜底，让后续循环按既有
	// 4 类对象分发逻辑处理，避免重复实现。
	objs := info.DatabaseObjects
	if len(objs) == 0 {
		enumerated, err := e.enumerateSchemaObjects(ctx, info.SchemaName)
		if err != nil {
			return nil, err
		}
		objs = enumerated
	}

	for _, obj := range objs {
		if obj == nil {
			continue
		}

		switch obj.ObjectType {
		case driverV2.ObjectType_TABLE:
			ddl, err := e.extractTable(ctx, info.SchemaName, obj.ObjectName)
			if err != nil {
				return nil, err
			}
			result.DatabaseObjectDDLs = append(result.DatabaseObjectDDLs, ddl)

		case driverV2.ObjectType_VIEW:
			ddl, err := e.extractView(ctx, info.SchemaName, obj.ObjectName)
			if err != nil {
				return nil, err
			}
			result.DatabaseObjectDDLs = append(result.DatabaseObjectDDLs, ddl)

		case driverV2.ObjectType_FUNCTION:
			// 同名重载：extractFunction 返回 slice，每条 ObjectName 携带
			// 参数签名（design §6.4），用 rs... 展开聚合。
			rs, err := e.extractFunction(ctx, info.SchemaName, obj.ObjectName)
			if err != nil {
				return nil, err
			}
			result.DatabaseObjectDDLs = append(result.DatabaseObjectDDLs, rs...)

		case driverV2.ObjectType_PROCEDURE:
			// 与 FUNCTION 同构；prokind 差异封装在 extractProcedure 内。
			rs, err := e.extractProcedure(ctx, info.SchemaName, obj.ObjectName)
			if err != nil {
				return nil, err
			}
			result.DatabaseObjectDDLs = append(result.DatabaseObjectDDLs, rs...)

		default:
			// design §6.1 line 399 字面值，对 INDEX / TRIGGER / EVENT 等本期不支持的对象统一返回。
			return nil, fmt.Errorf(
				"object type %s is not supported in this release",
				obj.ObjectType,
			)
		}
	}

	return result, nil
}
