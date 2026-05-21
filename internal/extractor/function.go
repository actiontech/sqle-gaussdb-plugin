// function.go implements design §6.2.3 — FUNCTION DDL extraction via
// pg_get_functiondef + pg_get_function_arguments + prokind = 'f'，并按 design
// §6.4 把 `ObjectName` 字段填为 `funcname(arg_type_list)` 形式，让上层
// compareSchema map key 天然唯一（同名重载分裂为多条独立结果）。
//
// 实现策略（Task-Test-Fix-001 P1.1）：拆成两次单列查询而非一次双列查询。
//
// 历史背景：Task-Dev-006 初版用单 SQL 双列 SELECT
//
//	SELECT pg_get_functiondef(p.oid), pg_get_function_arguments(p.oid) FROM ...
//
// 在 GaussDB（kernel 505.2.1）+ openGauss 官方 Go 驱动
// `gitee.com/opengauss/openGauss-connector-go-pq` 下，多列 SELECT 含
// `pg_get_functiondef` 时驱动会把整行打包成 record/composite 字面值返回，
// 形如 `(4,"CREATE OR REPLACE FUNCTION ...")`，Scan 到两个 string 也无法去除
// 外层 tuple 包裹。表象：API 旁路 modify_sql_statements 输出的 sql_statement
// 文本前缀含 `(N,"..."`、尾部含 `")"`，复制到 psql 直接执行报语法错误
// （详见 docs/test/case-2-3.md 与 case-2-4.md）。
//
// 修复方案：拆成两次单列 query，按 `ORDER BY p.oid` 对齐两份结果（同 oid 即
// 同重载实例）。两次 round-trip 的代价远低于"输出不可执行 SQL"的体验损失。
//
// SQL 模板：byte-for-byte 锁定（sqlmock.QueryMatcherEqual 单测验证）。
//
// 红线（design §6.3）：schemaName / funcName 必须通过 $1 / $2 参数化绑定，
// 禁止任何 Go 侧的字符串拼接 / 手工 escape——转义责任在 PostgreSQL 服务端 +
// libpq 驱动。`pg_get_function_arguments` 返回参数签名字符串由服务端拼装，
// Go 侧仅做 `fmt.Sprintf("%s(%s)", funcName, arguments)` 拼接 ObjectName，
// 该拼接结果仅作为业务层 map key 使用，**不会**重新作为 SQL 文本下发。

package extractor

import (
	"context"
	"fmt"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// extractFunctionDefSQL 单列查询：取所有同名 FUNCTION 重载的 DDL 文本，按 oid
// 升序返回。
//
// 单列查询规避 GaussDB 驱动把多列含 `pg_get_functiondef` 的行打包成 record
// 字面值的问题（Task-Test-Fix-001 P1.1）。
const extractFunctionDefSQL = `SELECT pg_get_functiondef(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'f'
  AND p.proname = $2
ORDER BY p.oid`

// extractFunctionArgsSQL 单列查询：取所有同名 FUNCTION 重载的参数签名字符串，
// 按 oid 升序返回（与 extractFunctionDefSQL 的 oid 顺序对齐）。
const extractFunctionArgsSQL = `SELECT pg_get_function_arguments(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $1
  AND p.prokind = 'f'
  AND p.proname = $2
ORDER BY p.oid`

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
// 两次单列查询若返回行数不一致：按"min(len(defs), len(args))"对齐取较短一段，
// 不报错——一致性破坏极小概率（同事务并发增删重载），且对结果集语义无破坏。
// schemaName / funcName 不在 Go 侧做任何 escape，全部走 database/sql 参数化
// 绑定（design §6.3）。
func (e *Extractor) extractFunction(
	ctx context.Context,
	schemaName, funcName string,
) ([]*driverV2.DatabaseObjectDDL, error) {
	defs, err := e.queryFuncDefs(ctx, extractFunctionDefSQL, schemaName, funcName)
	if err != nil {
		return nil, err
	}
	args, err := e.queryFuncDefs(ctx, extractFunctionArgsSQL, schemaName, funcName)
	if err != nil {
		return nil, err
	}

	if len(defs) == 0 {
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

	// 对齐两份单列结果（同 oid 顺序）；防御性处理长度不一致。
	pairCount := len(defs)
	if len(args) < pairCount {
		pairCount = len(args)
	}

	results := make([]*driverV2.DatabaseObjectDDL, 0, pairCount)
	for i := 0; i < pairCount; i++ {
		results = append(results, &driverV2.DatabaseObjectDDL{
			DatabaseObject: &driverV2.DatabaseObject{
				ObjectName: fmt.Sprintf("%s(%s)", funcName, args[i]),
				ObjectType: driverV2.ObjectType_FUNCTION,
			},
			ObjectDDL: defs[i],
		})
	}

	return results, nil
}

// queryFuncDefs 是 FUNCTION / PROCEDURE 单列查询共用的 "list 单列 string" helper。
//
// 与 enumerate.go 的 queryObjectNames 同结构，但单独维护一个 helper 让 SQL
// 文本 + 参数语义自描述（明确是 functiondef / arguments 列表），便于阅读。
func (e *Extractor) queryFuncDefs(
	ctx context.Context,
	sqlText, schemaName, objectName string,
) ([]string, error) {
	rows, err := e.db.QueryContext(ctx, sqlText, schemaName, objectName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var v string
		if scanErr := rows.Scan(&v); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, v)
	}
	if iterErr := rows.Err(); iterErr != nil {
		return nil, iterErr
	}
	return out, nil
}
