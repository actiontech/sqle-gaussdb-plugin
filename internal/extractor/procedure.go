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
	"regexp"
	"strings"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// plsqlTrailingSlashPattern 匹配 PROCEDURE DDL 末尾独立行 `/`（PL/SQL 终止符）。
//
// 设计要点：
//
//   - `(?m)`：多行模式让 `^` / `$` 匹配行首行尾，方便识别"独立行"形态；
//   - `\s*\n+`：允许 `/` 之前有空白行或空格；
//   - `^\s*/\s*$`：`/` 单独占一行（前后允许空白）；
//   - `\s*\z`：行尾以及文件末尾的尾随空白都吞掉，让剥除结果干净。
//
// 背景（semantic/opengauss_procedure_slash_terminator_driver_incompat_20260521）：
// openGauss `pg_get_functiondef(oid)` 对 PROCEDURE 类型（prokind='p'）返回的
// DDL 末尾会包含一行独立的 `/`，是 PL/SQL 风格的客户端语句终止符（gsql /
// SQL*Plus 风格）；Go database/sql + openGauss-connector-go-pq、JDBC、psycopg2
// 等"驱动直连+逐语句解析"通道不识别 `/` 作为语句终止符，会撞
// `syntax error at or near "/"`。FUNCTION（prokind='f'）输出不带末尾 `/`（结尾是
// `$function$;`），所以仅 PROCEDURE 路径需要剥除。
//
// 中间内嵌 `/`（理论上不会出现在 PL/SQL body 内，因 `/` 是 gsql 终止符，
// PL/SQL 块体合法语法不含字面值 `/` 单独占行）不会被该正则误命中——只剥除
// 文末尾部的 `/`。
var plsqlTrailingSlashPattern = regexp.MustCompile(`(?m)\n\s*/\s*\z`)

// stripPLSQLTerminator 把 PROCEDURE DDL 末尾的 PL/SQL 终止符 `/` 行剥除，
// 让 Go/JDBC 驱动可以逐语句直接执行（Task-Test-Fix-002 P3）。
//
// 实现策略：先 TrimRight 一次尾随空白，再用正则剥末尾独立 `/` 行；剥除后
// 再 TrimRight 一次清掉剩余空白。这样无论 pg_get_functiondef 输出末尾是
// `...\nEND;\n/\n` 还是 `...\nEND;\n/` 都能正确处理。
func stripPLSQLTerminator(def string) string {
	def = strings.TrimRight(def, " \t\r\n")
	def = plsqlTrailingSlashPattern.ReplaceAllString(def, "")
	return strings.TrimRight(def, " \t\r\n")
}

// extractProcedureDefSQL 单列查询：取所有同名 PROCEDURE 重载的 DDL 文本，
// 按 oid 升序返回。
//
// 注意 `(pg_get_functiondef(p.oid)).definition` 的显式字段访问——同 function.go
// 注释里说明的 GaussDB record 返回类型问题（Task-Test-Fix-001 P1.1）。
const extractProcedureDefSQL = `SELECT (pg_get_functiondef(p.oid)).definition
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
		// Task-Test-Fix-002 P3: 剥除 pg_get_functiondef 对 PROCEDURE 返回值
		// 末尾的 PL/SQL 终止符 `/`，让 Go/JDBC 驱动可以直接执行变更 SQL；
		// FUNCTION 路径（extractFunction）不需要此处理（不带 `/`）。
		results = append(results, &driverV2.DatabaseObjectDDL{
			DatabaseObject: &driverV2.DatabaseObject{
				ObjectName: fmt.Sprintf("%s(%s)", procName, args[i]),
				ObjectType: driverV2.ObjectType_PROCEDURE,
			},
			ObjectDDL: stripPLSQLTerminator(defs[i]),
		})
	}

	return results, nil
}
