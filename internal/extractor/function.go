// function.go implements design §6.2.3 — FUNCTION DDL extraction via
// pg_get_functiondef + pg_get_function_arguments + prokind = 'f'，并按 design
// §6.4 把 `ObjectName` 字段填为 `funcname(arg_type_list)` 形式，让上层
// compareSchema map key 天然唯一（同名重载分裂为多条独立结果）。
//
// SQL 模板严格 byte-for-byte 匹配 design §6.2.3 line 437-447 的扩展形态
// （在原 SELECT 列表追加 pg_get_function_arguments(p.oid) 第二列）；任何空白 /
// 换行 / 关键字大小写偏离都视为破坏 design 契约（单测用
// sqlmock.QueryMatcherEqual 锁定）。
//
// 红线（design §6.3）：schemaName / funcName 必须通过 $1 / $2 参数化绑定，
// 禁止任何 Go 侧的字符串拼接 / 手工 escape——转义责任在 PostgreSQL 服务端 +
// libpq 驱动。pg_get_function_arguments 返回参数签名字符串由服务端拼装，
// Go 侧仅做 `fmt.Sprintf("%s(%s)", funcName, arguments)` 拼接 ObjectName，
// 该拼接结果仅作为业务层 map key 使用，**不会**重新作为 SQL 文本下发。

package extractor

import (
	"context"
	"fmt"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// extractFunctionSQL 是 FUNCTION DDL + 参数签名抽取 SQL（design §6.2.3 line
// 437-447 + §6.4 重载识别）。
//
// 选用「单 SQL 返回两列」方案而非「两次查询 functiondef + arguments」：
//
//   - 减少一次 round-trip，对重载多行场景更友好（两次查询需自行按行号对齐）；
//   - 单条 SQL 内 pg_get_functiondef + pg_get_function_arguments 共享同一行
//     p.oid，天然保证 functiondef 与 arguments 一一对应；
//   - byte-for-byte 锁定一条 SQL 字面值即可，sqlmock 断言更直观。
//
// prokind = 'f' 严格区分 FUNCTION 与 PROCEDURE（exploration §1.2 (4) 实测
// GaussDB 505.2.1 + openGauss 6.0 两端 pg_proc.prokind 列均存在）。
const extractFunctionSQL = `SELECT pg_get_functiondef(p.oid), pg_get_function_arguments(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'f'
  AND p.proname = $2`

// extractFunction 按 design §6.2.3 + §6.4 抽取同名 FUNCTION（含重载）的所有
// DDL，每条独立返回。
//
// 返回值约定（design §6.5 + R-Q4-2）：
//
//   - 命中 N 个重载（N ≥ 1）：返回 N 个 *driverV2.DatabaseObjectDDL，ObjectName
//     拼为 `funcname(arg_type_list)`（无参数函数 arg_type_list 为空 → `fn()`）；
//   - 对象不存在（pg_proc 一行都没有命中）：返回**单条**占位
//     *driverV2.DatabaseObjectDDL，ObjectDDL=""，ObjectName 不带参数签名
//     （等价于业务层"该侧不存在"信号；与 TABLE/VIEW 兜底一致）；**不报错**；
//   - 其他错误（权限 / 网络 / SQLSTATE / rows.Err()）：原样透传，避免吞掉
//     openGauss / GaussDB 的诊断信息。
//
// schemaName / funcName 不在 Go 侧做任何 escape，全部走 database/sql 参数化
// 绑定（design §6.3）。
func (e *Extractor) extractFunction(
	ctx context.Context,
	schemaName, funcName string,
) ([]*driverV2.DatabaseObjectDDL, error) {
	rows, err := e.db.QueryContext(ctx, extractFunctionSQL, schemaName, funcName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*driverV2.DatabaseObjectDDL
	for rows.Next() {
		var functiondef, arguments string
		if scanErr := rows.Scan(&functiondef, &arguments); scanErr != nil {
			return nil, scanErr
		}
		results = append(results, &driverV2.DatabaseObjectDDL{
			DatabaseObject: &driverV2.DatabaseObject{
				ObjectName: fmt.Sprintf("%s(%s)", funcName, arguments),
				ObjectType: driverV2.ObjectType_FUNCTION,
			},
			ObjectDDL: functiondef,
		})
	}
	if iterErr := rows.Err(); iterErr != nil {
		return nil, iterErr
	}

	if len(results) == 0 {
		// 对象不存在：返回单条占位 DDL，ObjectName 不带参数签名（无任何重载
		// 信息可拼接），让业务层 compareSchema 把它当作"该侧不存在"处理。
		return []*driverV2.DatabaseObjectDDL{
			{
				DatabaseObject: &driverV2.DatabaseObject{
					ObjectName: funcName,
					ObjectType: driverV2.ObjectType_FUNCTION,
				},
				ObjectDDL: "",
			},
		}, nil
	}

	return results, nil
}
