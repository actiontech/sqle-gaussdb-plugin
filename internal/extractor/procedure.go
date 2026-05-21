// procedure.go implements design §6.2.4 — PROCEDURE DDL extraction via
// pg_get_functiondef + pg_get_function_arguments + prokind = 'p'，结构与
// function.go 完全对齐，仅 prokind 字面值与 ObjectType 不同。
//
// SQL 模板严格 byte-for-byte 匹配 design §6.2.4 line 449-457 的扩展形态
// （追加 pg_get_function_arguments(p.oid) 第二列）；任何空白 / 换行 / 关键字
// 大小写偏离都视为破坏 design 契约（单测用 sqlmock.QueryMatcherEqual 锁定）。
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

// extractProcedureSQL 是 PROCEDURE DDL + 参数签名抽取 SQL（design §6.2.4 line
// 449-457 + §6.4 重载识别）。
//
// 与 extractFunctionSQL 同结构，仅 `prokind = 'p'` 不同，避免与 FUNCTION
// 命中同一行（PostgreSQL pg_proc.prokind: 'f' = function, 'p' = procedure,
// 'a' = aggregate, 'w' = window）。
const extractProcedureSQL = `SELECT pg_get_functiondef(p.oid), pg_get_function_arguments(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'p'
  AND p.proname = $2`

// extractProcedure 按 design §6.2.4 + §6.4 抽取同名 PROCEDURE（含重载）的所有
// DDL，每条独立返回。
//
// 返回值约定与 extractFunction 完全一致（design §6.5 + R-Q4-2）：
//
//   - 命中 N 个重载 → N 个 *driverV2.DatabaseObjectDDL，ObjectName 拼为
//     `procname(arg_type_list)`，DROP PROCEDURE 时上层会自然带上参数签名；
//   - 对象不存在 → 单条占位 ObjectDDL=""，ObjectName 不带参数签名，不报错；
//   - 其他错误 → 透传原错误，保留 SQLSTATE。
//
// schemaName / procName 不在 Go 侧做任何 escape，全部走 database/sql 参数化
// 绑定（design §6.3）。
func (e *Extractor) extractProcedure(
	ctx context.Context,
	schemaName, procName string,
) ([]*driverV2.DatabaseObjectDDL, error) {
	rows, err := e.db.QueryContext(ctx, extractProcedureSQL, schemaName, procName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*driverV2.DatabaseObjectDDL
	for rows.Next() {
		var procdef, arguments string
		if scanErr := rows.Scan(&procdef, &arguments); scanErr != nil {
			return nil, scanErr
		}
		results = append(results, &driverV2.DatabaseObjectDDL{
			DatabaseObject: &driverV2.DatabaseObject{
				ObjectName: fmt.Sprintf("%s(%s)", procName, arguments),
				ObjectType: driverV2.ObjectType_PROCEDURE,
			},
			ObjectDDL: procdef,
		})
	}
	if iterErr := rows.Err(); iterErr != nil {
		return nil, iterErr
	}

	if len(results) == 0 {
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

	return results, nil
}
