package inspector

import (
	"context"
	"strings"
	"unicode"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pkg/errors"
)

const (
	SQLE00224 = "SQLE00224"
)

func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLE00224,
			Desc:       "对象名禁止以pg、pgxc或sys、__(双下划线)或数字开头。",
			Annotation: "对象名若以pg、pgxc或sys、__(双下划线)开头，容易与数据库自身的系统表、内置表混为一谈，难以区分业务表的归属，增加用户的管理、维护成本，增加代码的复杂度、可读性、兼容性等。",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeNamingConvention,
			Params:     nil,
		},
		Message:              "对象名禁止以pg、pgxc或sys、__(双下划线)或数字开头。",
		RawSQLHandler:        RuleSQLE00224,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

/*
==== Prompt start ====
在 TBase(PostgreSQL的分布式版本) 中，您应该检查 SQL 是否违反了规则(SQLE00224): "在 TBase 中，对象名禁止以pg、pgxc或sys、__(双下划线)或数字开头。."
您应遵循以下逻辑：
1. 对于“CREATE ...” 语句，如果下面任意一项命名的前缀为pg、pgxc、sys、__(双下划线)、数字 中的任意一项，则报告违反规则：
  1. CREATE TABLE 后 表名前缀
  2. CREATE VIEW 后 视图名前缀
  3. CREATE SEQUENCE 后 序列名前缀
  4. CREATE INDEX 后 索引名前缀
  5. CREATE RULE 后 规则前缀
  6. CREATE OR REPLACE TRIGGER 后 触发器名前缀
  7. CREATE OR REPLACE FUNCTION 后，函数名前缀
  8. CREATE OR REPLACE PROCEDURE 后，存储过程名前缀
  9. CREATE USER 后 用户名前缀
  10. CREATE ROLE 后 角色名前缀
  11. CREATE TYPE 后 自定义类型名前缀
  12. CREATE SCHEMA 后 模式名前缀
  13. CREATE DATABASE 后 数据库名前缀
  14. 其他所有CREATE 语句 后，命名前缀
2. 对于“ALTER  ...” 语句，如果下面任意一项命名的前缀为pg、pgxc、sys、__(双下划线)、数字 中的任意一项，则报告违反规则：
  1. ALTER TABLE ... RENAME TO 后 表名前缀
  2. ALTER TABLE ... ADD 后 字段名前缀
  3. ALTER VIEW ... RENAME TO 后 视图名前缀
  4. ALTER SEQUENCE ... RENAME TO 后 序列名前缀
  5. ALTER INDEX ... RENAME TO 后 索引名前缀
  6. ALTER RULE ... RENAME TO 后 规则前缀
  7. ALTER TRIGGER ... RENAME TO 后 触发器名前缀
  8. ALTER FUNCTION ... RENAME TO 后，函数名前缀
  9. ALTER PROCEDURE 后，存储过程名前缀
  10. ALTER USER ... RENAME TO 后 用户名前缀
  11. ALTER ROLE ... RENAME TO 后 角色名前缀
  12. CREATE TYPE ... RENAME TO 后 自定义类型名前缀
  13. CREATE TYPE ... RENAME ... ATTRIBUTE  TO 后 自定义类型名前缀
  14. CREATE TYPE ... RENAME ... VALUE  TO 后 自定义类型名前缀
  15. ALTER  SCHEMA ... RENAME TO 后 模式名前缀
  16. ALTER  DATABASE ... RENAME TO 后 数据库名前缀
  17. 其他所有 ALTER 语句 RENAME TO 后，命名前缀
==== Prompt end ====
*/

// ==== Rule code start ====
// 规则函数实现开始
func RuleSQLE00224(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	// 解析 SQL 语句
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	// 检查对象名是否违反规则
	checkObjectNameViolation := func(objectName string) bool {
		if objectName == "" {
			return false
		}
		if strings.HasPrefix(objectName, "pg") || strings.HasPrefix(objectName, "pgxc") || strings.HasPrefix(objectName, "sys") ||
			strings.HasPrefix(objectName, "__") || unicode.IsDigit(rune(objectName[0])) {
			return true
		}
		return false
	}

	// 获取对象名并检查是否违反规则
	checkCreateStmtViolation := func(stmt *parser.Node_CreateStmt) bool {
		if stmt.CreateStmt.Relation != nil && checkObjectNameViolation(stmt.CreateStmt.Relation.Relname) {
			return true
		}
		for _, elt := range stmt.CreateStmt.TableElts {
			if checkObjectNameViolation(utilGetColumnName(elt)) {
				return true
			}
		}
		return false
	}

	checkAlterStmtViolation := func(stmt *parser.Node_AlterTableStmt) bool {
		for _, cmd := range stmt.AlterTableStmt.Cmds {
			switch alterCmd := cmd.GetNode().(type) {
			case *parser.Node_AlterTableCmd:
				if alterCmd.AlterTableCmd.Name != "" && checkObjectNameViolation(alterCmd.AlterTableCmd.Name) {
					return true
				}
			}
		}
		return false
	}

	// 主逻辑：根据不同的 SQL 类型执行相应的检查
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		if checkCreateStmtViolation(stmt) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CompositeTypeStmt:
		if stmt.CompositeTypeStmt.Typevar != nil && checkObjectNameViolation(stmt.CompositeTypeStmt.Typevar.Relname) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateSeqStmt:
		if stmt.CreateSeqStmt.Sequence != nil && checkObjectNameViolation(stmt.CreateSeqStmt.Sequence.Relname) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_IndexStmt:
		if stmt.IndexStmt.GetIdxname() != "" && checkObjectNameViolation(stmt.IndexStmt.GetIdxname()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreatedbStmt: //create database
		//CREATE DATABASE [dbname];
		if stmt.CreatedbStmt.GetDbname() != "" && checkObjectNameViolation(stmt.CreatedbStmt.GetDbname()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateTableAsStmt: //create tabel as
		//CREATE TABLE [tablename] AS SELECT ...;
		if stmt.CreateTableAsStmt.GetInto().GetRel().GetRelname() != "" && checkObjectNameViolation(stmt.CreateTableAsStmt.GetInto().GetRel().GetRelname()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateTrigStmt:
		if stmt.CreateTrigStmt.Trigname != "" && checkObjectNameViolation(stmt.CreateTrigStmt.Trigname) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateUserMappingStmt:
		if stmt.CreateUserMappingStmt.User != nil && checkObjectNameViolation(stmt.CreateUserMappingStmt.User.GetRolename()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateRoleStmt:
		if stmt.CreateRoleStmt.Role != "" && checkObjectNameViolation(stmt.CreateRoleStmt.Role) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateSchemaStmt:
		if stmt.CreateSchemaStmt.Schemaname != "" && checkObjectNameViolation(stmt.CreateSchemaStmt.Schemaname) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterTableStmt:
		if checkAlterStmtViolation(stmt) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterUserMappingStmt:

	case *parser.Node_AlterDatabaseStmt:
		if stmt.AlterDatabaseStmt.Dbname != "" && checkObjectNameViolation(stmt.AlterDatabaseStmt.Dbname) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterRoleStmt:
		if stmt.AlterRoleStmt.Role != nil && checkObjectNameViolation(stmt.AlterRoleStmt.Role.GetRolename()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_RenameStmt: // rename
		renameStmt := stmt.RenameStmt
		if renameStmt.GetNewname() != "" && checkObjectNameViolation(renameStmt.GetNewname()) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// 规则函数实现结束
// ==== Rule code end ====
