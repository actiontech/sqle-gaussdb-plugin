// enumerate.go — Extract 入口的"空 DatabaseObjects → 按 schema 全枚举"兜底
// （Task-Test-Fix-001 P0）。
//
// 背景：sqle-ee 主路径 execute_comparison 调用 plugin.GetDatabaseObjectDDL
// 时传入的 *DatabaseSchemaInfo 仅设置 SchemaName，DatabaseObjects 为 nil/空，
// 期望 plugin 按 SchemaName 自行枚举该 schema 下全部对象。
//
// 该约定在 design 文档与 sqle-ee/sqle/server/compare/database_compare_ee.go
// line 88-95 中存在，但 plugin Task-Dev-005 初版仅做"显式列表迭代"，遗漏了
// 该兜底——导致主路径 `inconsistent_num` 恒为 0（详见
// docs/test/case-2-1.md ~ case-2-4.md 与
// expertise_docs/episodic/sqle_gaussdb_compare_main_path_empty_objects_bug_20260521.md）。
//
// 实现策略：在 Extract 顶部调用 enumerateSchemaObjects 把 4 类对象的 name
// 列表先查出来，转成等价的 []*DatabaseObject 写回 info.DatabaseObjects，再走
// 既有 switch 分发逻辑，避免重复实现 4 类对象的抽取代码。
//
// SQL 模板说明：
//
//   - TABLE: pg_class.relkind = 'r'（普通表；分区表 'p' 不在本期 design §6.2.1
//     的对象类型枚举内，主路径暂不枚举分区表）；
//   - VIEW: pg_class.relkind = 'v'；
//   - FUNCTION: pg_proc.prokind = 'f'，GROUP BY p.proname 去重——重载分裂在
//     extractFunction helper 内部处理；
//   - PROCEDURE: pg_proc.prokind = 'p'，同 GROUP BY 去重。
//
// 所有 SQL 用 $1 参数化绑定 schema 名（design §6.3 红线），禁止 Go 侧拼接。
//
// 注：对象不存在 / schema 不存在 / 权限不足等错误由调用方透传，与
// table.go / view.go 既有的 wrapPGError 一致；本 helper 不做额外吞错。

package extractor

import (
	"context"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// listTablesSQL 枚举指定 schema 下所有普通表（pg_class.relkind = 'r'）。
//
// 严格 byte-for-byte 锁定，避免单测/集成测试偏离；ORDER BY relname 让结果
// 具备确定性（多 instance 测试间可直接做断言）。
const listTablesSQL = `SELECT c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1
  AND c.relkind = 'r'
ORDER BY c.relname`

// listViewsSQL 枚举指定 schema 下所有视图（pg_class.relkind = 'v'）。
const listViewsSQL = `SELECT c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1
  AND c.relkind = 'v'
ORDER BY c.relname`

// listFunctionsSQL 枚举指定 schema 下所有 FUNCTION 名（pg_proc.prokind = 'f'）。
//
// GROUP BY proname 去重：同名重载在本枚举阶段只产出**一个** ObjectName，重载
// 分裂在 extractFunction helper 内部按 (proname, pg_get_function_arguments)
// 全部展开（design §6.4），避免 enumerate → switch → extractFunction 链路里
// 出现重复查询。
const listFunctionsSQL = `SELECT p.proname
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'f'
GROUP BY p.proname
ORDER BY p.proname`

// listProceduresSQL 枚举指定 schema 下所有 PROCEDURE 名（pg_proc.prokind = 'p'）。
// 同 listFunctionsSQL 的 GROUP BY 去重策略。
const listProceduresSQL = `SELECT p.proname
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'p'
GROUP BY p.proname
ORDER BY p.proname`

// enumerateSchemaObjects 把 4 类对象的 name 列表汇集成等价的
// []*driverV2.DatabaseObject 切片，按 (TABLE → VIEW → FUNCTION → PROCEDURE)
// 顺序追加，单类内按 name 字典序（来自 SQL ORDER BY），整体顺序确定。
//
// 任一查询失败立即透传 error，不做 partial 返回——避免 plugin 把"半枚举"
// 当成"全枚举"造成主路径 inconsistent_num 误判。
func (e *Extractor) enumerateSchemaObjects(
	ctx context.Context,
	schemaName string,
) ([]*driverV2.DatabaseObject, error) {
	var objects []*driverV2.DatabaseObject

	type listSpec struct {
		sql        string
		objectType string
	}
	specs := []listSpec{
		{listTablesSQL, driverV2.ObjectType_TABLE},
		{listViewsSQL, driverV2.ObjectType_VIEW},
		{listFunctionsSQL, driverV2.ObjectType_FUNCTION},
		{listProceduresSQL, driverV2.ObjectType_PROCEDURE},
	}

	for _, spec := range specs {
		names, err := e.queryObjectNames(ctx, spec.sql, schemaName)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			objects = append(objects, &driverV2.DatabaseObject{
				ObjectName: n,
				ObjectType: spec.objectType,
			})
		}
	}

	return objects, nil
}

// queryObjectNames 是 4 类枚举 SQL 共用的"单列 name 列表"查询 helper。
// 入参 sqlText 已含 $1 占位，arg 仅 schema 名。
func (e *Extractor) queryObjectNames(
	ctx context.Context,
	sqlText, schemaName string,
) ([]string, error) {
	rows, err := e.db.QueryContext(ctx, sqlText, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if scanErr := rows.Scan(&n); scanErr != nil {
			return nil, scanErr
		}
		names = append(names, n)
	}
	if iterErr := rows.Err(); iterErr != nil {
		return nil, iterErr
	}
	return names, nil
}
