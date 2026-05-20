package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00225 = "SQLE00225"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00225,
			Desc:       "禁止使用非分片键对分片表进行关联",
			Annotation: "在数据库中执行存在分片表关联的SQL语句时，如表关联条件未使用分片键时，会导致大量的跨分片查询和数据传输，严重影响性能，且消耗更多系统资源如网络带宽和内存。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "禁止使用非分片键对分片表进行关联",
		RawSQLHandler:        RuleSQLE00225,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00225): "在 TBase 中，禁止使用非分片键对分片表进行关联."
您应遵循以下逻辑：
1. 对于"SELECT ... "语句
	1. 对于语句中有 LEFT JOIN、RIGHT JOIN、INNER JOIN、JOIN 等标准表关联的子句，定义一个集合，把ON 关键词后面的多表关联字段写入集合）；
	2. 对于语句中 FROM  子句后面表以逗号分隔的内联，比如 FROM T1,T2 WHERE T1.ID = T2.ID， 则把WHERE 关键词后面的多表关联字段写入集合）；
	3. 登录数据库，分别查询SQL语句中每张表的分片键，并且查询出来后和集合中的关联键进行对比，如果关联键不是分片键，则报告违反规则。
2. 对于"UPDATE" 语句，执行与上述同样检查
3. 对于"DELETE" 语句，执行与上述同样检查
4. 对于"WITH" 语句，执行与上述同样检查
5. 对于"INSERT ... SELECT " 语句，执行与上述同样检查

1. 对于UNION...语句, 对于其中的所有SELECT子句进行与SELECT语句相同的检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00225(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取CTE表
	var cteTableNames []string
	for _, expr := range utilGetCommonTableExpr(node.GetStmt()) {
		cteTableNames = append(cteTableNames, expr.Ctename)
	}

	// 检查关联键是否是分片键
	checkShardKeyViolation := func(tables []*parser.RangeVar, joinKeys map[string]map[string]struct{}) (bool, error) {
		if len(joinKeys) == 0 {
			return false, nil
		}

		// 创建一个映射，用于存储每个表的分片键
		shardKeys := make(map[string]string)

		// 遍历所有的表，获取它们的分片键，去除 CTE 表
		for _, table := range tables {
			if utilIsStrInSlice(table.Relname, cteTableNames) {
				continue
			}
			shardKey, err := utilGetTableDistributionColumn(ctx, table.Schemaname, table.Relname)
			if err != nil {
				return false, errors.Wrap(err, "get table distribution column")
			}
			shardKeys[table.Relname] = shardKey
			if alias := utilGetTableAliasName(table); alias != "" {
				shardKeys[alias] = shardKey
			}
		}

		// 检查每个 joinKeys 的元素是否都是某个表的分片键
		for tableName, keys := range joinKeys {
			// 去除 CTE 表
			if utilIsStrInSlice(tableName, cteTableNames) {
				continue
			}

			foundShardKey := false
			for key := range keys {
				for shardTableName, shardKey := range shardKeys {
					if tableName == "" && key == shardKey {
						foundShardKey = true
						break
					}
					if tableName == shardTableName && key == shardKey {
						foundShardKey = true
						break
					}
				}
				if foundShardKey {
					break
				}
			}
			if !foundShardKey {
				return true, nil // 违反规则
			}
		}

		return false, nil
	}

	// 处理 where t1.id = t2.id 的情况
	getAllColAsJoinKeysFromWhere := func(whereClause *parser.Node) map[string]map[string]struct{} {
		joinKeys := make(map[string]map[string]struct{})

		utilScanWhereStmt(func(expr *parser.Node) (skip bool) {
			for _, col := range utilGetColumnRefFromNodes(expr) {
				_, tableName, colName := utilParseColumnRef(col)
				if _, exists := joinKeys[tableName]; !exists {
					joinKeys[tableName] = make(map[string]struct{})
				}
				joinKeys[tableName][colName] = struct{}{}
			}

			return false
		}, whereClause)
		return joinKeys
	}

	// 处理 where user_id in (select id from users where id =1 )
	getJoinKeysFromSubLink := func(subLink *parser.SubLink) map[string]map[string]struct{} {
		joinKeys := make(map[string]map[string]struct{})
		if subLink.Testexpr == nil {
			return joinKeys
		}

		if col, ok := subLink.Testexpr.Node.(*parser.Node_ColumnRef); ok {
			_, tableName, colName := utilParseColumnRef(col.ColumnRef)
			if _, exists := joinKeys[tableName]; !exists {
				joinKeys[tableName] = make(map[string]struct{})
			}
			joinKeys[tableName][colName] = struct{}{}
		}

		return joinKeys

	}

	// 处理使用Join进行关联的情况
	getJoinKeysFromJoinExpr := func(joinExpr *parser.JoinExpr) map[string]map[string]struct{} {
		joinKeys := make(map[string]map[string]struct{})
		if joinExpr != nil && joinExpr.Quals != nil {
			for _, col := range utilGetColumnRefFromNodes(joinExpr.Quals) {
				_, tableName, colName := utilParseColumnRef(col)
				if _, exists := joinKeys[tableName]; !exists {
					joinKeys[tableName] = make(map[string]struct{})
				}
				joinKeys[tableName][colName] = struct{}{}
			}
		}
		return joinKeys
	}

	// 获取 From 子句中的表
	getTablesFromFromClause := func(fromClause []*parser.Node) []*parser.RangeVar {
		rangeVars := []*parser.RangeVar{}
		for _, from := range fromClause {
			for _, rangeVar := range utilGetRangeVars(from) {
				rangeVars = append(rangeVars, rangeVar)
			}
		}
		return rangeVars
	}

	// 判断是否为 from t1, t2, t3 的形式
	isImplicitJoin := func(selectStmt *parser.SelectStmt) bool {
		if selectStmt.FromClause == nil {
			return false
		}
		for _, node := range selectStmt.FromClause {
			if _, ok := (*node).Node.(*parser.Node_RangeVar); !ok {
				return false
			}
		}
		return true
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查

	// 处理所有DML语句中的SELECT子句
	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_InsertStmt, *parser.Node_DeleteStmt, *parser.Node_UpdateStmt:
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			tables := getTablesFromFromClause(selectStmt.FromClause)

			// 对于语句中 FROM  子句后面表以逗号分隔的内联，比如 FROM T1,T2 WHERE T1.ID = T2.ID...
			if isImplicitJoin(selectStmt) {
				joinKeysFromWhere := getAllColAsJoinKeysFromWhere(selectStmt.WhereClause)

				if violation, err := checkShardKeyViolation(tables, joinKeysFromWhere); err != nil {
					return "", err
				} else if violation {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}

			// 对于语句中有 LEFT JOIN、RIGHT JOIN、INNER JOIN、JOIN 等标准表关联的子句...
			for _, joinExpr := range utilGetJoinExpr(&parser.Node{Node: &parser.Node_SelectStmt{SelectStmt: selectStmt}}) {
				joinKeysFromJoinExpr := getJoinKeysFromJoinExpr(joinExpr)
				if violation, err := checkShardKeyViolation(tables, joinKeysFromJoinExpr); err != nil {
					return "", err
				} else if violation {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}

		}
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_UpdateStmt:
		updateStmt := stmt.UpdateStmt
		for _, sub := range utilGetSubLink(node.GetStmt()) {
			joinKeys := getJoinKeysFromSubLink(sub)
			if violation, err := checkShardKeyViolation([]*parser.RangeVar{updateStmt.Relation}, joinKeys); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_DeleteStmt:
		deleteStmt := stmt.DeleteStmt
		for _, sub := range utilGetSubLink(node.GetStmt()) {
			joinKeys := getJoinKeysFromSubLink(sub)
			if violation, err := checkShardKeyViolation([]*parser.RangeVar{deleteStmt.Relation}, joinKeys); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
