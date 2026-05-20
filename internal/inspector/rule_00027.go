package inspector

import (
	"context"
	"fmt"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00027 = "SQLE00027"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00027,
			Desc:       "列建议添加注释",
			Annotation: "列添加注释能够使列的意义更明确，方便日后的维护",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "列建议添加注释. 违反规则的列: %s",
		RawSQLHandler:        RuleSQLE00027,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00027): "在 TBase 中，列建议添加注释."
您应遵循以下逻辑：
当前规则需通过上下文检查。
当前规则需通过上下文检查。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00027(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	violateColumnNames := []string{}
	colNames := [][3]string{}

	// 检查 CREATE 语句和 ALTER 语句中的列定义
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		for _, elt := range stmt.CreateStmt.TableElts {
			if colName := utilGetColumnName(elt); colName != "" {
				colNames = append(colNames, [3]string{stmt.CreateStmt.Relation.Schemaname, stmt.CreateStmt.Relation.Relname, colName})
			}
		}
	case *parser.Node_AlterTableStmt:
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddColumn) {
			if colName := utilGetColumnName(cmd.Def); colName != "" {
				colNames = append(colNames, [3]string{stmt.AlterTableStmt.Relation.Schemaname, stmt.AlterTableStmt.Relation.Relname, colName})
			}
		}
	}

	if len(colNames) <= 0 {
		return "", nil
	}

	commentNames := [][3]string{}
	for _, sql := range nextSQL {
		node, err := sqlParserFuncV2(sql)
		if err != nil {
			return "", fmt.Errorf("parse next sql: %v", err)
		}

		commentStmt, ok := node.GetStmt().GetNode().(*parser.Node_CommentStmt)
		if !ok {
			continue
		}

		commentNames = append(commentNames, utilGetCommentStmtColumn(commentStmt))
	}

Next:
	for _, c := range colNames {
		for _, comment := range commentNames {
			if c == comment {
				continue Next
			}
		}
		violateColumnNames = append(violateColumnNames, c[2])
	}

	if len(violateColumnNames) > 0 {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(violateColumnNames, ",")), nil
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
