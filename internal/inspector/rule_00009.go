package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00009 = "SQLE00009"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00009,
			Desc:       "避免对条件字段使用函数操作",
			Annotation: "对条件字段做函数操作，可能会破坏索引值的有序性，导致优化器选择放弃走索引，使查询性能大幅度降低",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "避免对条件字段使用函数操作",
		RawSQLHandler:        RuleSQLE00009,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00009): "在 TBase 中，避免对条件字段使用函数操作."
您应遵循以下逻辑：
1. 对于"SELECT..."语句，检查SQL语句，如果存在 WHERE 关键词以及WHERE条件中存在字段是否有函数计算，若有，判断是否存在该函数索引，若没有，则报告违反规则。其中函数索引信息是在线获取的信息。
2. 对于"INSERT...SELECT..."语句，执行与上面类似的检查。
3. 对于"UPDATE..."语句，执行与上面类似的检查。
4. 对于"DELETE..."语句，执行与上面类似的检查。
5. 对于"... UNION ALL ..."语句，执行与上面类似的检查。
6. 对于"WITH..."语句，执行与上面类似的检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00009(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查字段集合是否包含函数调用且没有对应的函数索引
	checkFunctionViolation := func(tables []*parser.RangeVar, whereClause *parser.Node) (bool, error) {
		funcCallToCheck := []*parser.FuncCall{}
		utilScanWhereStmt(func(expr *parser.Node) (skip bool) {
			switch n := expr.GetNode().(type) {
			case *parser.Node_AExpr:
				if n.AExpr != nil {
					if n.AExpr.Lexpr != nil {
						if f, ok := n.AExpr.Lexpr.GetNode().(*parser.Node_FuncCall); ok {
							funcCallToCheck = append(funcCallToCheck, f.FuncCall)
						}
					}
					if n.AExpr.Rexpr != nil {
						if f, ok := n.AExpr.Rexpr.GetNode().(*parser.Node_FuncCall); ok {
							funcCallToCheck = append(funcCallToCheck, f.FuncCall)
						}
					}
				}
			case *parser.Node_BoolExpr:
				return false // 继续递归
			}
			return false
		}, whereClause)

		for _, table := range tables {
			// 获取表的索引信息
			indexInfo, err := utilGetTableIndexInfo(ctx, table.Schemaname, table.Relname)
			if err != nil {
				return false, errors.Wrap(err, "get table index info")
			}

			// 检查每个字段是否有索引
			for _, f := range funcCallToCheck {
				if !utilIsFuncCallIndexed(f, indexInfo) {
					// 报告违反规则
					return true, nil
				}
			}
		}
		return false, nil
	}

	// 获取 SELECT 语句中的表
	getSelectTables := func(stmt *parser.SelectStmt) []*parser.RangeVar {
		rangeVars := []*parser.RangeVar{}
		for _, from := range stmt.FromClause {
			for _, rangeVar := range utilGetRangeVars(from) {
				rangeVars = append(rangeVars, rangeVar)
			}
		}
		return rangeVars
	}

	// 主逻辑：根据不同的 DML 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt: // Node_SelectStmt 已经包含了 Union语句
		for _, selectStmt := range utilGetSelectStmt(node.GetStmt()) {
			if selectStmt.WhereClause != nil {
				if violation, err := checkFunctionViolation(getSelectTables(selectStmt), selectStmt.WhereClause); err != nil {
					return "", err
				} else if violation {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	case *parser.Node_InsertStmt:
		insertStmt := stmt.InsertStmt
		if insertStmt.SelectStmt != nil {
			for _, selectStmt := range utilGetSelectStmt(insertStmt.SelectStmt) {
				if selectStmt.WhereClause != nil {
					if violation, err := checkFunctionViolation(getSelectTables(selectStmt), selectStmt.WhereClause); err != nil {
						return "", err
					} else if violation {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}
		}
	case *parser.Node_UpdateStmt:
		updateStmt := stmt.UpdateStmt
		if updateStmt.WhereClause != nil {
			if violation, err := checkFunctionViolation([]*parser.RangeVar{updateStmt.Relation}, updateStmt.WhereClause); err != nil {
				return "", err
			} else if violation {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_DeleteStmt:
		deleteStmt := stmt.DeleteStmt
		if deleteStmt.WhereClause != nil {
			if violation, err := checkFunctionViolation([]*parser.RangeVar{deleteStmt.Relation}, deleteStmt.WhereClause); err != nil {
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
