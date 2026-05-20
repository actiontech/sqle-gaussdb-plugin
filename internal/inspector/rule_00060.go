package inspector

import (
	"context"
	"fmt"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00060 = "SQLE00060"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00060,
			Desc:       "表建议添加注释",
			Annotation: "表添加注释能够使表的意义更明确，方便日后的维护",
			Level:      driverV2.RuleLevelNotice,
			Category:   RuleTypeSuggestion,
			Params:     nil,
		},
		Message:              "表建议添加注释",
		RawSQLHandler:        RuleSQLE00060,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00060): "在 TBase 中，表建议添加注释."
您应遵循以下逻辑：
当前规则需通过上下文检查。
当前规则需通过上下文检查。
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00060(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	var schemaTableName [2]string
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		schemaTableName[0] = stmt.CreateStmt.Relation.Schemaname
		schemaTableName[1] = stmt.CreateStmt.Relation.Relname
	default:
		return "", nil
	}

	commentNames := [][2]string{}
	for _, sql := range nextSQL {
		node, err := sqlParserFuncV2(sql)
		if err != nil {
			return "", fmt.Errorf("parse next sql: %v", err)
		}

		commentStmt, ok := node.GetStmt().GetNode().(*parser.Node_CommentStmt)
		if !ok {
			continue
		}

		commentNames = append(commentNames, utilGetCommentStmtTable(commentStmt))
	}

	for _, comment := range commentNames {
		if schemaTableName == comment {
			return "", nil
		}
	}

	return fmt.Sprintf(RuleHandlerMap[rule.Name].Message), nil
}

// ==== Rule code end ====
