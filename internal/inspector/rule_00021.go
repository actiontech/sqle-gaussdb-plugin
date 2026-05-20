package inspector

import (
	"context"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00021 = "SQLE00021"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00021,
			Desc:       "禁止表字段缺少NOT NULL约束",
			Annotation: "若数据库表字段缺少NOT NULL约束，则字段存储值有能是NULL，后期判断时，需要加上IS NULL判断，增加SQL 编写的复杂度。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDDLConvention,
			Params:     nil,
		},
		Message:              "禁止表字段缺少NOT NULL约束",
		RawSQLHandler:        RuleSQLE00021,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00021): "在 TBase 中，禁止表字段缺少NOT NULL约束."
您应遵循以下逻辑：
1. 对于“CREATE TABLE ... ”语句, 存在一个字段的定义是不包含 NOT NULL约束时，则报告违反规则：
2. 对于“ALTER TABLE ... ADD ...”语句，执行与上面类似的检查。
3. 对于“ALTER TABLE ... ALTER ... ”语句,修改的目标是column时，若"DROP NOT NULL" 子句，则报告违反规则。
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00021(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 记录违反规则的列名
	violateColumnNames := []string{}

	// 检查 CREATE TABLE 语句
	switch stmt := node.GetStmt().Node.(type) {
	case *parser.Node_CreateStmt:
		for _, elt := range stmt.CreateStmt.TableElts {
			// 如果列没有 NOT NULL 约束，则记录列名
			if !utilIsColumnHasConstraint(elt, parser.ConstrType_CONSTR_NOTNULL) {
				violateColumnNames = append(violateColumnNames, utilGetColumnName(elt))
			}
		}
	// 检查 ALTER TABLE ADD COLUMN 语句
	case *parser.Node_AlterTableStmt:
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_AddColumn) {
			// 如果新增的列没有 NOT NULL 约束，则记录列名
			if !utilIsColumnHasConstraint(cmd.Def, parser.ConstrType_CONSTR_NOTNULL) {
				violateColumnNames = append(violateColumnNames, utilGetColumnName(cmd.Def))
			}
		}
		// 检查 ALTER TABLE DROP NOT NULL 语句
		for _, cmd := range utilGetAlterTableCommandsByTypes(stmt.AlterTableStmt, parser.AlterTableType_AT_DropNotNull) {
			// 如果修改的列包含 SET NOT NULL 子句，则记录列名
			violateColumnNames = append(violateColumnNames, cmd.Name)
		}
	}

	// 如果存在违反规则的列名，返回规则消息
	if len(violateColumnNames) > 0 {
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
