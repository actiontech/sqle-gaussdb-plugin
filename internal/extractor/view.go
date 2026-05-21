// view.go implements design §6.2.2 — VIEW DDL extraction via
// pg_get_viewdef(c.oid, true) + relkind = 'v'，并在 SELECT 列表中用
// quote_ident 拼接 schema 限定的 CREATE OR REPLACE VIEW 头部。
//
// SQL 模板严格 byte-for-byte 匹配 design §6.2.2 line 424-432；pretty-print mode
// 必须保留 (oid, true) 第二参数（false 模式输出无换行难以 diff）。
//
// 红线（design §6.3）：schemaName / viewName 必须通过 $1 / $2 参数化绑定；
// 拼接到 SELECT 列表里的 schema 限定符通过 PostgreSQL 内建 quote_ident()
// 在数据库服务端处理，**禁止** Go 侧手工 escape。

package extractor

import (
	"context"
	"database/sql"
	"errors"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// extractViewSQL 是 VIEW DDL 抽取 SQL（design §6.2.2 line 424-432）。
//
// 头部显式拼 `CREATE OR REPLACE VIEW ...` 是因为 pg_get_viewdef 默认只返回
// SELECT 主体；schema 与 view 名通过 quote_ident 函数在服务端转义，确保
// PostgreSQL 标识符引用规则一致（exploration §3.2）。
const extractViewSQL = `SELECT
    'CREATE OR REPLACE VIEW ' || quote_ident(n.nspname) || '.' || quote_ident(c.relname) || ' AS '
    || pg_get_viewdef(c.oid, true)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1
  AND c.relkind = 'v'
  AND c.relname = $2`

// extractView 按 design §6.2.2 抽取单个视图的 DDL。
//
// 返回值约定与 extractTable 对齐（design §6.5）：
//
//   - 视图不存在 → 占位 ObjectDDL="" 不报错；
//   - 其他错误 → 透传原错误，保留 SQLSTATE。
//
// schemaName / viewName 不在 Go 侧做任何 escape——全部走 database/sql 参数化
// 绑定 + 服务端 quote_ident（design §6.3）。
func (e *Extractor) extractView(
	ctx context.Context,
	schemaName, viewName string,
) (*driverV2.DatabaseObjectDDL, error) {
	objectMeta := &driverV2.DatabaseObject{
		ObjectName: viewName,
		ObjectType: driverV2.ObjectType_VIEW,
	}

	var ddl string
	err := e.db.QueryRowContext(ctx, extractViewSQL, schemaName, viewName).Scan(&ddl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &driverV2.DatabaseObjectDDL{
				DatabaseObject: objectMeta,
				ObjectDDL:      "",
			}, nil
		}
		return nil, err
	}

	return &driverV2.DatabaseObjectDDL{
		DatabaseObject: objectMeta,
		ObjectDDL:      ddl,
	}, nil
}
