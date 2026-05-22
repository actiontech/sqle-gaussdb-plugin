// differ.go — GenerateModifySQLs 入口 + 4 类对象 × 3 种差异分类的 ModifySQL
// 字符串拼接实现（Task-Dev-007）。
//
// 注：package differ 的 godoc 包注释统一写在 doc.go；本文件顶部使用普通注释
// 块以避免 `go vet` 报 duplicate package documentation（参考 episodic
// go_package_doc_dual_file_collision_20260521）。
//
// 类型签名漂移说明（semantic/design_pseudocode_param_type_drift_20260521）：
// design §7 伪代码把 differ 入口写成
//
//	Generate(base, compared []*driverV2.DatabaseSchemaObjectResult) []string
//
// 但 real 链路（sqle/sqle/driver/v2/driver_interface.go:431-468）中
// DatabaseSchemaObjectResult 是 **per-schema** 聚合容器，包含 SchemaName 与
// DatabaseObjectDDLs，差异比对真正消费的是 schema-pair 内的对象 DDL 列表。
// 按"业务字段访问语义优先 + 文件头注释 + commit 标注偏离"准则，本实现将
// differ 入口入参定为 schema-pair 内的对象 DDL 列表：
//
//	GenerateModifySQLs(
//	    baseSchemaName string,
//	    baseDDLs []*driverV2.DatabaseObjectDDL,
//	    comparedSchemaName string,
//	    comparedDDLs []*driverV2.DatabaseObjectDDL,
//	) []string
//
// driver 接入（Task-Dev-008）时一次调用 extractor 取得 base / compared 两个
// *DatabaseSchemaObjectResult 后，把 .SchemaName 和 .DatabaseObjectDDLs 解构传
// 入，无需在 differ 层做二次类型转换；语义与 design §7 完全等价。
//
// 实现路径选择（design §7.1）：PG 系幂等 DDL（CREATE OR REPLACE VIEW /
// FUNCTION / PROCEDURE）+ TABLE 走 DROP/CREATE 兜底；本期不实现语义级 ALTER。
//
// SQL 注入兜底（design §6.3 + critical_boundaries[3]）：differ 不直接拼接用户
// 输入，schema/object 名字面值来自 extractor 上游产出（已通过 $1 / $2 参数化
// 绑定 + pg_get_* 服务端拼装），differ 仅做字符串拼接；**禁止** 在本层做任何
// 额外 escape / 引号包裹。

package differ

import (
	"fmt"
	"regexp"
	"strings"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

// rewriteSchemaQualifier 把 baseDDL 中所有 `baseSchema.` 前缀替换为 `comparedSchema.`，
// 用于 VIEW / FUNCTION / PROCEDURE 在 base 侧 ObjectDDL 含 CREATE OR REPLACE
// 形如 `CREATE OR REPLACE VIEW base.v AS ...` 时，把目标 schema 改写为 compared，
// 让变更 SQL 实际作用在 compared 侧（design §7.2 收敛方向；docs/test/case-2-2.md
// 与 case-2-3.md 缺陷溯源）。
//
// 替换策略：使用 strings.ReplaceAll 全字符串替换。该策略在结构化 DDL（pg_get_*
// 输出）中是安全的：
//
//   - CREATE OR REPLACE VIEW / FUNCTION / PROCEDURE 行中的 schema 限定一定形如
//     `schemaName.objectName`，替换后语义等价；
//   - 视图 / 函数体内引用了 base schema 同名对象（如 view 体 `FROM base.t_user`）
//     时一并被改写，与 base 侧 DDL 在 compared schema 上重建为"自包含定义"的
//     语义一致；
//   - 若 baseSchema 与 comparedSchema 相同（自对比兜底），ReplaceAll 是 no-op。
//
// 不做替换的两种边界：
//
//   - baseSchema 为空字符串：ReplaceAll 短路返回原 DDL；
//   - DDL 用双引号包裹了 schema（如 `"BaseSchema".obj`）：本期 schema 名全小写
//     不带引号，不进入此分支；未来若需支持引号场景可同步处理 `"`+baseSchema+`"`
//     形态。
func rewriteSchemaQualifier(ddl, baseSchema, comparedSchema string) string {
	if baseSchema == "" || baseSchema == comparedSchema {
		return ddl
	}
	// 用 `<schema>.` 作为最小可识别前缀，避免误命中字面值包含 schemaName 但
	// 不是限定符（虽然 pg_get_* 输出里通常不会出现这种边界，仍 defensive）。
	oldPrefix := baseSchema + "."
	newPrefix := comparedSchema + "."
	return strings.ReplaceAll(ddl, oldPrefix, newPrefix)
}

// setSearchPathPattern 匹配 pg_get_tabledef 输出的首行
// `SET search_path = <schema>;`。
//
// 设计要点：
//
//   - 行首 `^` + 多行模式 `(?m)`：仅匹配行首的 SET 语句，避免误伤 DDL 体
//     内（如表注释）的字面 "SET search_path"；
//   - `\s+` / `\s*` 容忍 GaussDB / openGauss 间空白宽容差异；
//   - schema 字面值用 capture group `([A-Za-z0-9_]+)`，限定标识符字符，
//     避免越界；本仓 schema 名全小写下划线（test_2905_base /
//     test_2905_compared），完全落在该字符集内；
//   - 末尾分号可选（pg_get_tabledef 实测带分号，但稳健性需要兼容缺分号）。
//
// 见 Task-Test-Fix-002 P2-A：pg_get_tabledef 在 GaussDB / openGauss 上返回
// `SET search_path = <baseSchema>;\nCREATE TABLE <name> (...) WITH (...);\n
// ALTER TABLE <name> ADD CONSTRAINT ...;`；TABLE 不带 schema 限定的 CREATE
// 依赖此 SET 行决定建在哪个 schema。Task-Test-Fix-001 P1.2 修复了 VIEW /
// FUNCTION / PROCEDURE 三类的 schema 限定改写（CREATE OR REPLACE schema.name
// 形态），但 TABLE 形态不同，需要单独改写 SET search_path 行（case-3-1.md
// 缺陷溯源）。
var setSearchPathPattern = regexp.MustCompile(`(?m)^SET\s+search_path\s*=\s*([A-Za-z0-9_]+)(\s*;?)`)

// rewriteSetSearchPath 把 TABLE DDL 文本（pg_get_tabledef 输出）首行
// `SET search_path = baseSchema;` 改写为 `SET search_path = comparedSchema;`，
// 让后续裸表名 CREATE TABLE 落在 compared schema 而非 base schema。
//
// 仅替换 `SET search_path = <baseSchema>` 这一精确匹配，不影响：
//
//   - DDL 体内（如 view 体 / function 体）任何 SET search_path 字面值（实际
//     pg_get_tabledef 输出不含 view/function 体，但稳健性需要锁定 base 字面值
//     匹配，避免 grep-style 误伤）；
//   - 替换后的 SET 语句末尾分号 / 空白原样保留（capture group $2 回写）。
//
// design §7.2 红线：schema 改写只能 base→compared 单向，不可反向；当 base 与
// compared schema 相同（自对比兜底）或 base 为空时短路返回原 DDL。
func rewriteSetSearchPath(ddl, baseSchema, comparedSchema string) string {
	if baseSchema == "" || baseSchema == comparedSchema {
		return ddl
	}
	// 用 ReplaceAllStringFunc 精确捕获 schema 字面值，仅当与 baseSchema 相
	// 等时才做替换（避免误命中其他 schema 的 SET 行 —— 实际 pg_get_tabledef
	// 输出只有一行 SET 且必然指向 baseSchema，仍保留这层 defensive 校验）。
	return setSearchPathPattern.ReplaceAllStringFunc(ddl, func(m string) string {
		sub := setSearchPathPattern.FindStringSubmatch(m)
		if len(sub) < 3 || sub[1] != baseSchema {
			return m
		}
		return "SET search_path = " + comparedSchema + sub[2]
	})
}

// dropTableWarningComment 是 DROP TABLE 语句前置的固定注释行，byte-for-byte
// 锁定（design §7.3 + impact_analysis.yml R-Q4-1 / risk_points 第 4 条）。
//
// 任何编辑（含空格 / 标点 / 大小写偏移）都会破坏工单审批人侧的格式契约——
// 该注释是 DROP TABLE 误删生产数据的唯一兜底（无第二道防御），单测
// TestDiffer_WarningCommentInjection 专门用字面值副本断言锁定。
const dropTableWarningComment = "-- WARNING: DROP TABLE will drop data. Review before executing."

// ustoreNormalizePattern 把 `storage_type=<USTORE 任意大小写>` 归一为
// `storage_type=ustore`，用于 DDL 字符串比较时的兜底（Task-Test-Fix-002 P4
// 兼容性合并修复）。
//
// 背景：GaussDB pg_get_tabledef 输出的 `WITH (orientation=row, storage_type=...)`
// 子句中 storage_type 的大小写在 base / compared 两侧偶发不一致——首次
// CREATE TABLE 服务端缓存写 `USTORE`，重建后变 `ustore`；同表结构因纯大小写
// 字面差异被识别为 inconsistent，误报变更。pg 系数据源的 WITH 子句字面值
// 大小写不影响存储引擎语义。case-2-1.md 遗留次要 P4 缺陷溯源。
var ustoreNormalizePattern = regexp.MustCompile(`(?i)storage_type\s*=\s*ustore`)

// normalizeDDLForCompare 把 DDL 文本做字符串比较前的归一化：
//
//  1. P4: `storage_type=USTORE` 大小写归一到 `storage_type=ustore`；
//
// 仅用于"两侧 ObjectDDL 是否相等"短路判断的副本比较，**不**改原 DDL（落盘到
// modify SQL 时仍是 base 侧原文，避免影响下游执行）。
//
// 后续 P4 之外的归一化项可以在该 helper 内部继续累加（如未来发现的其他
// 大小写规范化点），保持调用点单一。
func normalizeDDLForCompare(s string) string {
	return ustoreNormalizePattern.ReplaceAllString(s, "storage_type=ustore")
}

// objectKey 是 (ObjectName, ObjectType) 二元组私有类型，用于在 base / compared
// 两侧 DDL 列表间建立索引。
//
// FUNCTION / PROCEDURE 同名重载在 Task-Dev-006 已让 ObjectName 携带参数签名
// （形如 `fn(integer, text)`），故 (ObjectName, ObjectType) 元组天然唯一，
// 不会把 N 个重载坍缩成一条（参考 semantic
// sqle_plugin_overload_objectname_signature_20260521）。
type objectKey struct {
	Name string
	Type string
}

// GenerateModifySQLs 对单个 schema-pair 内的 base/compared DDL 列表做差异
// 对比，按 design §7.2 矩阵生成 ModifySQLs 字符串切片。
//
// 入参顺序：base 三元组先于 compared 三元组；driver 接入时 base/compared 一
// 一对应（base 为目标形态、compared 为源端形态——让 compared 收敛到 base 形态）。
//
// 对比策略（design §6.4 + §7.2 / §7.4）：
//
//  1. 按 (ObjectName, ObjectType) 元组建 map（baseMap / comparedMap）；
//  2. 遍历 compared 一次，处理"仅 compared 侧有"与"两侧都有但 ObjectDDL 不同"
//     两类；过程中将命中的 base key 标记，剩余 base map 未访问的 key 即"仅
//     base 侧有"；
//  3. 两侧都有且 ObjectDDL 完全相同 → 不输出。
//
// 本函数无 db 依赖、无 ctx 依赖、无副作用，纯字符串拼接。
//
// 不实现的场景（design §7.5）：分区表 SPLIT/MERGE、列存变更、分布键变更等
// 随 `pg_get_tabledef` 原文携带，按整表 DROP+CREATE 兜底；INDEX / TRIGGER 本
// 期不在抽取范围 → DDL 不出现 → diff 不出现。
//
// 防御性处理：对意外的 ObjectType（如 INDEX / TRIGGER）跳过不生成 ModifySQL
// 且不 panic（extractor 上游已拒绝 unsupported 类型）。
func GenerateModifySQLs(
	baseSchemaName string,
	baseDDLs []*driverV2.DatabaseObjectDDL,
	comparedSchemaName string,
	comparedDDLs []*driverV2.DatabaseObjectDDL,
) []string {
	baseMap := buildObjectMap(baseDDLs)
	comparedMap := buildObjectMap(comparedDDLs)

	modifySQLs := make([]string, 0)
	consumedBaseKeys := make(map[objectKey]bool, len(baseMap))

	// 第一遍：遍历 compared 处理"仅 compared 侧有"与"两侧都有但不同"。
	for key, comparedDDL := range comparedMap {
		baseDDL, existsInBase := baseMap[key]
		if !existsInBase {
			// 仅 compared 侧有 → 生成 DROP（让 compared 收敛到 base 形态）。
			if sql := genOnlyComparedSide(comparedSchemaName, key.Type, comparedDDL); sql != "" {
				modifySQLs = append(modifySQLs, sql)
			}
			continue
		}

		consumedBaseKeys[key] = true

		// 两侧都有：比较 ObjectDDL 文本（含 Task-Test-Fix-002 P5 + P4
		// 双层归一化）：
		//
		//  P5（schema 替换前置）：把 base 侧 ObjectDDL 内的 baseSchema. 限定
		//   全部替换为 comparedSchema.，让"仅 schema 限定不同 + body 相同"的
		//   场景（典型：FUNCTION/PROCEDURE 重载的 same body）也能被识别为
		//   "无实质差异"，不输出冗余 CREATE OR REPLACE（case-2-3.md 遗留
		//   次要缺陷）；
		//
		//  P4（USTORE 大小写归一）：把两侧 DDL 中 `storage_type=USTORE` /
		//   `storage_type=ustore` 归一到统一形态再比较，避免 GaussDB
		//   pg_get_tabledef 输出的大小写漂移误判（case-2-1.md 遗留次要缺陷）；
		//
		// 比较仅作用于 normalize 后的副本，不修改原 ObjectDDL；命中相等时
		// 短路跳过，否则继续走 genBothDiff 输出真实变更。
		// 双层 schema 改写：rewriteSchemaQualifier 处理 `schema.name` 限定形态
		// （VIEW/FUNCTION/PROCEDURE 路径），rewriteSetSearchPath 处理
		// `SET search_path = schema` 形态（TABLE 路径）。两者互不干扰：
		// rewriteSchemaQualifier 走的是 `<schema>.` 前缀模式，不会误伤
		// SET 行；rewriteSetSearchPath 走的是行首正则，不会误伤 schema.name。
		baseAfterSchemaRewrite := rewriteSetSearchPath(
			rewriteSchemaQualifier(baseDDL.ObjectDDL, baseSchemaName, comparedSchemaName),
			baseSchemaName, comparedSchemaName,
		)
		baseForCompare := normalizeDDLForCompare(baseAfterSchemaRewrite)
		comparedForCompare := normalizeDDLForCompare(comparedDDL.ObjectDDL)
		if baseForCompare == comparedForCompare {
			// 完全相同（含 schema 改写后的等价 + USTORE 归一） → 无变更，跳过
			// （Task-Test-Fix-001 P1.3 重载 same DDL 短路 + Task-Test-Fix-002
			// P4/P5 兼容性归一短路）。
			continue
		}

		// 两侧都有但不同 → 按对象类型生成"双向收敛"语句。
		if sql := genBothDiff(baseSchemaName, comparedSchemaName, key.Type, baseDDL); sql != "" {
			modifySQLs = append(modifySQLs, sql)
		}
	}

	// 第二遍：扫描 base map 中未消费的 key 即"仅 base 侧有"。
	for key, baseDDL := range baseMap {
		if consumedBaseKeys[key] {
			continue
		}
		if sql := genOnlyBaseSide(baseSchemaName, comparedSchemaName, key.Type, baseDDL); sql != "" {
			modifySQLs = append(modifySQLs, sql)
		}
	}

	return modifySQLs
}

// buildObjectMap 按 (ObjectName, ObjectType) 元组把 DDL 列表索引化。
//
// 防御性跳过 nil 条目 / nil DatabaseObject，避免 driver / extractor 上游偶发
// 空指针时 panic。
//
// 注：differ 入参语义约定为"已过滤好的真实存在对象列表"，extractor 上游兜
// 底「对象不存在 → 占位 ObjectDDL==""」（design §6.5）由 sqle-ee 业务层
// compareSchema 在调用 differ 之前过滤掉占位条目；differ 层不再二次识别
// ObjectDDL==""，避免双重语义入口。
func buildObjectMap(ddls []*driverV2.DatabaseObjectDDL) map[objectKey]*driverV2.DatabaseObjectDDL {
	m := make(map[objectKey]*driverV2.DatabaseObjectDDL, len(ddls))
	for _, ddl := range ddls {
		if ddl == nil || ddl.DatabaseObject == nil {
			continue
		}
		key := objectKey{
			Name: ddl.DatabaseObject.ObjectName,
			Type: ddl.DatabaseObject.ObjectType,
		}
		m[key] = ddl
	}
	return m
}

// genOnlyBaseSide 处理"仅 base 侧有"分支（design §7.2 表第 1 行）。
//
// 该分支的语义是"compared 侧不存在该对象 → 在 compared schema 上重建 base 形态"。
// 对 4 类对象的处理：
//
//   - TABLE: pg_get_tabledef 原文（含 CREATE TABLE）。extractor 上游产出形态为
//     `SET search_path = baseSchema;\nCREATE TABLE <name> (...);\n...`，CREATE
//     是裸表名，依赖 SET 行决定建在哪个 schema。Task-Test-Fix-002 P2-A：
//     必须把首行 SET search_path 改写为 comparedSchema，让 CREATE 落在
//     compared 侧（case-3-1.md 缺陷溯源）；
//   - VIEW / FUNCTION / PROCEDURE: ObjectDDL 含 `CREATE OR REPLACE X base.name`，
//     必须把 base schema 前缀替换为 compared schema，让变更 SQL 在 compared
//     侧创建对象（Task-Test-Fix-001 P1.2；design §7.2 收敛方向）。
//
// 不支持 ObjectType 返回空字符串，调用方跳过。
func genOnlyBaseSide(baseSchema, comparedSchema, objectType string, baseDDL *driverV2.DatabaseObjectDDL) string {
	switch objectType {
	case driverV2.ObjectType_TABLE:
		return rewriteSetSearchPath(baseDDL.ObjectDDL, baseSchema, comparedSchema)
	case driverV2.ObjectType_VIEW,
		driverV2.ObjectType_FUNCTION,
		driverV2.ObjectType_PROCEDURE:
		return rewriteSchemaQualifier(baseDDL.ObjectDDL, baseSchema, comparedSchema)
	default:
		return ""
	}
}

// genOnlyComparedSide 处理"仅 compared 侧有"分支（design §7.2 表第 2 行）。
//
// schema 参数取 comparedSchemaName：DROP 发生在 compared schema 上，让目标库
// 收敛到 base schema 形态。
//
// 输出格式（无引号包裹，PG 系大小写敏感由 ObjectName 字面值表达；schema 与
// 对象名之间用 `.` 分隔；无尾随分号）：
//
//   - TABLE     -> dropTableWarningComment + "\n" + DROP TABLE IF EXISTS schema.name
//   - VIEW      -> DROP VIEW IF EXISTS schema.name
//   - FUNCTION  -> DROP FUNCTION IF EXISTS schema.name(arg_signature)   // R-Q4-2
//   - PROCEDURE -> DROP PROCEDURE IF EXISTS schema.name(arg_signature)  // R-Q4-2
//
// FUNCTION / PROCEDURE 的参数签名已在 Task-Dev-006 落地到 ObjectName 字段
// （形如 `fn(integer, text)`），直接拼字面值即满足 R-Q4-2（同名重载 DROP 必
// 须带参数列表才能被 PG 唯一定位）。
func genOnlyComparedSide(schema, objectType string, comparedDDL *driverV2.DatabaseObjectDDL) string {
	objectName := comparedDDL.DatabaseObject.ObjectName
	switch objectType {
	case driverV2.ObjectType_TABLE:
		return dropTableWarningComment + "\n" +
			fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", schema, objectName)
	case driverV2.ObjectType_VIEW:
		return fmt.Sprintf("DROP VIEW IF EXISTS %s.%s", schema, objectName)
	case driverV2.ObjectType_FUNCTION:
		return fmt.Sprintf("DROP FUNCTION IF EXISTS %s.%s", schema, objectName)
	case driverV2.ObjectType_PROCEDURE:
		return fmt.Sprintf("DROP PROCEDURE IF EXISTS %s.%s", schema, objectName)
	default:
		return ""
	}
}

// genBothDiff 处理"两侧都有但 ObjectDDL 不同"分支（design §7.2 表第 3 行）。
//
// TABLE 走 DROP+CREATE 兜底（PG 系无 ALTER TABLE 幂等路径，整表替换是
// design §7.1 选定方案）；VIEW / FUNCTION / PROCEDURE 用 base 侧 ObjectDDL
// （已含 CREATE OR REPLACE，PG 系幂等），但 schema 限定需改写为 compared
// （Task-Test-Fix-001 P1.2；同 genOnlyBaseSide 处理）。
//
// schema 参数取 comparedSchema：DROP 发生在 compared schema 上；后续
// CREATE TABLE 也作用于 compared 端（base 侧 ObjectDDL 含完整 schema 限定，
// 由 extractor 上游 pg_get_tabledef 拼装）。
func genBothDiff(baseSchema, comparedSchema, objectType string, baseDDL *driverV2.DatabaseObjectDDL) string {
	switch objectType {
	case driverV2.ObjectType_TABLE:
		// 形态：WARNING 注释 + "\n" + DROP TABLE IF EXISTS schema.name; +
		// "\n" + CREATE TABLE 原文（已改写 SET search_path 到 compared 侧）。
		// Task-Test-Fix-002 P2-A：与 genOnlyBaseSide 同步改写 SET search_path，
		// 否则两侧都有但 DDL 不同的 TABLE 仍会撞 base schema 同名表
		// （case-3-1.md / case-2-1.md 系列缺陷的同源根因）。
		return dropTableWarningComment + "\n" +
			fmt.Sprintf("DROP TABLE IF EXISTS %s.%s;", comparedSchema, baseDDL.DatabaseObject.ObjectName) +
			"\n" + rewriteSetSearchPath(baseDDL.ObjectDDL, baseSchema, comparedSchema)
	case driverV2.ObjectType_VIEW,
		driverV2.ObjectType_FUNCTION,
		driverV2.ObjectType_PROCEDURE:
		return rewriteSchemaQualifier(baseDDL.ObjectDDL, baseSchema, comparedSchema)
	default:
		return ""
	}
}
