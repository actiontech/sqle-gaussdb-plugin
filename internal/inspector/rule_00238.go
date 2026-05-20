package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00238 = "SQLE00238"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00238,
			Desc:       "建议分片键使用指定数据类型",
			Annotation: "建议分片键使用int、bigint、smallint、char、varchar类型，以确保数据分布均匀并优化查询性能，避免使用其他类型导致的性能和维护问题。",
			Level:      driverV2.RuleLevelWarn,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "建议分片键使用指定数据类型",
		RawSQLHandler:        RuleSQLE00238,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00238): "在 TBase 中，建议分片键使用指定数据类型."
您应遵循以下逻辑：
1. 对于"CREATE TABLE..."语句，检查SQL语句:
	1. 不是分区子表，即不包含关键词：partition of，进行下一步检查
  2. 如果有关键词 distribute by shard(filed1)，把field1 对应的数据类型和集合（int,bigint,smallint，char，varchar）对比，如果不是其子集，则报告违反规则。
  3. 如果没有关键词 distribute ，那么把语句的第一个字段的数据类型拿出来和集合（int,bigint,smallint，char，varchar）对比，如果不是其子集，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00238(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查分片键的数据类型是否符合要求
	checkShardKeyType := func(columnDefNode *parser.Node) bool {
		return utilIsColumnTypeEqual(columnDefNode, SqlTypeInt4, SqlTypeInt8, SqlTypeInt2, SqlTypeVarchar, SqlTypeChar)
	}

	// 获取创建表语句中的分片键
	getShardKeyFromCreateTable := func(createStmt *parser.CreateStmt) (string, *parser.Node) {
		if createStmt.Distributeby != nil {
			shardKey := ""
			for _, column := range createStmt.Distributeby.Colname {
				if column.GetString_() != nil {
					shardKey = column.GetString_().GetSval()
				}
			}
			for _, elt := range createStmt.TableElts {
				if utilGetColumnName(elt) == shardKey {
					return shardKey, elt
				}
			}
		}
		return "", nil
	}

	// 获取创建表语句中的第一个字段
	getFirstColumnFromCreateTable := func(createStmt *parser.CreateStmt) *parser.Node {
		if len(createStmt.TableElts) > 0 {
			return createStmt.TableElts[0]
		}
		return nil
	}

	// 主逻辑：根据不同的 DDL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		createStmt := stmt.CreateStmt

		// 检查是否为分区子表
		if len(stmt.CreateStmt.InhRelations) > 0 {
			// 存在分区子表，不违反规则
			return "", nil
		}

		// 检查是否有分片键
		shardKey, shardKeyNode := getShardKeyFromCreateTable(createStmt)
		if shardKey != "" {
			// 检查分片键的数据类型是否符合要求
			valid := checkShardKeyType(shardKeyNode)
			if !valid {
				return RuleHandlerMap[rule.Name].Message, nil // 违反规则
			}
		} else {
			// 没有分片键，检查第一个字段的数据类型是否符合要求
			firstColumn := getFirstColumnFromCreateTable(createStmt)
			if firstColumn != nil {
				valid := checkShardKeyType(firstColumn)
				if !valid {
					return RuleHandlerMap[rule.Name].Message, nil // 违反规则
				}
			}
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
