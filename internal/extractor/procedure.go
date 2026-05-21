// procedure.go implements design §6.2.4 — PROCEDURE DDL extraction via
// pg_get_functiondef + pg_get_function_arguments + prokind = 'p'，结构与
// function.go 完全对齐，仅 prokind 字面值与 ObjectType 不同。
//
// 实现策略：与 function.go 同因——拆成两次单列查询规避 GaussDB 驱动多列
// 含 pg_get_functiondef 时的 record 字面值包裹（Task-Test-Fix-001 P1.1）。
//
// 红线（design §6.3）：schemaName / procName 必须通过 $1 / $2 参数化绑定，
// 禁止任何 Go 侧的字符串拼接 / 手工 escape——转义责任在 PostgreSQL 服务端 +
// libpq 驱动。

package extractor

import (
	"context"
	"fmt"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// extractProcedureDefSQL 单列查询：取所有同名 PROCEDURE 重载的 DDL 文本，
// 按 oid 升序返回。
const extractProcedureDefSQL = `SELECT pg_get_functiondef(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'p'
  AND p.proname = $2
ORDER BY p.oid`

// extractProcedureArgsSQL 单列查询：取所有同名 PROCEDURE 重载的参数签名字符串，
// 按 oid 升序返回（与 extractProcedureDefSQL 对齐）。
const extractProcedureArgsSQL = `SELECT pg_get_function_arguments(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'p'
  AND p.proname = $2
ORDER BY p.oid`

// extractProcedure 按 design §6.2.4 + §6.4 抽取同名 PROCEDURE（含重载）的所有
// DDL，每条独立返回。语义与 extractFunction 完全一致，仅 prokind 字面值差异。
//
// 复用 queryFuncDefs helper（在 function.go 定义）做单列字符串列表的两次
// query；两次结果按 oid ORDER BY 自然对齐。
func (e *Extractor) extractProcedure(
	ctx context.Context,
	schemaName, procName string,
) ([]*driverV2.DatabaseObjectDDL, error) {
	defs, err := e.queryFuncDefs(ctx, extractProcedureDefSQL, schemaName, procName)
	if err != nil {
		return nil, err
	}
	args, err := e.queryFuncDefs(ctx, extractProcedureArgsSQL, schemaName, procName)
	if err != nil {
		return nil, err
	}

	if len(defs) == 0 {
		return []*driverV2.DatabaseObjectDDL{
			{
				DatabaseObject: &driverV2.DatabaseObject{
					ObjectName: procName,
					ObjectType: driverV2.ObjectType_PROCEDURE,
				},
				ObjectDDL: "",
			},
		}, nil
	}

	pairCount := len(defs)
	if len(args) < pairCount {
		pairCount = len(args)
	}

	results := make([]*driverV2.DatabaseObjectDDL, 0, pairCount)
	for i := 0; i < pairCount; i++ {
		results = append(results, &driverV2.DatabaseObjectDDL{
			DatabaseObject: &driverV2.DatabaseObject{
				ObjectName: fmt.Sprintf("%s(%s)", procName, args[i]),
				ObjectType: driverV2.ObjectType_PROCEDURE,
			},
			ObjectDDL: defs[i],
		})
	}

	return results, nil
}
