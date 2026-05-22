// table.go implements design §6.2.1 — TABLE DDL extraction via
// pg_get_tabledef + relkind IN ('r', 'p').
//
// SQL 模板严格 byte-for-byte 匹配 design §6.2.1 line 411-418；任何空白 / 换行 /
// 关键字大小写偏离都视为破坏 design 契约（单测用
// sqlmock.QueryMatcherEqual 锁定）。
//
// 红线（design §6.3）：schemaName / tableName 必须通过 $1 / $2 参数化绑定，
// 禁止任何 Go 侧的字符串拼接 / 手工 escape——转义责任在 PostgreSQL 服务端 +
// libpq 驱动。

package extractor

import (
	"context"
	"database/sql"
	"errors"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// extractTableSQL 是 TABLE DDL 抽取 SQL（design §6.2.1 line 411-418）。
//
// 用 const 单一定义，便于单测端通过 import 同字面值断言；保持原始换行 + 缩进
// 风格与 design.md 代码块一致。
//
// relkind IN ('r', 'p') 同时覆盖普通表（r）与分区表（p），exploration §1.2 (4)
// 在 GaussDB 505.2.1 + openGauss 6.0 均实测可用。
const extractTableSQL = `SELECT pg_get_tabledef(c.oid)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1
  AND c.relkind IN ('r', 'p')
  AND c.relname = $2`

// extractTable 按 design §6.2.1 抽取单张表的 DDL。
//
// 返回值约定（design §6.5）：
//
//   - 对象不存在（pg_class 无行命中 → sql.ErrNoRows）：返回一个
//     ObjectDDL=="" 的占位 *driverV2.DatabaseObjectDDL，**不报错**，让业务层
//     compareSchema 把它当作"该侧不存在"处理；
//   - 其他错误（权限 / 网络 / SQLSTATE）：原样透传，避免吞掉
//     openGauss / GaussDB 的诊断信息（与 driver_common.go wrapPGError 策略一致）。
//
// schemaName / tableName 不在 Go 侧做任何 escape，全部走 database/sql 参数化
// 绑定（design §6.3）。
func (e *Extractor) extractTable(
	ctx context.Context,
	schemaName, tableName string,
) (*driverV2.DatabaseObjectDDL, error) {
	objectMeta := &driverV2.DatabaseObject{
		ObjectName: tableName,
		ObjectType: driverV2.ObjectType_TABLE,
	}

	var ddl string
	err := e.db.QueryRowContext(ctx, extractTableSQL, schemaName, tableName).Scan(&ddl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 对象不存在：返回空 DDL 占位，不报错。
			return &driverV2.DatabaseObjectDDL{
				DatabaseObject: objectMeta,
				ObjectDDL:      "",
			}, nil
		}
		// 透传原错误（含 SQLSTATE）。
		return nil, err
	}

	return &driverV2.DatabaseObjectDDL{
		DatabaseObject: objectMeta,
		ObjectDDL:      ddl,
	}, nil
}
