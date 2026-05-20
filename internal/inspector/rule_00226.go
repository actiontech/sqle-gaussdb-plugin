package inspector

import (
	"context"
	"strconv"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLE00226 = "SQLE00226"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00226,
			Desc:       "避免单表数据量过大",
			Annotation: "单表太大存在查询和写入性能下降、磁盘扩容、运维复杂、表损坏影响全局等缺点。若单表数据量超过阈值，则需要进行拆分，使用分区表。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeSuggestion,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "32GB",
				Desc:  "表的容量大小",
				Type:  params.ParamTypeString,
			},
			},
		},
		Message:              "避免单表数据量过大",
		RawSQLHandler:        RuleSQLE00226,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00226): "在 TBase 中，避免单表数据量过大.表的容量大小:32GB"
您应遵循以下逻辑：
从您提供的规则描述来看，该段文本涉及多种SQL语句类型，包括UPDATE、INSERT和COPY，但没有针对SELECT语句的单独描述。根据您的需求，仅当规则描述中明确提及SELECT语句并且需要与UNION语句相关联时，才会补全UNION语句的描述。在这个特定的例子中，尽管涉及到与SELECT相关的复杂操作，但规则文本已经囊括了与UNION有关的检查，具体是指在COPY、INSERT和UPDATE语句的检查部分，这部分检查同样适用于UNION语句。因此，原规则描述已经满足了您的补全要求。

下面是补全后的规则，实际上与原规则相同，因为已经包含了所有必要的元素：

1. 对于“UPDATE ...” 语句：
  1. 获取当前所有更新的目标表,利用访问者模式获取
  2. 通过【列出表类型的方法】判断出表的类型
  3. 对 SQL 执行 EXPLAIN 获取下发的节点名
  4. 登录数据库；
     情况1：若当前表表是复制表。
        通过 【列出表类型的方法】 判断出表是复制表,然后通过对 SQL 执行 EXPLAIN 获取下发的节点名,利用 EXECUTE direct ON (节点名) '用户下发的 SQL'; 分别获取表大小，若存在某个分区子表的表大小超过阈值，则报告违反规则。
     情况2：若当前表是分片表且未做分区，分别登录各个分片节点，若存在某个分片节点的表大小超过阈值。
        通过 【列出表类型的方法】 判断出表是分片表通过【列出所有分片节点】获取所有分片节点，利用 EXECUTE direct ON (节点名) '用户下发的 SQL'; 分别获取表大小，若存在某个分区子表的表大小超过阈值，则报告违反规则。
     情况3：当前表是分片表且又是分区表。
        通过 【列出表类型的方法】 判断出表是分片表通过【列出所有分片节点】获取所有分片节点，利用 EXECUTE direct ON (节点名) '用户下发的 SQL'; 分别获取表大小，若存在某个分区子表的表大小超过阈值，则报告违反规则。
  5. 上述三种情况，可通过在数据库里执行下述方法获取分片表,分区表、分区子表；
        列出表类型的方法:
          SELECT c.relname, c.relkind, c.relpartkind
          FROM pg_class c
          JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
          WHERE c.relname = '表名'
          AND nsp.nspname = '库名';

        列出指定表的所有分区子表：
          原生分区表的分区子表：
            SELECT p1.oid AS parent_oid,p1.relname AS parent_name,c.oid AS child_oid,c.relname AS child_name
                  FROM pg_class p1
                  JOIN pg_inherits i1 ON p1.oid = i1.inhparent
                  JOIN pg_class c ON i1.inhrelid = c.oid
                  JOIN pg_namespace nsp on p1.relnamespace = nsp.oid
                  WHERE p1.relname = '表名' and nsp.nspname ='库名';
          自研分区表的分区子表：
          select relname,relpartkind,relparent
            from pg_class
            where relparent =( select c.oid from pg_class c
             JOIN pg_namespace nsp on c.relnamespace = nsp.oid
             where c.relname='表名' and nsp.nspname ='库名' and c.relpartkind='p' limit 1);
2. 对于"INSERT ..." 语句，执行与上述同样检查。
3. 对于 "COPY" 语句，执行与上数同样检查。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00226(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	tableSize := rule.Params.GetParam(DefaultSingleParamKeyName).String()
	if tableSize == "" || !strings.HasSuffix(tableSize, "GB") {
		return "", errors.New("table size must be a string with 'GB' suffix")
	}

	tableSizeGB, err := strconv.Atoi(strings.TrimSuffix(tableSize, "GB"))
	if err != nil {
		return "", errors.Wrap(err, "parse table size")
	}

	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 获取表的大小并检查是否超过阈值
	checkTableSizeViolation := func(table *parser.RangeVar) (bool, error) {
		if table == nil {
			return false, nil
		}

		tableType, err := utilGetTableType(ctx, table.Schemaname, table.Relname)
		if err != nil {
			return false, errors.Wrap(err, "get table type")
		}

		// 获取分区表的所有分区子表
		subTables, err := utilGetSubTables(ctx, tableType, table.Schemaname, table.Relname)
		if err != nil {
			return false, errors.Wrap(err, "get sub tables")
		}

		if tableType == executor.RelationTableType { // 集中式下的表
			var tableNameList []string
			for _, subTable := range subTables {
				tableNameList = append(tableNameList, subTable.TableName)
			}
			// 获取表的大小
			tableSizeList, err := utilGetTableSizeGBBySingleNode(ctx, table.Schemaname, tableNameList)
			if err != nil {
				return false, errors.Wrap(err, "get table size")
			}

			for _, tableSize := range tableSizeList {
				if tableSize.Size >= tableSizeGB {
					return true, nil // 违反规则
				}
			}
		} else {
			plan, err := utilGetExecutionPlan(ctx, sql)
			if err != nil {
				return false, errors.Wrap(err, "get execution plan text")
			}
			// 遍历执行计划
			var nodeList []string
			utilVisitPlan(plan, func(planNode *PlanType) bool {
				if planNode.Nodes != "" {
					nodeList = strings.Split(planNode.Nodes, ",")
					return true
				}
				if len(planNode.NodeList) > 0 {
					nodeList = planNode.NodeList
					return true
				}
				return false
			})

			for _, dataNode := range nodeList {
				var tableNameList []string
				for _, subTable := range subTables {
					tableNameList = append(tableNameList, subTable.TableName)
				}
				// 获取表的大小
				tableSizeList, err := utilGetTableSizeGBByNode(ctx, dataNode, table.Schemaname, tableNameList)
				if err != nil {
					return false, errors.Wrap(err, "get table size")
				}

				for _, tableSize := range tableSizeList {
					if tableSize.Size >= tableSizeGB {
						return true, nil // 违反规则
					}
				}
			}
		}
		return false, nil
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_InsertStmt:
		// "INSERT INTO ..." 语句
		if violation, err := checkTableSizeViolation(stmt.InsertStmt.Relation); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_UpdateStmt:
		// "UPDATE ..." 语句
		if violation, err := checkTableSizeViolation(stmt.UpdateStmt.Relation); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CopyStmt:
		// "COPY ..." 语句
		if violation, err := checkTableSizeViolation(stmt.CopyStmt.Relation); err != nil {
			return "", err
		} else if violation {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// ==== Rule code end ====