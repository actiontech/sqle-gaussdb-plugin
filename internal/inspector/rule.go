package inspector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle-pg-plugin/internal/utils"
	pkgParser "github.com/actiontech/sqle-pg-plugin/pkg/parser"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

const (
	// rule type
	RuleTypeDMLConvention     = "DML规范"
	RuleTypeDDLConvention     = "DDL规范"
	RuleTypeDQLConvention     = "DQL规范"
	RuleTypeSuggestion        = "使用建议"
	RuleTypeNamingConvention  = "命名规范"
	RuleTypeGlobalConfig      = "全局配置"
	RuleTypeIndexOptimization = "索引优化"
	RuleTypeIndexConvention   = "索引规范"

	// rule name
	RuleId1  = "pg_001"
	RuleId2  = "pg_002"
	RuleId3  = "pg_003"
	RuleId4  = "pg_004"
	RuleId5  = "pg_005"
	RuleId6  = "pg_006"
	RuleId7  = "pg_007"
	RuleId8  = "pg_008"
	RuleId9  = "pg_009"
	RuleId10 = "pg_010"
	RuleId11 = "pg_011"
	RuleId12 = "pg_012"
	RuleId15 = "pg_015"
	RuleId16 = "pg_016"
	RuleId17 = "pg_017"
	RuleId19 = "pg_019"
	RuleId21 = "pg_021"
	RuleId22 = "pg_022"
	RuleId23 = "pg_023"
	RuleId24 = "pg_024"
	RuleId25 = "pg_025"
	RuleId26 = "pg_026"
	RuleId27 = "pg_027"
	RuleId28 = "pg_028"
	RuleId31 = "pg_031"
	RuleId32 = "pg_032"
	RuleId33 = "pg_033"
	RuleId34 = "pg_034"
	RuleId35 = "pg_035"
	RuleId36 = "pg_036"
	RuleId37 = "pg_037"
	RuleId38 = "pg_038"
	RuleId39 = "pg_039"
	RuleId40 = "pg_040"
	RuleId41 = "pg_041"
	RuleId42 = "pg_042"
	RuleId43 = "pg_043"
	RuleId44 = "pg_044"
	RuleId45 = "pg_045"
	RuleId47 = "pg_047"
	RuleId48 = "pg_048"
	RuleId49 = "pg_049"
	RuleId50 = "pg_050"
	RuleId51 = "pg_051"
	RuleId52 = "pg_052"
	RuleId53 = "pg_053"
	RuleId54 = "pg_054"
	RuleId55 = "pg_055"
	RuleId56 = "pg_056"
	RuleId57 = "pg_057"
	RuleId59 = "pg_059"
	RuleId60 = "pg_060"
	RuleId61 = "pg_061"
	RuleId62 = "pg_062"
	RuleId63 = "pg_063"
	RuleId64 = "pg_064"
	RuleId65 = "pg_065"
	RuleId66 = "pg_066"
	RuleId67 = "pg_067"
	RuleId68 = "pg_068"
	RuleId69 = "pg_069"
	RuleId70 = "pg_070"
	RuleId71 = "pg_071"
	RuleId72 = "pg_072"
	RuleId73 = "pg_073"
	RuleId74 = "pg_074"
	RuleId75 = "pg_075"
	RuleId76 = "pg_076"
	RuleId77 = "pg_077"
	RuleId78 = "pg_078"
	RuleId79 = "pg_079"
)

type RuleHandler struct {
	Rule                 driverV2.Rule
	Message              string
	AstSQLHandler        func(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error)
	RawSQLHandler        func(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error)
	AllowOffline         bool
	NotAllowOfflineStmts []interface{}
}

// IsAllowOfflineRule 允许下线规则判定，返回false或true
func (rh *RuleHandler) IsAllowOfflineRule(node *parser.RawStmt) bool {
	if !rh.AllowOffline {
		return false
	}
	// 遍历切片，查看是否存在
	for _, stmt := range rh.NotAllowOfflineStmts {
		if reflect.TypeOf(stmt).Kind() == reflect.TypeOf(node.GetStmt().GetNode()).Kind() {
			return false
		}
	}
	return true
}

var RuleHandlerMap = map[string]RuleHandler{}

func init() {
	for _, rh := range RuleHandlers {
		RuleHandlerMap[rh.Rule.Name] = rh
	}
}

func rule1(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		// check select all column
		for _, target := range stmt.SelectStmt.GetTargetList() {
			column, ok := target.GetResTarget().GetVal().GetNode().(*parser.Node_ColumnRef)
			if !ok {
				continue
			}
			for _, filed := range column.ColumnRef.GetFields() {
				_, ok = filed.GetNode().(*parser.Node_AStar)
				if ok {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	}
	return "", nil
}

func rule2(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_UpdateStmt:
		if stmt.UpdateStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DeleteStmt:
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}

	}
	return "", nil
}

func rule3(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		if stmt.SelectStmt.HavingClause != nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule4(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_DropStmt:
		if stmt.DropStmt.RemoveType != parser.ObjectType_OBJECT_INDEX {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DropdbStmt, *parser.Node_DropRoleStmt, *parser.Node_DropTableSpaceStmt,
		*parser.Node_DropOwnedStmt, *parser.Node_DropSubscriptionStmt, *parser.Node_DropUserMappingStmt:
		return RuleHandlerMap[rule.Name].Message, nil
	default:
	}
	return "", nil
}

func rule5(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_ViewStmt:
		return RuleHandlerMap[rule.Name].Message, nil
	}
	return "", nil
}

func rule6(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateTrigStmt:
		return RuleHandlerMap[rule.Name].Message, nil
	}
	return "", nil
}

func rule7(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		hasPK := false
		for _, elt := range stmt.CreateStmt.TableElts {
			switch def := elt.GetNode().(type) {
			case *parser.Node_ColumnDef:
				for _, constraint := range def.ColumnDef.Constraints {
					//constraint.GetNode().
					c := constraint.GetConstraint()
					if c != nil && c.Contype == parser.ConstrType_CONSTR_PRIMARY {
						hasPK = true
						break
					}
				}
			case *parser.Node_Constraint:
				if def.Constraint.Contype == parser.ConstrType_CONSTR_PRIMARY {
					hasPK = true
					break
				}
			}
		}
		if len(stmt.CreateStmt.TableElts) > 0 && !hasPK {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule8(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		hasFK := false
		for _, elt := range stmt.CreateStmt.TableElts {
			switch def := elt.GetNode().(type) {
			case *parser.Node_ColumnDef:
				for _, constraint := range def.ColumnDef.Constraints {
					//constraint.GetNode().
					c := constraint.GetConstraint()
					if c != nil && c.Contype == parser.ConstrType_CONSTR_FOREIGN {
						hasFK = true
						break
					}
				}
			case *parser.Node_Constraint:
				if def.Constraint.Contype == parser.ConstrType_CONSTR_FOREIGN {
					hasFK = true
					break
				}
			}
		}
		if len(stmt.CreateStmt.TableElts) > 0 && hasFK {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule9(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		columnCounter := 0
		for _, elt := range stmt.CreateStmt.TableElts {
			switch elt.GetNode().(type) {
			case *parser.Node_ColumnDef:
				columnCounter++
			}
		}
		count := rule.Params.GetParam("max_column_count").Int()
		if count > 0 && columnCounter > count {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, count, columnCounter), nil
		}
	}
	return "", nil
}

func rule10(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	maxSqlLen := rule.Params.GetParam("sql_length").Int()
	sqlLen := len(sql)
	if maxSqlLen > 0 && sqlLen > maxSqlLen {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxSqlLen), nil
	}
	return "", nil
}

func max(num1, num2 int) int {
	var result int
	if num1 > num2 {
		result = num1
	} else {
		result = num2
	}
	return result
}

func rule11(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	maxCompositeIndexColumnNumber := 0
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_AlterTableStmt:
		// alter table
		for _, constraint := range stmt.AlterTableStmt.GetCmds() {
			switch constraint.GetAlterTableCmd().GetSubtype() {
			case parser.AlterTableType_AT_AddColumn:
				// alter table add column
				currentCompositeIndexColumnNumber := len(constraint.GetAlterTableCmd().Def.GetColumnDef().TypeName.Typmods)
				maxCompositeIndexColumnNumber = max(currentCompositeIndexColumnNumber, maxCompositeIndexColumnNumber)
			case parser.AlterTableType_AT_AddConstraint:
				// alter table add constraint
				typeConstrType := constraint.GetAlterTableCmd().Def.GetConstraint().GetContype()
				if typeConstrType == parser.ConstrType_CONSTR_UNIQUE || typeConstrType == parser.ConstrType_CONSTR_PRIMARY {
					// unique and primary
					currentCompositeIndexColumnNumber := len(constraint.GetAlterTableCmd().Def.GetConstraint().Keys)
					maxCompositeIndexColumnNumber = max(currentCompositeIndexColumnNumber, maxCompositeIndexColumnNumber)
				}
			}
		}
	case *parser.Node_CreateStmt:
		// create table
		for _, constraint := range stmt.CreateStmt.TableElts {
			if constraint.GetColumnDef() != nil && constraint.GetColumnDef().GetColname() == "key" {
				// create table add index
				currentCompositeIndexColumnNumber := len(constraint.GetColumnDef().TypeName.Typmods)
				maxCompositeIndexColumnNumber = max(currentCompositeIndexColumnNumber, maxCompositeIndexColumnNumber)
			} else if constraint.GetConstraint() != nil {
				// create table add constraint
				typeConstrType := constraint.GetConstraint().GetContype()
				if typeConstrType == parser.ConstrType_CONSTR_UNIQUE || typeConstrType == parser.ConstrType_CONSTR_PRIMARY {
					// unique and primary
					currentCompositeIndexColumnNumber := len(constraint.GetConstraint().Keys)
					maxCompositeIndexColumnNumber = max(currentCompositeIndexColumnNumber, maxCompositeIndexColumnNumber)
				}
			}
		}
	case *parser.Node_IndexStmt:
		// create index
		currentCompositeIndexColumnNumber := len(stmt.IndexStmt.IndexParams)
		maxCompositeIndexColumnNumber = max(currentCompositeIndexColumnNumber, maxCompositeIndexColumnNumber)
	}

	expectCounter := rule.Params.GetParam("expect_max_index_column_number").Int()
	if maxCompositeIndexColumnNumber > expectCounter {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectCounter), nil
	}
	return "", nil
}

func rule12(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	indexNames := []string{}
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// create table
		for _, constraint := range stmt.CreateStmt.TableElts {
			if constraint.GetColumnDef() != nil && constraint.GetColumnDef().GetColname() == "key" {
				// create table add index
				for _, getStrName := range constraint.GetColumnDef().TypeName.GetNames() {
					indexNames = append(indexNames, getStrName.GetString_().GetSval())
				}
			}
		}
	case *parser.Node_AlterTableStmt:
		// alter table
		for _, constraint := range stmt.AlterTableStmt.GetCmds() {
			switch constraint.GetAlterTableCmd().GetSubtype() {
			case parser.AlterTableType_AT_AddColumn:
				// alter table add column
				alterColumnDef := constraint.GetAlterTableCmd().Def.GetColumnDef()
				if alterColumnDef != nil && alterColumnDef.Colname == "index" {
					// alter table add key
					for _, getStrName := range alterColumnDef.TypeName.GetNames() {
						indexNames = append(indexNames, getStrName.GetString_().GetSval())
					}
				}
			}
		}
	case *parser.Node_IndexStmt:
		// create index
		if !stmt.IndexStmt.Unique {
			// exclude unique
			indexNames = append(indexNames, stmt.IndexStmt.GetIdxname())
		}
	}
	prefix := rule.Params.GetParam("expect_index_name_prefix").String()
	for _, name := range indexNames {
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, prefix), nil
		}
	}
	return "", nil
}

func inKeywords(word string) bool {
	for _, keyword := range Keywords {
		if word == keyword {
			return true
		}
	}
	return false
}

func rule15(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var name []string
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt: //nomal create table
		//CREATE TABLE [tablename] ( colname ...,KEY keyname ...,CONSTRAINT constraintname ...);
		name = append(name, stmt.CreateStmt.Relation.GetRelname())
		for _, columnDef := range stmt.CreateStmt.GetTableElts() {
			if columnDef.GetColumnDef().GetColname() != "" {
				if columnDef.GetColumnDef().GetColname() == "key" {
					//CREATE TABLE tablename( colname ...,KEY [keyname] ...,CONSTRAINT constraintname ...);
					for _, keyName := range columnDef.GetColumnDef().GetTypeName().GetNames() {
						name = append(name, keyName.GetString_().GetSval())
					}
				} else {
					//CREATE TABLE tablename( [colname] ...,KEY keyname ...,CONSTRAINT constraintname ...);
					name = append(name, columnDef.GetColumnDef().GetColname())

				}
			} else if columnDef.GetColumnDef().GetColname() == "" && columnDef.GetConstraint().Conname != "" {
				//CREATE TABLE tablename( colname ...,KEY keyname ...,CONSTRAINT [constraintname] ...);
				name = append(name, columnDef.GetConstraint().Conname)

			}
		}
	case *parser.Node_IndexStmt: //create index
		// CREATE INDEX [indexname] ON tablename (columnname,...);
		name = append(name, stmt.IndexStmt.GetIdxname())
	case *parser.Node_CreatedbStmt: //create database
		//CREATE DATABASE [dbname];
		name = append(name, stmt.CreatedbStmt.GetDbname())
	case *parser.Node_CreateTableAsStmt: //create tabel as
		//CREATE TABLE [tablename] AS SELECT ...;
		name = append(name, stmt.CreateTableAsStmt.GetInto().GetRel().GetRelname())
	case *parser.Node_CreateSchemaStmt: //create schema
		//CREATE SCHEMA [schemaname];
		name = append(name, stmt.CreateSchemaStmt.GetSchemaname())
	case *parser.Node_AlterTableStmt: //alter table
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			alterTableCmd := cmd.GetAlterTableCmd()
			if alterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddColumn {
				columnDef := alterTableCmd.GetDef().GetColumnDef()
				if columnDef.GetColname() == "index" {
					//ALTER TABLE tablename ADD INDEX [indexname] ...;
					for _, indexNameString := range columnDef.GetTypeName().GetNames() {
						name = append(name, indexNameString.GetString_().GetSval())
					}
				} else {
					//ALTER TABLE tablename ADD [columnname] ...;
					name = append(name, columnDef.GetColname())
				}
			} else if alterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
				//ALTER TABLE tablename ADD constraint [constraintname] ...;
				name = append(name, alterTableCmd.GetDef().GetConstraint().GetConname())
			}
		}
	case *parser.Node_RenameStmt: // rename
		//ALTER TABLE tablename RENAME TO [newtablename]; || ALTER TABLE tablename RENAME column columnname TO [newcolumnname];
		renameStmt := stmt.RenameStmt
		if renameStmt.GetRenameType() == parser.ObjectType_OBJECT_TABLE ||
			renameStmt.GetRenameType() == parser.ObjectType_OBJECT_COLUMN {
			name = append(name, renameStmt.GetNewname())
		}
	}
	if len(name) > 0 {
		var illegalWords []string
		for _, dbObjectName := range name {
			if inKeywords(strings.ToLower(dbObjectName)) {
				illegalWords = append(illegalWords, dbObjectName)
			}
		}
		if len(illegalWords) > 0 {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(illegalWords, ", ")), nil
		}
	}
	return "", nil
}

func examineSubClauseLeftFuzzyMatch(subStmt *parser.Node) []string {
	var fuzzyMatchStrings []string
	//where column like '[%value]'
	if aExpr := subStmt.GetAExpr(); aExpr != nil && aExpr.GetKind() == parser.A_Expr_Kind_AEXPR_LIKE {
		constStr := aExpr.GetRexpr().GetAConst().GetSval().GetSval()
		if constStr != "" && constStr[0:1] == "%" {
			fuzzyMatchStrings = append(fuzzyMatchStrings, constStr)
		}
	}
	//where ... ([select ...])
	if subLink := subStmt.GetSubLink(); subLink != nil {
		fuzzyMatchStrings = append(fuzzyMatchStrings, examineSubClauseLeftFuzzyMatch(subLink.GetSubselect())...)
	}
	//where ... ([condition1]) ... ([condition2]) ... ([condition3]) ...
	if boolExpr := subStmt.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.GetArgs() {
			fuzzyMatchStrings = append(fuzzyMatchStrings, examineSubClauseLeftFuzzyMatch(arg)...)
		}
	}

	if selectStmt := subStmt.GetSelectStmt(); selectStmt != nil {
		//Select ... from ... where [...]
		if whereClause := selectStmt.WhereClause; whereClause != nil {
			fuzzyMatchStrings = append(fuzzyMatchStrings, examineSubClauseLeftFuzzyMatch(whereClause)...)
		}
		//Select ... from [...] where ...
		if fromClauses := subStmt.GetSelectStmt().FromClause; fromClauses != nil {
			for _, fromClause := range fromClauses {
				if subquery := fromClause.GetRangeSubselect().GetSubquery(); subquery != nil {
					fuzzyMatchStrings = append(fuzzyMatchStrings, examineSubClauseLeftFuzzyMatch(subquery)...)
				}
			}
		}
		//Select [...] from ... where ...
		if targetList := subStmt.GetSelectStmt().TargetList; targetList != nil {
			for _, target := range targetList {
				if subselect := target.GetResTarget().GetVal().GetSubLink().GetSubselect(); subselect != nil {
					fuzzyMatchStrings = append(fuzzyMatchStrings, examineSubClauseLeftFuzzyMatch(subselect)...)
				}
			}
		}
	}
	return fuzzyMatchStrings
}

func rule16(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt(); stmt.GetNode().(type) {
	case *parser.Node_SelectStmt:
		//Select ...
		allFuzzyMatchStrings := examineSubClauseLeftFuzzyMatch(stmt)
		if allFuzzyMatchStrings != nil {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(allFuzzyMatchStrings, ", ")), nil
		}
	case *parser.Node_UpdateStmt:
		//Update ...
		var allFuzzyMatchStrings []string
		if whereClause := stmt.GetUpdateStmt().GetWhereClause(); whereClause != nil {
			allFuzzyMatchStrings = examineSubClauseLeftFuzzyMatch(whereClause)
		}
		if allFuzzyMatchStrings != nil {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(allFuzzyMatchStrings, ", ")), nil
		}
	case *parser.Node_DeleteStmt:
		//Delete ...
		var allFuzzyMatchStrings []string
		if whereClause := stmt.GetDeleteStmt().GetWhereClause(); whereClause != nil {
			allFuzzyMatchStrings = examineSubClauseLeftFuzzyMatch(whereClause)
		}
		if allFuzzyMatchStrings != nil {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, strings.Join(allFuzzyMatchStrings, ", ")), nil
		}
	}
	return "", nil
}

func checkEpWithRule17(plan PlanType, rule *driverV2.Rule, expectMaxScanRowNumber int) bool {
	switch plan.NodeType {
	case "Seq Scan", "Result":
		if plan.PlanRows > int64(expectMaxScanRowNumber) {
			return false
		}
	}
	return true
}

// getExecutionPlan get Execution Plan.
func executionPlan(e *executor.Executor, sql string) (*[]PlanType, error) {
	queryPlan, err := e.GetExecutionPlan(sql)
	if err != nil {
		return nil, err
	}

	// handle json
	str := strings.Join(queryPlan, "")
	jsonQueryPlans := make([]map[string]PlanType, 1)
	jsonErr := json.Unmarshal([]byte(str), &jsonQueryPlans)
	if jsonErr != nil {
		return nil, jsonErr
	}
	queryPlans := make([]PlanType, 0)
	for _, v := range jsonQueryPlans {
		queryPlans = append(queryPlans, v["Plan"])
	}
	return &queryPlans, nil
}

func getExecutorFromCtx(ctx context.Context) (*executor.Executor, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return nil, fmt.Errorf("cannot get context for rule handler")
	}
	e := c.GetExecutor()
	if e == nil {
		return nil, errNoConnection
	}
	return e, nil
}

func getCurrentSchemaFromCtx(ctx context.Context) (string, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return "", fmt.Errorf("cannot get context for rule handler")
	}

	return c.GetCurrentSchema(), nil
}

func getDatabaseCollateFromCtx(ctx context.Context) (string, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return "", fmt.Errorf("cannot get context for rule handler")
	}

	return c.GetCurrentDatabaseCollation(), nil
}

func getPgContextFromCtx(ctx context.Context) (*PgContext, error) {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return nil, fmt.Errorf("cannot get context for rule handler")
	}
	pgContext := c.GetPgContext()
	if pgContext == nil {
		return nil, errNoInitPgContext
	}
	return pgContext, nil
}

func rule17(ctx context.Context, rule *driverV2.Rule, rawSql string, nextSQL []string) (string, error) {
	// sql from MyBatis XML file is not the executable sql. so can't do explain for it.
	// TODO(@wy) ignore explain when audit Mybatis file
	nodes, err := pkgParser.ParseSQL(rawSql)
	if err != nil {
		return "", err
	}
	if len(nodes) <= 0 {
		return "", fmt.Errorf("can not find parse tree from SQL")
	}

	// filter out IURD
	switch nodes[0].GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
	case *parser.Node_DeleteStmt:
	case *parser.Node_InsertStmt:
	case *parser.Node_UpdateStmt:
	default:
		return "", nil
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	if pgContext.UsingType == UsingTypeOffline {
		return "", nil
	}
	e := pgContext.Executor

	jsonQueryPlans, err := getExplainResult(pgContext, rawSql, e)
	if err != nil {
		return "", err
	}

	expectMaxScanRowNumber := rule.Params.GetParam("expect_number_of_scan_line").Int()
	for _, queryPlan := range *jsonQueryPlans {
		if !recursionCheckEp([]PlanType{queryPlan}, rule, expectMaxScanRowNumber, checkEpWithRule17, true) {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectMaxScanRowNumber), nil
		}
	}
	return "", nil
}

func getExplainResult(pgContext *PgContext, rawSql string, e *executor.Executor) (*[]PlanType, error) {
	var (
		jsonQueryPlans *[]PlanType
		ok             = false
		err            error
	)
	if jsonQueryPlans, ok = pgContext.ExecutionPlanCache[rawSql]; !ok {
		jsonQueryPlans, err = executionPlan(e, rawSql)
		if err != nil {
			// check dml related table or database is created, if not exist, explain will executed failure.
			return nil, err
		}
		pgContext.ExecutionPlanCache[rawSql] = jsonQueryPlans
	}
	return jsonQueryPlans, nil
}

func countSubSelect(fromClauses []*parser.Node) int {
	if len(fromClauses) == 0 {
		return 0
	}
	max := 0
	for _, fromClause := range fromClauses {
		nestingFromClause := fromClause.GetRangeSubselect().GetSubquery().GetSelectStmt().GetFromClause()
		subMax := countSubSelect(nestingFromClause) + 1
		if subMax > max {
			max = subMax
		}
	}
	return max
}

func rule19(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		maxSubSelectInFrom := countSubSelect(stmt.SelectStmt.GetFromClause())
		expectedNestingLayers := rule.Params.GetParam("expected_nesting_layers").Int()
		if maxSubSelectInFrom > expectedNestingLayers {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedNestingLayers, maxSubSelectInFrom), nil
		}

	}
	return "", nil
}

func rule21(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	checkLockingClause := func(sLC *parser.Node) string {
		if sLC != nil && sLC.GetLockingClause().GetStrength() == parser.LockClauseStrength_LCS_FORUPDATE {
			return RuleHandlerMap[rule.Name].Message
		}
		return ""
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		// simple sql
		for _, sLC := range stmt.SelectStmt.LockingClause {
			return checkLockingClause(sLC), nil
		}
		// from clause
		for _, sFC := range stmt.SelectStmt.FromClause { /* If from is null skip check. EX: select version; */
			// common clause
			if sFC.GetRangeSubselect() != nil {
				for _, sGL := range sFC.GetRangeSubselect().Subquery.GetSelectStmt().LockingClause {
					return checkLockingClause(sGL), nil
				}
			}
			// join clause
			if sFC.GetJoinExpr() != nil && sFC.GetJoinExpr().Rarg != nil &&
				sFC.GetJoinExpr().Rarg.GetRangeSubselect() != nil {
				for _, sGL := range sFC.GetJoinExpr().Rarg.GetRangeSubselect().Subquery.GetSelectStmt().LockingClause {
					return checkLockingClause(sGL), nil
				}
			}
		}
		// union clause
		if stmt.SelectStmt.Larg != nil {
			for _, sLF := range stmt.SelectStmt.Larg.FromClause {
				if stmt.SelectStmt.Op == parser.SetOperation_SETOP_UNION && sLF.GetRangeSubselect() != nil {
					for _, sGL := range sLF.GetRangeSubselect().Subquery.GetSelectStmt().LockingClause {
						return checkLockingClause(sGL), nil
					}
				}
			}
		}
		// where clause
		if stmt.SelectStmt.WhereClause.GetSubLink() != nil {
			for _, sGL := range stmt.SelectStmt.WhereClause.GetSubLink().Subselect.GetSelectStmt().LockingClause {
				return checkLockingClause(sGL), nil
			}
		}
		if stmt.SelectStmt.WhereClause.GetBoolExpr() != nil {
			for _, wGA := range stmt.SelectStmt.WhereClause.GetBoolExpr().Args {
				if wGA.GetSubLink() != nil {
					for _, sGL := range wGA.GetSubLink().Subselect.GetSelectStmt().LockingClause {
						return checkLockingClause(sGL), nil
					}
				}
			}
		}
	}
	return "", nil
}

func rule22(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQLs []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	tableName := ""
	specifiedSchemaName := ""
	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_VariableSetStmt:
		// 切换当前schema
		if stmt.VariableSetStmt.GetName() != "search_path" {
			return "", nil
		}
		currentSchemaName, err = setCurrentSchemaFromVariableSetStmt(ctx, stmt)
		if err != nil {
			return "", err
		}
		return "", nil
	case *parser.Node_CreateStmt:
		tableName = stmt.CreateStmt.GetRelation().GetRelname()
		specifiedSchemaName = stmt.CreateStmt.GetRelation().GetSchemaname()

		// todo create table as 不会把源表已有的注释带过来，需要检查
	//case *parser.Node_CreateTableAsStmt:
	//	tableName = stmt.CreateTableAsStmt.GetInto().GetRel().GetRelname()
	//case *parser.Node_CreateForeignTableStmt:
	//	tableName = stmt.CreateForeignTableStmt.GetBaseStmt().GetRelation().GetRelname()
	default:
		return "", nil
	}

	if specifiedSchemaName == "" {
		// 没有显式指定schema，使用默认schema
		specifiedSchemaName = currentSchemaName
	}

	found, err := findCommentForTable(ctx, specifiedSchemaName, currentSchemaName, tableName, nextSQLs)
	if err != nil {
		return "", err
	}
	if !found {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, tableName), nil
	}
	return "", nil
}

func findCommentForTable(ctx context.Context, targetSchemaName, currentSchemaName, targetTableName string, sqls []string) (found bool, err error) {
	//e.g. COMMENT ON TABLE schema.table IS 'test'  // item[0] -- schema  item[1] -- table
	//e.g. COMMENT ON TABLE table IS 'test'  // item[0] -- table
	if len(sqls) <= 0 {
		return false, nil
	}

	for _, sql := range sqls {
		nodes, err := pkgParser.ParseSQL(sql)
		if err != nil {
			return false, fmt.Errorf("parse SQL failed: %v", err)
		}
		if len(nodes) <= 0 {
			continue
		}

		stmt, ok := nodes[0].GetStmt().GetNode().(*parser.Node_VariableSetStmt)
		if ok {
			currentSchemaName, err = setCurrentSchemaFromVariableSetStmt(ctx, stmt)
			continue
		}

		stmtComment, ok := nodes[0].GetStmt().GetNode().(*parser.Node_CommentStmt)
		if !ok {
			continue
		}

		if parser.ObjectType_OBJECT_TABLE != stmtComment.CommentStmt.GetObjtype() {
			continue
		}

		items := stmtComment.CommentStmt.GetObject().GetList().GetItems()
		if len(items) == 2 { // 显式指定schema
			itemNamePt, ok := items[0].GetNode().(*parser.Node_String_)
			if !ok || itemNamePt == nil || itemNamePt.String_ == nil || itemNamePt.String_.GetSval() != targetSchemaName { // 匹配schema名
				continue
			}

			itemNamePt, ok = items[1].GetNode().(*parser.Node_String_)
			if !ok || itemNamePt == nil || itemNamePt.String_ == nil || itemNamePt.String_.GetSval() != targetTableName { // 匹配表名
				continue
			}
			return true, nil
		} else if len(items) == 1 { // 没有显式指定schema，使用当前schema
			itemNamePt, ok := items[0].GetNode().(*parser.Node_String_)
			if !ok || itemNamePt == nil || itemNamePt.String_ == nil || itemNamePt.String_.GetSval() != targetTableName { // 匹配表名
				continue
			}

			// 没有显式指定schema，使用当前schema
			// 如果要匹配的schema不为空，且当前schema为空，说明没有获取过当前schema，需要获取当前schema
			if targetSchemaName != "" && currentSchemaName == "" {
				pgContext, err := getPgContextFromCtx(ctx)
				if err != nil {
					return false, err
				}
				currentSchemaName = pgContext.DatabaseInfo.CurrentSchema
				err = setCurrentSchemaToCtx(ctx, currentSchemaName)
				if err != nil {
					return false, err
				}
			}

			if targetSchemaName != currentSchemaName {
				continue
			}
			return true, nil
		} else {
			continue
		}
	}

	return false, nil
}

func rule23(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQLs []string) (string, error) {
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	tableName := ""
	specifiedSchemaName := ""
	columns := []string{}
	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_VariableSetStmt:
		// 切换当前schema
		if stmt.VariableSetStmt.GetName() != "search_path" {
			return "", nil
		}
		currentSchemaName, err = setCurrentSchemaFromVariableSetStmt(ctx, stmt)
		if err != nil {
			return "", err
		}
		return "", nil
	case *parser.Node_CreateStmt:
		tableName = stmt.CreateStmt.GetRelation().GetRelname()
		specifiedSchemaName = stmt.CreateStmt.GetRelation().GetSchemaname()
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			colName := elt.GetColumnDef().GetColname()
			if colName == "" {
				continue
			}
			columns = append(columns, colName)
		}
	case *parser.Node_AlterTableStmt:
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() != parser.AlterTableType_AT_AddColumn {
				return "", nil
			}
			break
		}
		if cmdNode == nil {
			// 没有找到需要校验规则的语法
			return "", nil
		}
		specifiedSchemaName = stmt.AlterTableStmt.GetRelation().GetSchemaname()
		tableName = stmt.AlterTableStmt.GetRelation().GetRelname()
		colDef := cmdNode.AlterTableCmd.GetDef().GetColumnDef()
		columns = append(columns, colDef.GetColname())
	default:
		return "", nil
	}

	if specifiedSchemaName == "" {
		// 没有显式指定schema，使用默认schema
		specifiedSchemaName = currentSchemaName
	}
	columnsWithoutComment, err := findCommentForColumns(ctx, specifiedSchemaName, currentSchemaName, tableName, columns, nextSQLs)
	if err != nil {
		return "", err
	}
	if len(columnsWithoutComment) > 0 {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, tableName, strings.Join(columnsWithoutComment, ", ")), nil
	}
	return "", nil
}

func rule26(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_AlterTableStmt:
		cmd := stmt.AlterTableStmt.GetCmds()[0]
		if cmd.GetAlterTableCmd().GetSubtype() == parser.AlterTableType_AT_AlterColumnType {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule27(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQLs []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_InsertStmt:
		if stmt.InsertStmt.GetCols() == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule28(ctx context.Context, rule *driverV2.Rule, rawSql string, nextSQL []string) (string, error) {
	nodes, err := pkgParser.ParseSQL(rawSql)
	if err != nil {
		return "", err
	}
	if len(nodes) <= 0 {
		return "", fmt.Errorf("can not find parse tree from SQL")
	}

	// filter out IURD
	switch nodes[0].GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
	case *parser.Node_DeleteStmt:
	case *parser.Node_InsertStmt:
	case *parser.Node_UpdateStmt:
	default:
		return "", nil
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	if pgContext.UsingType == UsingTypeOffline {
		return "", nil
	}
	e := pgContext.Executor

	jsonQueryPlans, err := getExplainResult(pgContext, rawSql, e)
	if err != nil {
		return "", err
	}

	// true:存在笛卡尔积；false:不存在笛卡尔积
	if checkForCartesian(jsonQueryPlans) {
		return RuleHandlerMap[rule.Name].Message, nil
	}
	return "", nil
}

// rule28 method : true:存在笛卡尔积；false:不存在笛卡尔积
func checkForCartesian(queryPlans *[]PlanType) bool {
	for _, queryPlan := range *queryPlans {
		result := checkPlanForCartesian(queryPlan)
		if result {
			return true
		}
	}
	return false
}

// rule28 method
func checkPlanForCartesian(plan PlanType) bool {
	switch plan.NodeType {
	case "Nested Loop":
		return true
	default:
		subPlans := plan.Plans
		if subPlans != nil && len(*subPlans) > 0 {
			// 递归调用检查子计划
			result := checkForCartesian(subPlans)
			if result {
				return true
			}
		}
	}
	return false
}

//goland:noinspection LanguageDetectionInspection
func rule31(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	schemaName := ""
	tableName := ""
	indexColumns := make([]string, 0)
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		tableElts := stmt.CreateStmt.TableElts
		for _, tableElt := range tableElts {
			if tableElt.GetColumnDef() != nil {
				// 获取列定义中的唯一约束
				columnConstraints := tableElt.GetColumnDef().GetConstraints()
				for _, constraint := range columnConstraints {
					if constraint.GetConstraint() == nil {
						continue
					}
					if constraint.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE {
						indexColumns = append(indexColumns, strings.ToLower(tableElt.GetColumnDef().GetColname()))
					}
				}
			}
			// 获取列后面唯一约束
			if tableElt.GetConstraint() != nil &&
				tableElt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE {
				columns := make([]string, 0)
				keys := tableElt.GetConstraint().GetKeys()
				for _, key := range keys {
					columns = append(columns, strings.ToLower(key.GetString_().GetSval()))
				}
				for _, indexColumn := range indexColumns {
					eachIndexColumn := strings.Split(indexColumn, ",")[:]
					for i := 0; i < len(columns) && i < len(eachIndexColumn); i++ {
						if columns[i] == eachIndexColumn[i] {
							return RuleHandlerMap[rule.Name].Message, nil
						}
					}
				}
				// 将不冗余的索引对应的列组方法slice中，为下一步比较使用
				indexColumns = append(indexColumns, strings.Join(columns, ","))
			}
		}
	case *parser.Node_AlterTableStmt:
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			alterTableCmd := cmd.GetAlterTableCmd()
			// alter table test.testIndex add index idx_testIndex2(column1, column2);
			if alterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddColumn {
				columnDef := alterTableCmd.GetDef().GetColumnDef()
				if columnDef.GetColname() == "index" {
					typMods := columnDef.GetTypeName().GetTypmods()
					for _, typMod := range typMods {
						fields := typMod.GetColumnRef().GetFields()
						for _, field := range fields {
							indexColumns = append(indexColumns, field.GetString_().GetSval())
						}
					}
				}
				continue
			}
			// alter table test.testIndex add constraint idx_testIndex3 unique (column1, column2);
			if alterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
				if alterTableCmd.Def != nil && alterTableCmd.Def.GetConstraint() != nil &&
					alterTableCmd.Def.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE {
					keys := alterTableCmd.Def.GetConstraint().GetKeys()
					for _, key := range keys {
						indexColumns = append(indexColumns, key.GetString_().GetSval())
					}
				}
			}
		}
		schemaName = stmt.AlterTableStmt.GetRelation().GetSchemaname()
		tableName = stmt.AlterTableStmt.GetRelation().GetRelname()
	case *parser.Node_IndexStmt:
		schemaName = stmt.IndexStmt.GetRelation().GetSchemaname()
		tableName = stmt.IndexStmt.GetRelation().GetRelname()
		indexParams := stmt.IndexStmt.IndexParams
		for _, param := range indexParams {
			indexColumns = append(indexColumns, param.GetIndexElem().GetName())
		}
	}

	// create table 不需要查询数据库，跳过
	if len(tableName) == 0 {
		return "", nil
	}

	// 从数据库中获取索引对应列进行校验
	if len(schemaName) == 0 {
		schemaName = currentSchemaName
	}

	dbIndexColumns := pgContext.GetTableIndexColumns(schemaName, tableName)
	if len(dbIndexColumns) == 0 {
		// 获取数据库中索引列
		dbIndexColumns, err = utils.GetTableIndexColumns(pgContext.Executor, schemaName, tableName)
		if err != nil {
			return "", err
		}
	}

	for _, dbIndexColumn := range dbIndexColumns {
		eachDbIndexColumn := strings.Split(dbIndexColumn, ",")[:]
		for i := 0; i < len(indexColumns) && i < len(eachDbIndexColumn); i++ {
			if indexColumns[i] == eachDbIndexColumn[i] {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}
	return "", nil
}

func rule32(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		if stmt.InsertStmt.SelectStmt != nil && stmt.InsertStmt.SelectStmt.GetSelectStmt() != nil &&
			stmt.InsertStmt.SelectStmt.GetSelectStmt().GetFromClause() != nil {
			subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		if stmt.UpdateStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if stmt.UpdateStmt.WhereClause != nil && stmt.UpdateStmt.WhereClause.GetSubLink() == nil {
			if isWhereConditionAlwaysTrue(stmt.UpdateStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if stmt.DeleteStmt.WhereClause != nil && stmt.DeleteStmt.WhereClause.GetSubLink() == nil {
			if isWhereConditionAlwaysTrue(stmt.DeleteStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		if isWhereConditionAlwaysTrue(subQuery.WhereClause) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if subQuery.GetLarg() != nil {
			if isWhereConditionAlwaysTrue(subQuery.GetLarg().GetWhereClause()) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		if subQuery.GetRarg() != nil {
			if isWhereConditionAlwaysTrue(subQuery.GetRarg().GetWhereClause()) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}
	return "", nil
}

// 规则32方法：where条件是否恒为TRUE
func isWhereConditionAlwaysTrue(whereClause *parser.Node) bool {
	// 为空直接返回true触发规则
	if whereClause == nil {
		return true
	}

	switch clause := whereClause.Node.(type) {
	case *parser.Node_BoolExpr:
		// 检查子表达式
		leftTrue := isWhereConditionAlwaysTrue(clause.BoolExpr.Args[0])
		if len(clause.BoolExpr.Args) > 1 {
			rightTrue := isWhereConditionAlwaysTrue(clause.BoolExpr.Args[1])
			if clause.BoolExpr.Boolop == parser.BoolExprType_AND_EXPR {
				// 若子表达式都为 true，则整个表达式为 true
				if leftTrue && rightTrue {
					return true
				}
			} else if clause.BoolExpr.Boolop == parser.BoolExprType_OR_EXPR {
				// 若子表达式有一个为true，则整个表达式为 true
				if leftTrue || rightTrue {
					return true
				}
			}
		}
		return leftTrue
	case *parser.Node_AExpr:
		// 检查节点类型为 Op 的表达式
		if clause.AExpr.Kind == parser.A_Expr_Kind_AEXPR_OP {
			// 获取左右值
			leftValue := getConstantValue(clause.AExpr.Lexpr)
			rightValue := getConstantValue(clause.AExpr.Rexpr)
			switch clause.AExpr.Name[0].GetString_().GetSval() {
			case "=":
				// 检查左右值是否相等
				if leftValue == rightValue {
					return true
				}
			case "!=", "<>":
				// 检查左右值是否不相等
				if leftValue != rightValue {
					return true
				}
			case ">", ">=", "<", "<=":
				// 将值转换为适当的类型 (例如，将字符串转换为数字)
				leftNum, leftErr := strconv.ParseFloat(leftValue, 64)
				rightNum, rightErr := strconv.ParseFloat(rightValue, 64)
				// 左右都是数值才比较大小
				if leftErr == nil && rightErr == nil {
					// 根据运算符进行比较
					switch clause.AExpr.Name[0].GetString_().GetSval() {
					case ">":
						if leftNum > rightNum {
							return true
						}
					case ">=":
						if leftNum >= rightNum {
							return true
						}
					case "<":
						if leftNum < rightNum {
							return true
						}
					case "<=":
						if leftNum <= rightNum {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// getConstantValue :获取常量值
func getConstantValue(expr *parser.Node) string {
	// 检查表达式是否为常量节点
	constant, ok := expr.Node.(*parser.Node_AConst)
	if ok {
		switch nodeType := constant.AConst.Val.(type) {
		case *parser.A_Const_Ival:
			return strconv.Itoa(int(nodeType.Ival.GetIval()))
		case *parser.A_Const_Fval:
			return nodeType.Fval.GetFval()
		case *parser.A_Const_Sval:
			return nodeType.Sval.GetSval()
		}
	}
	typeCast, ok := expr.Node.(*parser.Node_TypeCast)
	if ok {
		return getConstantValue(typeCast.TypeCast.Arg)
	}
	columnRef, ok := expr.Node.(*parser.Node_ColumnRef)
	if ok {
		if columnRef != nil && columnRef.ColumnRef != nil {
			fields := columnRef.ColumnRef.Fields
			columnNames := make([]string, 0)
			for _, field := range fields {
				columnNames = append(columnNames, strings.ToLower(field.GetString_().GetSval()))
			}
			return strings.Join(columnNames, ".")
		}
	}
	return ""
}

func rule33(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		currentSelect := stmt.SelectStmt
		var subQueries []*parser.SelectStmt
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
		for _, subQuery := range subQueries {
			// 主查询跳过
			if subQuery == currentSelect {
				continue
			}
			result := recursiveValidateScalarSubQuery(subQuery)
			if result {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}
	return "", nil
}

// 规则33方法：递归校验标量子查询，标量子查询是只返回一个值的子查询，如：select id, (select count(*) from test1) as num from test2
func recursiveValidateScalarSubQuery(subQuery *parser.SelectStmt) bool {
	// 只返回一个值的子查询是标量子查询，注意：select * 这个不是标量子查询
	targetList := subQuery.TargetList
	if len(targetList) == 1 {
		// 标量子查询：1.使用了集合函数
		funcCall := subQuery.TargetList[0].GetResTarget().GetVal().GetFuncCall()
		if funcCall != nil {
			allowFunctionNames := []string{"count", "max", "min", "avg", "sum"}
			functionName := strings.ToLower(funcCall.GetFuncname()[0].GetString_().GetSval())
			for _, allowFunctionName := range allowFunctionNames {
				if functionName == allowFunctionName {
					return true
				}
			}
		}
		// 标量子查询：2.字段是常量值,如：select id,(select 1 from test1) from test2
		aConst := subQuery.TargetList[0].GetResTarget().GetVal().GetAConst()
		if aConst != nil {
			return true
		}
		// 表量子查询：3.select column1 from test limit 1或者select column1 from test FETCH FIRST 1 ROWS ONLY
		columnRef := subQuery.TargetList[0].GetResTarget().GetVal().GetColumnRef()
		if columnRef != nil {
			fields := columnRef.GetFields()
			if len(fields) == 1 {
				if subQuery.GetLimitCount() != nil &&
					subQuery.GetLimitCount().GetAConst().GetIval().GetIval() == 1 {
					return true
				}
			}
		}
		// 存在子查询
		subLink := subQuery.TargetList[0].GetResTarget().GetVal().GetSubLink()
		if subLink != nil {
			subSelect := subLink.GetSubselect()
			return recursiveValidateScalarSubQuery(subSelect.GetSelectStmt())
		}
	}
	return false
}

func rule34(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		if stmt.SelectStmt != nil {
			var subQueries []*parser.SelectStmt
			subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
			for _, subQuery := range subQueries {
				if _, isExist := findOrCondition(subQuery.WhereClause); isExist {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	}
	return "", nil
}

// 规则34方法：找到or的方法
func findOrCondition(node *parser.Node) (map[string]string, bool) {
	if node == nil || node.Node == nil {
		return nil, false
	}
	switch n := node.Node.(type) {
	case *parser.Node_BoolExpr:
		if n.BoolExpr.Boolop == parser.BoolExprType_OR_EXPR {
			columnNameMap := make(map[string]string)
			args := n.BoolExpr.GetArgs()
			for _, arg := range args {
				if arg.GetAExpr() != nil {
					setColumnNameMap(arg, columnNameMap)
				}
				if arg.GetBoolExpr() == nil {
					continue
				}
				getArgs := arg.GetBoolExpr().GetArgs()
				if len(getArgs) == 0 {
					continue
				}
				if getArgs[0].GetAExpr() != nil {
					setColumnNameMap(getArgs[0], columnNameMap)
					if len(columnNameMap) > 1 {
						return columnNameMap, true
					}
				}
				// 递归调用获取or连接的列名map
				recursiveColumnNameMap, isExist := findOrCondition(arg)
				if isExist {
					return recursiveColumnNameMap, true
				}
				for key, value := range recursiveColumnNameMap {
					if _, ok := columnNameMap[key]; !ok {
						columnNameMap[key] = value
					}
				}
			}
			if len(columnNameMap) > 1 {
				return columnNameMap, true
			}
		}
	}
	return nil, false
}

func setColumnNameMap(arg *parser.Node, columnNameMap map[string]string) {
	lExpr, rExpr := arg.GetAExpr().GetLexpr(), arg.GetAExpr().GetRexpr()
	columnName := getColumnNameFromWhereCondition(lExpr)
	if len(columnName) > 0 {
		columnNameMap[columnName] = columnName
	}
	columnName = getColumnNameFromWhereCondition(rExpr)
	if len(columnName) > 0 {
		columnNameMap[columnName] = columnName
	}
}

func getColumnNameFromWhereCondition(expr *parser.Node) string {
	if expr == nil {
		return ""
	}
	columnRef := expr.GetColumnRef()
	if columnRef != nil {
		columnNames := make([]string, 0)
		fields := columnRef.Fields
		for _, field := range fields {
			columnNames = append(columnNames, strings.ToLower(field.GetString_().GetSval()))
		}
		return strings.Join(columnNames, ".")
	}
	return ""
}

func rule35(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		relation := stmt.CreateStmt.GetRelation()
		if relation != nil {
			owner := relation.GetSchemaname()
			if len(owner) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_CreateTableAsStmt:
		into := stmt.CreateTableAsStmt.GetInto()
		if into != nil && into.GetRel() != nil {
			schemaName := into.GetRel().GetSchemaname()
			if len(schemaName) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		query := stmt.CreateTableAsStmt.GetQuery()
		if query != nil && query.GetSelectStmt() != nil {
			result, err := checkSchemaNameRecursive(query.GetSelectStmt(), rule.Name)
			if err != nil {
				return "", fmt.Errorf("check schema error for rule[%v]", rule.Name)
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	case *parser.Node_ViewStmt:
		schema := stmt.ViewStmt.View.GetSchemaname()
		if len(schema) == 0 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if stmt.ViewStmt.GetQuery() != nil && stmt.ViewStmt.GetQuery().GetSelectStmt() != nil {
			result, err := checkSchemaNameRecursive(stmt.ViewStmt.GetQuery().GetSelectStmt(), rule.Name)
			if err != nil {
				return "", fmt.Errorf("check schema error for rule[%v]", rule.Name)
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	case *parser.Node_IndexStmt:
		schemaName := stmt.IndexStmt.GetRelation().GetSchemaname()
		if len(schemaName) == 0 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateSeqStmt:
		sequence := stmt.CreateSeqStmt.GetSequence()
		if sequence != nil && len(sequence.GetSchemaname()) == 0 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateFunctionStmt:
		functionNames := stmt.CreateFunctionStmt.GetFuncname()
		if len(functionNames) == 1 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateTrigStmt:
		relation := stmt.CreateTrigStmt.GetRelation()
		if relation != nil {
			owner := relation.GetSchemaname()
			if len(owner) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_AlterTableStmt:
		relation := stmt.AlterTableStmt.GetRelation()
		if relation != nil {
			owner := relation.GetSchemaname()
			if len(owner) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_AlterSeqStmt:
		sequence := stmt.AlterSeqStmt.GetSequence()
		if sequence != nil && len(sequence.GetSchemaname()) == 0 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_RenameStmt:
		// alter trigger trigger_test on test rename to new_trigger_name;
		relation := stmt.RenameStmt.GetRelation()
		if relation == nil || len(relation.GetSchemaname()) == 0 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DropStmt:
		objects := stmt.DropStmt.GetObjects()
		for _, object := range objects {
			if object == nil {
				continue
			}
			if object.GetList() != nil {
				items := object.GetList().GetItems()
				switch stmt.DropStmt.GetRemoveType() {
				case parser.ObjectType_OBJECT_TRIGGER:
					if len(items) < 3 {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				default:
					if len(items) < 2 {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}
			// validate: drop function if exists log_test_changes;
			if object.GetObjectWithArgs() != nil {
				objNames := object.GetObjectWithArgs().GetObjname()
				if len(objNames) < 2 {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	case *parser.Node_AlterFunctionStmt:
		actions := stmt.AlterFunctionStmt.GetActions()
		for _, action := range actions {
			schemaName := action.GetRangeVar().GetSchemaname()
			if len(schemaName) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_InsertStmt:
		relation := stmt.InsertStmt.GetRelation()
		if relation != nil {
			if len(relation.GetSchemaname()) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		selectStmt := stmt.InsertStmt.GetSelectStmt()
		if selectStmt != nil && selectStmt.GetSelectStmt() != nil {
			result, err := checkSchemaNameRecursive(selectStmt.GetSelectStmt(), rule.Name)
			if err != nil {
				return "", fmt.Errorf("check schema error for rule[%v]", rule.Name)
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	case *parser.Node_UpdateStmt:
		relation := stmt.UpdateStmt.GetRelation()
		if relation != nil {
			if len(relation.GetSchemaname()) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		targetList := stmt.UpdateStmt.GetTargetList()
		for _, list := range targetList {
			if list == nil || list.GetResTarget() == nil || list.GetResTarget().GetVal() == nil {
				continue
			}
			if list.GetResTarget().GetVal().GetSubLink() == nil {
				continue
			}
			if list.GetResTarget().GetVal().GetSubLink().GetSubselect() == nil {
				continue
			}
			if sl := list.GetResTarget().GetVal().GetSubLink().GetSubselect().GetSelectStmt(); sl != nil {
				result, err := checkSchemaNameRecursive(sl, rule.Name)
				if err != nil {
					return "", fmt.Errorf("check schema error for rule[%v]", rule.Name)
				}
				if len(result) > 0 {
					return result, nil
				}
			}
		}
		whereClause := stmt.UpdateStmt.GetWhereClause()
		if whereClause != nil && whereClause.GetSubLink() != nil && whereClause.GetSubLink().GetSubselect() != nil &&
			whereClause.GetSubLink().GetSubselect().GetSelectStmt() != nil {
			result, err := checkSchemaNameRecursive(whereClause.GetSubLink().GetSubselect().GetSelectStmt(), rule.Name)
			if err != nil {
				return "", fmt.Errorf("check schema error for rule[%v]", rule.Name)
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	case *parser.Node_DeleteStmt:
		relation := stmt.DeleteStmt.GetRelation()
		if relation != nil {
			if len(relation.GetSchemaname()) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		whereClause := stmt.DeleteStmt.GetWhereClause()
		if whereClause != nil && whereClause.GetSubLink() != nil && whereClause.GetSubLink().GetSubselect() != nil &&
			whereClause.GetSubLink().GetSubselect().GetSelectStmt() != nil {
			result, err := checkSchemaNameRecursive(whereClause.GetSubLink().GetSubselect().GetSelectStmt(), rule.Name)
			if err != nil {
				return "", fmt.Errorf("check schema error for rule[%v]", rule.Name)
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	case *parser.Node_SelectStmt:
		selectStmt := stmt.SelectStmt
		if selectStmt != nil {
			result, err := checkSchemaNameRecursive(selectStmt, rule.Name)
			if err != nil {
				return "", fmt.Errorf("check schema error for rule[%v]", rule.Name)
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	default:
	}
	return "", nil
}

// rule35方法 checkSchemaNameRecursive: 校验schema是否存在
func checkSchemaNameRecursive(selectStmt *parser.SelectStmt, ruleName string) (string, error) {
	fromClauses := selectStmt.FromClause
	for _, fromClause := range fromClauses {
		if fromClause != nil && fromClause.GetRangeVar() != nil && len(fromClause.GetRangeVar().GetSchemaname()) == 0 {
			return RuleHandlerMap[ruleName].Message, nil
		}
		if recursiveValidateJoinOwner(fromClause.GetJoinExpr()) {
			return RuleHandlerMap[ruleName].Message, nil
		}
	}

	whereClause := selectStmt.WhereClause
	if whereClause != nil && whereClause.GetFromExpr() != nil {
		fromList := whereClause.GetFromExpr().Fromlist
		for _, from := range fromList {
			if from != nil && from.GetRangeVar() != nil && len(from.GetRangeVar().GetSchemaname()) == 0 {
				return RuleHandlerMap[ruleName].Message, nil
			}
			if recursiveValidateJoinOwner(from.GetJoinExpr()) {
				return RuleHandlerMap[ruleName].Message, nil
			}
		}
	}

	if whereClause != nil && whereClause.GetBoolExpr() != nil {
		args := whereClause.GetBoolExpr().Args
		for _, arg := range args {
			if arg == nil || arg.GetSubLink() == nil || arg.GetSubLink().GetSubselect() == nil {
				continue
			}
			if sl := arg.GetSubLink().GetSubselect().GetSelectStmt(); sl != nil {
				result, err := checkSchemaNameRecursive(sl, ruleName)
				if err != nil {
					return "", err
				}
				if len(result) > 0 {
					return result, nil
				}
			}
		}
	}

	if whereClause != nil && whereClause.GetSubLink() != nil {
		if whereClause.GetSubLink().GetSubselect() != nil {
			stmt := whereClause.GetSubLink().GetSubselect().GetSelectStmt()
			if stmt != nil {
				result, err := checkSchemaNameRecursive(stmt, ruleName)
				if err != nil {
					return "", err
				}
				if len(result) > 0 {
					return result, nil
				}
			}
		}
	}

	targetList := selectStmt.TargetList
	for _, list := range targetList {
		if list == nil || list.GetResTarget() == nil || list.GetResTarget().GetVal() == nil {
			continue
		}
		if list.GetResTarget().GetVal().GetSubLink() == nil {
			continue
		}
		if list.GetResTarget().GetVal().GetSubLink().GetSubselect() == nil {
			continue
		}
		if sl := list.GetResTarget().GetVal().GetSubLink().GetSubselect().GetSelectStmt(); sl != nil {
			result, err := checkSchemaNameRecursive(sl, ruleName)
			if err != nil {
				return "", err
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	}
	return "", nil
}

func recursiveValidateJoinOwner(joinExpr *parser.JoinExpr) bool {
	if joinExpr != nil {
		lArg := joinExpr.GetLarg()
		rArg := joinExpr.GetRarg()
		if lArg != nil {
			if lArg.GetRangeVar() != nil && len(lArg.GetRangeVar().GetSchemaname()) == 0 {
				return true
			}
		}
		if rArg != nil {
			if rArg.GetRangeVar() != nil && len(rArg.GetRangeVar().GetSchemaname()) == 0 {
				return true
			}
		}
		if lArg.GetJoinExpr() != nil {
			return recursiveValidateJoinOwner(lArg.GetJoinExpr())
		}
	}
	return false
}

func rule36(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_DropStmt:
		objects := stmt.DropStmt.Objects
		if len(objects) > 0 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_DropdbStmt, *parser.Node_DropTableSpaceStmt:
		return RuleHandlerMap[rule.Name].Message, nil
	default:
	}
	return "", nil
}

func rule37(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_AlterTableStmt:
		cmd := stmt.AlterTableStmt.GetCmds()[0]
		if cmd.GetAlterTableCmd().GetSubtype() == parser.AlterTableType_AT_DropColumn {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule38(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		if stmt.SelectStmt != nil {
			subQueries := utils.ExtractSubQueries(node.GetStmt())
			for _, subQuery := range *subQueries {
				if countDqlUnions(subQuery) > 0 {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	}
	return "", nil
}

func rule39(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	tableName := ""
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	// 获取最大索引字段数，默认是5
	maxIndexFields := rule.Params.GetParam("expect_max_index_column_number").Int()

	indexColumnMap := make(map[string]string)
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// create table test(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test unique(col1,col2,col3,col4,col5,col6));
		// create table test(id int,col1 int,col2 int,col3 int,col4 int,col5 int,col6 int,constraint constraint_test1 unique(col1,col2,col3),constraint constraint_test2unique(col4,col5,col6));
		for _, colDef := range stmt.CreateStmt.TableElts {
			constraint := colDef.GetConstraint()
			if constraint != nil {
				constraintType := constraint.GetContype()
				if constraintType == parser.ConstrType_CONSTR_UNIQUE ||
					constraintType == parser.ConstrType_CONSTR_PRIMARY {
					keys := constraint.GetKeys()
					for _, key := range keys {
						if _, ok := indexColumnMap[key.GetString_().GetSval()]; !ok {
							indexColumnMap[key.GetString_().GetSval()] = key.GetString_().GetSval()
						}
					}
				}
			}
		}
	case *parser.Node_IndexStmt:
		// create index idx_index_name on test(col1,col2,col3,col4,col5,col6);
		indexParams := stmt.IndexStmt.IndexParams
		for _, param := range indexParams {
			if _, ok := indexColumnMap[param.GetIndexElem().GetName()]; !ok {
				indexColumnMap[param.GetIndexElem().GetName()] = param.GetIndexElem().GetName()
			}
		}
		// 触发校验规则
		if len(indexColumnMap) > maxIndexFields {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxIndexFields), nil
		}
		// 查询数据库中同一表的所有索引列，如果加上当前新增索引的列超过阈值，也触发规则
		schemaname := stmt.IndexStmt.GetRelation().GetSchemaname()
		tableName = stmt.IndexStmt.GetRelation().GetRelname()
		if len(schemaname) == 0 {
			schemaname = currentSchemaName
		}

		indexColumnsRows := pgContext.GetTableIndexColumns(schemaname, tableName)
		for _, indexColumns := range indexColumnsRows {
			columns := strings.Split(indexColumns, ",")
			for _, column := range columns {
				if _, ok := indexColumnMap[column]; !ok {
					indexColumnMap[column] = column
				}
			}
		}

		if len(indexColumnMap) == 0 {
			indexColumnMapDb, msg, err := getTableIndexColumnListForRule39(pgContext, schemaname, tableName)
			if err != nil {
				return msg, err
			}
			for key, value := range indexColumnMapDb {
				if _, ok := indexColumnMap[key]; !ok {
					indexColumnMap[key] = value
				}
			}
		}
	case *parser.Node_AlterTableStmt:
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			tableCmd := cmd.GetAlterTableCmd()
			alterType := tableCmd.GetSubtype()
			switch alterType {
			case parser.AlterTableType_AT_AddColumn:
				// alter table test add index idx_constraint_test(col1,col2,col3,col4,col5,col6);
				typmods := tableCmd.GetDef().GetColumnDef().GetTypeName().GetTypmods()
				for _, typmod := range typmods {
					if _, ok := indexColumnMap[typmod.GetString_().GetSval()]; !ok {
						indexColumnMap[typmod.GetString_().GetSval()] = typmod.GetString_().GetSval()
					}
				}
			case parser.AlterTableType_AT_AddConstraint:
				// alter table test add constraint idx_constraint_test unique(col1,col2,col3,col4,col5,col6);
				typeConstrType := tableCmd.Def.GetConstraint().GetContype()
				if typeConstrType == parser.ConstrType_CONSTR_UNIQUE ||
					typeConstrType == parser.ConstrType_CONSTR_PRIMARY {
					keys := tableCmd.Def.GetConstraint().Keys
					for _, key := range keys {
						if _, ok := indexColumnMap[key.GetString_().GetSval()]; !ok {
							indexColumnMap[key.GetString_().GetSval()] = key.GetString_().GetSval()
						}
					}
				}
			default:
			}
		}
		// 触发校验规则
		if len(indexColumnMap) > maxIndexFields {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxIndexFields), nil
		}
		// 查询数据库中同一表的所有索引列，如果加上当前新增索引的列超过阈值，也触发规则
		schemaname := stmt.AlterTableStmt.GetRelation().GetSchemaname()
		tableName = stmt.AlterTableStmt.GetRelation().GetRelname()
		if len(schemaname) == 0 {
			schemaname = currentSchemaName
		}

		indexColumnsRows := pgContext.GetTableIndexColumns(schemaname, tableName)
		for _, indexColumns := range indexColumnsRows {
			columns := strings.Split(indexColumns, ",")
			for _, column := range columns {
				if _, ok := indexColumnMap[column]; !ok {
					indexColumnMap[column] = column
				}
			}
		}

		if len(indexColumnMap) == 0 {
			indexColumnMapDb, msg, err := getTableIndexColumnListForRule39(pgContext, schemaname, tableName)
			if err != nil {
				return msg, err
			}
			for key, value := range indexColumnMapDb {
				if _, ok := indexColumnMap[key]; !ok {
					indexColumnMap[key] = value
				}
			}
		}
	default:
	}

	// 触发校验规则
	if len(indexColumnMap) > maxIndexFields {
		return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxIndexFields), nil
	}
	return "", nil
}

func getTableIndexColumnListForRule39(pgContext *PgContext, schemaname string, tableName string) (map[string]string, string, error) {
	indexColumnMap := make(map[string]string)
	if pgContext.Executor == nil {
		return indexColumnMap, "", nil
	}
	indexesInfo, err := pgContext.GetTableIndexesInfo(schemaname, tableName)
	if err != nil {
		return indexColumnMap, "", err
	}
	for _, indexInfo := range indexesInfo {
		indexDDL := indexInfo.IndexDDL
		if len(indexDDL) == 0 {
			continue
		}
		ast, err := SqlParserFunc(indexDDL)
		if err != nil {
			return indexColumnMap, "", fmt.Errorf("parse sql=%s failed: %v", indexDDL, err)
		}
		switch stmt := ast.(*parser.RawStmt).GetStmt().GetNode().(type) {
		case *parser.Node_IndexStmt:
			indexParams := stmt.IndexStmt.IndexParams
			for _, indexParam := range indexParams {
				if _, ok := indexColumnMap[indexParam.GetIndexElem().GetName()]; !ok {
					indexColumnMap[indexParam.GetIndexElem().GetName()] = indexParam.GetIndexElem().GetName()
				}
			}
		}
	}
	return indexColumnMap, "", nil
}

func countDqlUnions(selectStmt *parser.SelectStmt) int {
	unionNumber := 0
	countDqlUnionsRecursive(selectStmt, &unionNumber)
	return unionNumber
}

func countDqlUnionsRecursive(selectStmt *parser.SelectStmt, unionNumber *int) {
	if selectStmt != nil {
		if selectStmt.Op == parser.SetOperation_SETOP_UNION && !selectStmt.GetAll() {
			*unionNumber++
		}
		if selectStmt.Larg != nil {
			countDqlUnionsRecursive(selectStmt.Larg, unionNumber)
		}
		if selectStmt.Rarg != nil {
			countDqlUnionsRecursive(selectStmt.Rarg, unionNumber)
		}
	}
}

func rule40(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		isPrimaryKey := false
		isAutoIncrement := false
		schema := stmt.CreateStmt.Relation.Schemaname
		if len(schema) == 0 {
			schema = pgContext.DatabaseInfo.CurrentSchema
		}
		tableName := stmt.CreateStmt.Relation.Relname
		for _, colDef := range stmt.CreateStmt.TableElts {

			// 主键使用：id int PRIMARY KEY
			constraints := colDef.GetColumnDef().GetConstraints()

			// 校验serial和bigserial
			// 校验序列自增主键类型：DEFAULT nextval('table_name_id_seq')
			// 只要两者至少有一个存在就通过
			isAutoIncrement = validateAutoIncrementPrimaryKey(colDef) || validateDefaultNextval(constraints)
			for _, constraint := range constraints {
				constraintType := constraint.GetConstraint().GetContype()
				if constraintType == parser.ConstrType_CONSTR_PRIMARY {
					isPrimaryKey = true
					if isAutoIncrement {
						break
					}
				}
			}

			// 自增列写到pg上下文中
			if isAutoIncrement {
				columnName := colDef.GetColumnDef().GetColname()
				columnType := ""
				names := colDef.GetColumnDef().GetTypeName().GetNames()
				for _, name := range names {
					nameOb, ok := name.Node.(*parser.Node_String_)
					if ok {
						columnType = strings.ToLower(nameOb.String_.GetSval())
					}
				}
				setColumnDefaultValueIntoPgContext(pgContext, schema, tableName, columnName, columnType)
			}

			// 自增主键跳出循环
			if isPrimaryKey && isAutoIncrement {
				break
			}

			// 主键使用：主键在CONSTRAINT中实现
			constraint := colDef.GetConstraint()
			if constraint != nil {
				constraintType := constraint.GetContype()
				if constraintType == parser.ConstrType_CONSTR_PRIMARY {
					keys := constraint.GetKeys()
					// 不考虑联合主键
					if len(keys) > 1 {
						continue
					}
					isPrimaryKey = true
					for _, key := range keys {
						for _, columnDef := range stmt.CreateStmt.TableElts {
							columnName := columnDef.GetColumnDef().GetColname()
							keyStr := key.GetString_().GetSval()
							if strings.Compare(keyStr, columnName) == 0 {
								// 校验serial和bigserial
								isAutoIncrement = validateAutoIncrementPrimaryKey(columnDef)
								if isAutoIncrement {
									break
								}
								// 获取表约束
								tableConstraints := columnDef.GetColumnDef().GetConstraints()
								// 序列自增主键类型：DEFAULT nextval('table_name_id_seq')
								isAutoIncrement = validateDefaultNextval(tableConstraints)
								if isAutoIncrement {
									break
								}
							}
						}
					}
				}
			}

			// 自增主键跳出循环
			if isPrimaryKey && isAutoIncrement {
				break
			}
		}

		// 不是自增主键走规则
		if isPrimaryKey && !isAutoIncrement {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterTableStmt:
		schema := stmt.AlterTableStmt.Relation.Schemaname
		if len(schema) == 0 {
			schema = currentSchemaName
		}
		tableName := stmt.AlterTableStmt.Relation.Relname
		isPrimaryKey := false
		isAutoIncrement := false
		primaryColumnName, autoIncrementColumnName := "", ""
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			tableCmd := cmd.GetAlterTableCmd()
			alterType := tableCmd.GetSubtype()
			switch alterType {
			case parser.AlterTableType_AT_AddColumn:
				isAutoIncrement = validateAutoIncrementPrimaryKey(tableCmd.GetDef())
				if !isAutoIncrement {
					constraints := tableCmd.GetDef().GetColumnDef().GetConstraints()
					// 序列自增主键类型：DEFAULT nextval('table_name_id_seq')
					isAutoIncrement = validateDefaultNextval(constraints)
				}
				if isAutoIncrement {
					autoIncrementColumnName = tableCmd.GetDef().GetColumnDef().GetColname()
					columnType := ""
					names := tableCmd.GetDef().GetColumnDef().GetTypeName().GetNames()
					for _, name := range names {
						nameOb, ok := name.Node.(*parser.Node_String_)
						if ok {
							columnType = strings.ToLower(nameOb.String_.GetSval())
						}
					}
					setColumnDefaultValueIntoPgContext(pgContext, schema, tableName, autoIncrementColumnName, columnType)
				}
			case parser.AlterTableType_AT_ColumnDefault: // 修改列增加自增主键
				if tableCmd.GetDef().GetFuncCall() != nil {
					funcNames := tableCmd.GetDef().GetFuncCall().GetFuncname()
					for _, funcName := range funcNames {
						nameOb, ok := funcName.Node.(*parser.Node_String_)
						if ok {
							functionName := strings.ToLower(nameOb.String_.GetSval())
							if functionName == "nextval" {
								isAutoIncrement = true
								break
							}
						}
					}
				}
				if isAutoIncrement {
					autoIncrementColumnName = tableCmd.Name
					columnType := ""
					names := tableCmd.GetDef().GetColumnDef().GetTypeName().GetNames()
					for _, name := range names {
						nameOb, ok := name.Node.(*parser.Node_String_)
						if ok {
							columnType = strings.ToLower(nameOb.String_.GetSval())
						}
					}
					setColumnDefaultValueIntoPgContext(pgContext, schema, tableName, autoIncrementColumnName, columnType)
				}
			case parser.AlterTableType_AT_AddConstraint:
				constraint := tableCmd.GetDef().GetConstraint()
				if constraint.GetContype() == parser.ConstrType_CONSTR_PRIMARY {
					isPrimaryKey = true
				}
				if isPrimaryKey {
					keys := constraint.GetKeys()
					if len(keys) == 1 {
						primaryColumnName = keys[0].GetString_().GetSval()
					}
				}
				// 从上下文中获取列的默认值，并判断是否为自增列
				if _, ok := pgContext.DatabaseInfo.SchemaInfoMap[schema]; !ok {
					return "", fmt.Errorf("schema %s not exist", schema)
				}
				tableInfoList := pgContext.DatabaseInfo.SchemaInfoMap[schema].TableInfoList
				for _, tableInfo := range tableInfoList {
					if tableInfo.TableName != tableName {
						continue
					}
					columnInfoList := tableInfo.ColumnInfoList
					for _, columnInfo := range columnInfoList {
						if columnInfo.ColumnName != primaryColumnName {
							continue
						}
						if strings.Contains(columnInfo.DefaultValue, "nextval(") {
							autoIncrementColumnName = columnInfo.ColumnName
							isAutoIncrement = true
							break
						}
					}
				}
				if isAutoIncrement {
					break
				}
				// 校验：ALTER TABLE test4 ADD PRIMARY KEY (id);
				// 从数据中获取字段id的定义和约束，看是否存在自增
				resultMap, err := pgContext.GetTableAutoIncrementColumnDefaultValue(schema, tableName)
				if err != nil {
					return "", err
				}
				// 写入列默认值到pgContext对应表中
				for key, value := range resultMap {
					if _, ok := pgContext.DatabaseInfo.SchemaInfoMap[schema]; !ok {
						break
					}
					tableInfoList := pgContext.DatabaseInfo.SchemaInfoMap[schema].TableInfoList
					for _, tableInfo := range tableInfoList {
						if tableInfo.TableName != tableName {
							continue
						}
						columnInfoList := tableInfo.ColumnInfoList
						for _, columnInfo := range columnInfoList {
							if columnInfo.ColumnName == key {
								columnInfo.DefaultValue = value
							}
						}
					}
				}
				if len(resultMap) > 0 {
					isAutoIncrement = true
					if _, isExistColumn := resultMap[primaryColumnName]; isExistColumn {
						return "", nil
					}
				}
				if isAutoIncrement {
					break
				}
			default:
			}
		}
		if !isPrimaryKey || !isAutoIncrement || primaryColumnName != autoIncrementColumnName {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	default:
	}
	return "", nil
}

func setColumnDefaultValueIntoPgContext(pgContext *PgContext, schema, tableName, columnName, columnType string) {
	// 设置列的默认值为:nextval('table_name_id_seq')
	if _, ok := pgContext.DatabaseInfo.SchemaInfoMap[schema]; !ok {
		return
	}
	tableInfoList := pgContext.DatabaseInfo.SchemaInfoMap[schema].TableInfoList
	isExistTable := false
	for _, tableInfo := range tableInfoList {
		if tableInfo.TableName == tableName {
			isExistTable = true
			break
		}
	}

	if !isExistTable {
		tableInfoList = append(tableInfoList, &TableInfo{
			TableName:      tableName,
			OwnerName:      schema,
			ColumnInfoList: make([]*ColumnInfo, 0),
		})
	}

	for _, tableInfo := range tableInfoList {
		if tableInfo.TableName == tableName {
			columnInfoList := tableInfo.ColumnInfoList
			if len(columnInfoList) == 0 {
				columnInfoList = append(columnInfoList, &ColumnInfo{
					OwnerName:    schema,
					TableName:    tableName,
					ColumnName:   columnName,
					ColumnType:   columnType,
					DefaultValue: "nextval('table_name_id_seq')",
				})
				tableInfo.ColumnInfoList = columnInfoList
				continue
			}
			for _, columnInfo := range columnInfoList {
				if columnInfo.ColumnName == columnName {
					columnInfo.DefaultValue = "nextval('table_name_id_seq')"
					break
				}
			}
			tableInfo.ColumnInfoList = columnInfoList
			break
		}
	}
	pgContext.DatabaseInfo.SchemaInfoMap[schema].TableInfoList = tableInfoList
}

func validateAutoIncrementPrimaryKey(columnDef *parser.Node) bool {
	isAutoIncrement := false
	names := columnDef.GetColumnDef().GetTypeName().GetNames()
	for _, name := range names {
		nameOb, ok := name.Node.(*parser.Node_String_)
		if ok {
			typeName := strings.ToLower(nameOb.String_.GetSval())
			if typeName == "serial" {
				isAutoIncrement = true
				break
			}
			if typeName == "bigserial" {
				isAutoIncrement = true
				break
			}
		}
	}
	constraints := columnDef.GetColumnDef().GetConstraints()
	for _, constraint := range constraints {
		generatedWhen := constraint.GetConstraint().GetGeneratedWhen()
		if len(generatedWhen) > 0 {
			isAutoIncrement = true
			break
		}
	}
	return isAutoIncrement
}

func validateDefaultNextval(tableConstraints []*parser.Node) bool {
	isAutoIncrement := false
	for _, defaultConstraint := range tableConstraints {
		if defaultConstraint.GetConstraint() == nil {
			continue
		}
		defaultConstraintType := defaultConstraint.GetConstraint().GetContype()
		if defaultConstraintType == parser.ConstrType_CONSTR_DEFAULT {
			if defaultConstraint.GetConstraint().GetRawExpr() == nil {
				continue
			}
			if defaultConstraint.GetConstraint().GetRawExpr().GetFuncCall() == nil {
				continue
			}
			funcNames := defaultConstraint.GetConstraint().GetRawExpr().GetFuncCall().GetFuncname()
			for _, funcName := range funcNames {
				nameOb, ok := funcName.Node.(*parser.Node_String_)
				if ok {
					functionName := strings.ToLower(nameOb.String_.GetSval())
					if functionName == "nextval" {
						isAutoIncrement = true
						break
					}
				}
			}
		}
	}
	return isAutoIncrement
}

func rule41(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	// 表链接个数
	expectTableConnectNumber := rule.Params.GetParam("max_table_connect_number").Int()

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		if stmt.InsertStmt.SelectStmt != nil {
			subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		}
		if stmt.InsertStmt.GetWithClause() != nil {
			subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
		}
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		if stmt.UpdateStmt.WhereClause != nil {
			subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		}
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		if stmt.UpdateStmt.GetWithClause() != nil {
			subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
		}
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		// 表连接数：表的总数-1
		if len(subQuery.FromClause)-1 > expectTableConnectNumber {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectTableConnectNumber), nil
		}
		fromClause := subQuery.FromClause
		for _, from := range fromClause {
			if from == nil || from.GetJoinExpr() == nil {
				continue
			}
			// 校验join表连接个数，表连接数 等于 表的总数-1 等于 join的个数
			joinCount := getJoinCountForRule41(from.GetJoinExpr(), currentSchemaName)
			if joinCount > expectTableConnectNumber {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectTableConnectNumber), nil
			}
		}
	}
	return "", nil
}

func getJoinCountForRule41(joinExpr *parser.JoinExpr, schema string) int {
	return countJoins(joinExpr, schema)
}

func countJoins(node *parser.JoinExpr, schema string) int {
	if node == nil {
		return 0
	}
	joinCount := 1
	if node.Larg != nil {
		joinCount += countJoins(node.Larg.GetJoinExpr(), schema)
	}
	if node.Rarg != nil {
		joinCount += countJoins(node.Rarg.GetJoinExpr(), schema)
	}
	return joinCount
}

func rule42(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	// 索引列
	columns := make([]string, 0)
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		if stmt.UpdateStmt.WhereClause != nil && stmt.UpdateStmt.WhereClause.GetSubLink() == nil {
			tableIndexColumnMap := make(map[string][]string)
			schemaName := stmt.UpdateStmt.GetRelation().GetSchemaname()
			tableName := stmt.UpdateStmt.GetRelation().GetRelname()
			tableNameAlias := stmt.UpdateStmt.GetRelation().GetAlias()
			if len(schemaName) == 0 {
				schemaName = currentSchemaName
			}
			columns = append(columns, pgContext.GetTableIndexColumns(schemaName, tableName)...)
			if len(columns) == 0 {
				// 获取索引列
				columns, err = getIndexColumnsFromDb(schemaName, tableName, pgContext)
				if err != nil {
					return "", err
				}
			}
			if tableNameAlias != nil {
				tableIndexColumnMap[tableNameAlias.GetAliasname()] = columns
			} else {
				tableIndexColumnMap[tableName] = columns
			}
			if findIndexColumnUsingFunctionOrExpression(stmt.UpdateStmt.WhereClause, tableIndexColumnMap) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause != nil && stmt.DeleteStmt.WhereClause.GetSubLink() == nil {
			tableIndexColumnMap := make(map[string][]string)
			schemaName := stmt.DeleteStmt.GetRelation().GetSchemaname()
			tableName := stmt.DeleteStmt.GetRelation().GetRelname()
			tableNameAlias := stmt.DeleteStmt.GetRelation().GetAlias()
			if len(schemaName) == 0 {
				schemaName = currentSchemaName
			}
			columns = pgContext.GetTableIndexColumns(schemaName, tableName)
			if len(columns) == 0 {
				// 获取索引列
				columns, err = utils.GetTableIndexColumns(pgContext.Executor, schemaName, tableName)
				if err != nil {
					return "", err
				}
			}
			if tableNameAlias != nil {
				tableIndexColumnMap[tableNameAlias.GetAliasname()] = columns
			} else {
				tableIndexColumnMap[tableName] = columns
			}
			if findIndexColumnUsingFunctionOrExpression(stmt.DeleteStmt.WhereClause, tableIndexColumnMap) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		tableIndexColumnMap := make(map[string][]string)
		fromClause := subQuery.FromClause
		for _, from := range fromClause {
			if from.GetRangeVar() != nil {
				schemaName := from.GetRangeVar().GetSchemaname()
				tableName := from.GetRangeVar().GetRelname()
				tableNameAlias := from.GetRangeVar().GetAlias()
				if len(schemaName) == 0 {
					schemaName = currentSchemaName
				}
				columns = pgContext.GetTableIndexColumns(schemaName, tableName)

				if len(columns) == 0 {
					// 获取索引列
					columns, err = utils.GetTableIndexColumns(pgContext.Executor, schemaName, tableName)
					if err != nil {
						return "", err
					}
				}

				if tableNameAlias != nil {
					tableIndexColumnMap[tableNameAlias.GetAliasname()] = columns
				} else {
					tableIndexColumnMap[tableName] = columns
				}
			}
		}
		if findIndexColumnUsingFunctionOrExpression(subQuery.WhereClause, tableIndexColumnMap) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if subQuery.GetLarg() != nil {
			if findIndexColumnUsingFunctionOrExpression(subQuery.GetLarg().GetWhereClause(), tableIndexColumnMap) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		if subQuery.GetRarg() != nil {
			if findIndexColumnUsingFunctionOrExpression(subQuery.GetRarg().GetWhereClause(), tableIndexColumnMap) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}
	return "", nil
}

// 规则42方法：递归检查WHERE条件中索引列是否使用了函数或表达式
func findIndexColumnUsingFunctionOrExpression(node *parser.Node, tableIndexColumnMap map[string][]string) bool {
	if node == nil || node.Node == nil {
		return false
	}
	switch n := node.Node.(type) {
	case *parser.Node_BoolExpr:
		return loopHandleArgs(n.BoolExpr.Args, tableIndexColumnMap)
	case *parser.Node_AExpr:
		if n.AExpr != nil && n.AExpr.GetKind() == parser.A_Expr_Kind_AEXPR_OP {
			lExpr, rExpr := n.AExpr.GetLexpr(), n.AExpr.GetRexpr()
			if hasIndexColumnOperate(lExpr, true, tableIndexColumnMap) ||
				hasIndexColumnOperate(rExpr, true, tableIndexColumnMap) {
				return true
			}
		}
	case *parser.Node_SelectStmt:
		stmt := n.SelectStmt
		if where := stmt.WhereClause; where != nil {
			if expr := where.GetAExpr(); expr != nil {
				if le := expr.GetLexpr(); le != nil {
					if findIndexColumnUsingFunctionOrExpression(le, tableIndexColumnMap) {
						return true
					}
				}
				if re := expr.GetRexpr(); re != nil {
					if findIndexColumnUsingFunctionOrExpression(re, tableIndexColumnMap) {
						return true
					}
				}
			}
			if boolExpr := where.GetBoolExpr(); boolExpr != nil {
				return loopHandleArgs(boolExpr.Args, tableIndexColumnMap)
			}
		}
	}
	return false
}

// 规则42方法：循环处理参数
func loopHandleArgs(args []*parser.Node, tableIndexColumnMap map[string][]string) bool {
	for _, arg := range args {
		if findIndexColumnUsingFunctionOrExpression(arg, tableIndexColumnMap) {
			return true
		}
	}
	return false
}

// 规则42方法：判断是否索引列使用表达式操作
func hasIndexColumnOperate(expr *parser.Node, isExpressionOp bool, tableIndexColumnMap map[string][]string) bool {
	if expr != nil && expr.GetAExpr() != nil && expr.GetAExpr().GetKind() == parser.A_Expr_Kind_AEXPR_OP {
		lExpr, rExpr := expr.GetAExpr().GetLexpr(), expr.GetAExpr().GetRexpr()
		if hasIndexColumnOperate(lExpr, true, tableIndexColumnMap) ||
			hasIndexColumnOperate(rExpr, true, tableIndexColumnMap) {
			return true
		}
	}
	// 表达式调用
	if isExpressionOp && expr != nil && expr.GetColumnRef() != nil && len(expr.GetColumnRef().GetFields()) > 0 {
		fields := expr.GetColumnRef().GetFields()
		if validateIsIndexColumn(fields, tableIndexColumnMap) {
			return true
		}
	}
	// 递归函数调用
	if recursiveHandleIndexColumnFuncCall(expr, tableIndexColumnMap) {
		return true
	}
	// 递归cast函数调用
	if recursiveHandleIndexColumnCastFunction(expr, tableIndexColumnMap) {
		return true
	}
	return false
}

// 规则42方法：校验是否索引列
func validateIsIndexColumn(fields []*parser.Node, tableIndexColumnMap map[string][]string) bool {
	if len(fields) == 0 {
		return false
	}
	field := fields[0].GetString_().GetSval()
	if len(fields) == 1 {
		for _, indexColumns := range tableIndexColumnMap {
			for _, indexColumn := range indexColumns {
				if indexColumn == field {
					return true
				}
			}
		}
	} else if len(fields) == 2 {
		for key, indexColumns := range tableIndexColumnMap {
			// 表名或表别名相同
			if key == field {
				currentColumn := fields[1].GetString_().GetSval()
				for _, indexColumn := range indexColumns {
					// 列是索引列
					if indexColumn == currentColumn {
						return true
					}
				}
			}
		}
	}
	return false
}

// 规则42方法：递归处理索引列函数调用
func recursiveHandleIndexColumnFuncCall(expr *parser.Node, tableIndexColumnMap map[string][]string) bool {
	if expr == nil || expr.GetFuncCall() == nil || len(expr.GetFuncCall().GetArgs()) == 0 {
		return false
	}
	args := expr.GetFuncCall().GetArgs()
	for _, arg := range args {
		if arg.GetFuncCall() != nil {
			if recursiveHandleIndexColumnFuncCall(arg, tableIndexColumnMap) {
				return true
			}
		}
		if arg.GetColumnRef() != nil {
			fields := arg.GetColumnRef().GetFields()
			if validateIsIndexColumn(fields, tableIndexColumnMap) {
				return true
			}
			continue
		}
		if arg.GetAExpr() != nil {
			lExpr, rExpr := arg.GetAExpr().GetLexpr(), arg.GetAExpr().GetRexpr()
			if lExpr.GetColumnRef() != nil && len(lExpr.GetColumnRef().GetFields()) > 0 {
				fields := lExpr.GetColumnRef().GetFields()
				if validateIsIndexColumn(fields, tableIndexColumnMap) {
					return true
				}
			}
			if rExpr.GetColumnRef() != nil && len(rExpr.GetColumnRef().GetFields()) > 0 {
				fields := rExpr.GetColumnRef().GetFields()
				if validateIsIndexColumn(fields, tableIndexColumnMap) {
					return true
				}
			}
		}
	}
	return false
}

// 规则42方法：递归处理索引列cast调用
func recursiveHandleIndexColumnCastFunction(expr *parser.Node, tableIndexColumnMap map[string][]string) bool {
	if expr == nil || expr.GetTypeCast() == nil || expr.GetTypeCast().GetArg() == nil {
		return false
	}
	if expr.GetTypeCast().GetArg().GetTypeCast() != nil {
		if recursiveHandleIndexColumnCastFunction(expr.GetTypeCast().GetArg(), tableIndexColumnMap) {
			return true
		}
		return false
	}
	fields := expr.GetTypeCast().GetArg().GetColumnRef().GetFields()
	if validateIsIndexColumn(fields, tableIndexColumnMap) {
		return true
	}
	return false
}

func rule43(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		if stmt.UpdateStmt.WhereClause != nil && stmt.UpdateStmt.WhereClause.GetSubLink() == nil {
			if containsExpression(stmt.UpdateStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause != nil && stmt.DeleteStmt.WhereClause.GetSubLink() == nil {
			if containsExpression(stmt.DeleteStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		if containsExpression(subQuery.WhereClause) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if subQuery.GetLarg() != nil {
			if containsExpression(subQuery.GetLarg().GetWhereClause()) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		if subQuery.GetRarg() != nil {
			if containsExpression(subQuery.GetRarg().GetWhereClause()) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}
	return "", nil
}

// 规则43方法：递归检查 WHERE 条件中是否使用了表达式
func containsExpression(node *parser.Node) bool {
	if node == nil || node.Node == nil {
		return false
	}
	switch n := node.Node.(type) {
	case *parser.Node_BoolExpr:
		return loopHandleArgsForOrdinaryColumn(n.BoolExpr.Args)
	case *parser.Node_AExpr:
		if n.AExpr != nil && n.AExpr.GetKind() == parser.A_Expr_Kind_AEXPR_OP {
			lExpr := n.AExpr.GetLexpr()
			rExpr := n.AExpr.GetRexpr()
			if hasColumnOperate(lExpr, false) || hasColumnOperate(rExpr, false) {
				return true
			}
			if hasColumnFunction(lExpr, false) || hasColumnFunction(rExpr, false) {
				return true
			}
		}
	case *parser.Node_SelectStmt:
		stmt := n.SelectStmt
		if where := stmt.WhereClause; where != nil {
			if expr := where.GetAExpr(); expr != nil {
				if le := expr.GetLexpr(); le != nil {
					if containsExpression(le) {
						return true
					}
				}
				if re := expr.GetRexpr(); re != nil {
					if containsExpression(re) {
						return true
					}
				}
			}
			if boolExpr := where.GetBoolExpr(); boolExpr != nil {
				return loopHandleArgsForOrdinaryColumn(boolExpr.Args)
			}
		}
	}
	return false
}

// 规则43方法：循环处理参数
func loopHandleArgsForOrdinaryColumn(args []*parser.Node) bool {
	for _, arg := range args {
		if containsExpression(arg) {
			return true
		}
	}
	return false
}

// 规则43方法：判断是否字段操作
func hasColumnOperate(expr *parser.Node, isExpressionOp bool) bool {
	if expr != nil && expr.GetAExpr() != nil && expr.GetAExpr().GetKind() == parser.A_Expr_Kind_AEXPR_OP {
		lExpr := expr.GetAExpr().GetLexpr()
		rExpr := expr.GetAExpr().GetRexpr()
		if hasColumnOperate(lExpr, true) || hasColumnOperate(rExpr, true) {
			return true
		}
	}
	return isExpressionOp && expr != nil && expr.GetColumnRef() != nil && len(expr.GetColumnRef().GetFields()) > 0
}

// 规则43方法：判断字段上是否有函数
func hasColumnFunction(expr *parser.Node, isFucCallOp bool) bool {
	if expr != nil && expr.GetFuncCall() != nil && len(expr.GetFuncCall().GetArgs()) > 0 {
		args := expr.GetFuncCall().GetArgs()
		isColumn := false
		for _, arg := range args {
			if arg.GetColumnRef() != nil {
				isColumn = true
				break
			}
			if arg.GetFuncCall() != nil {
				return hasColumnFunction(arg, true)
			}
		}
		if isColumn {
			return true
		}
	}
	return false
}

func rule44(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		// 表个数1个和0个不存在表连接
		if len(subQuery.FromClause) <= 1 {
			continue
		}

		tableColumnMap := make(map[string][]string)
		fromClause := subQuery.FromClause
		for _, from := range fromClause {
			if from.GetRangeVar() != nil {
				schemaName := from.GetRangeVar().GetSchemaname()
				tableName := from.GetRangeVar().GetRelname()
				tableNameAlias := from.GetRangeVar().GetAlias()
				if len(schemaName) == 0 {
					schemaName = currentSchemaName
				}
				columns := pgContext.GetTableColumns(schemaName, tableName)
				// 从数据库中获取表对应的列名列表
				if len(columns) == 0 {
					// 获取表对应列
					columns, err = utils.GetTableColumns(pgContext.Executor, schemaName, tableName)
					if err != nil {
						return "", err
					}
				}
				if tableNameAlias != nil {
					tableColumnMap[tableNameAlias.GetAliasname()] = columns
				} else {
					tableColumnMap[tableName] = columns
				}
			}
		}

		if findDifferentTableColumnOrCondition(subQuery.WhereClause, tableColumnMap) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

// 规则44方法：找到不同表列使用or连接的方法
func findDifferentTableColumnOrCondition(node *parser.Node, tableColumnMap map[string][]string) bool {
	if node == nil || node.Node == nil {
		return false
	}
	switch n := node.Node.(type) {
	case *parser.Node_BoolExpr:
		if n.BoolExpr.Boolop == parser.BoolExprType_OR_EXPR {
			differentTableMap := make(map[string]string)
			args := n.BoolExpr.GetArgs()
			for _, arg := range args {
				if arg == nil || arg.GetAExpr() == nil {
					continue
				}
				var fields []*parser.Node
				if arg.GetAExpr().GetLexpr() != nil && arg.GetAExpr().GetLexpr().GetColumnRef() != nil {
					fields = arg.GetAExpr().GetLexpr().GetColumnRef().GetFields()
				}
				if arg.GetAExpr().GetRexpr() != nil && arg.GetAExpr().GetRexpr().GetColumnRef() != nil {
					fields = arg.GetAExpr().GetRexpr().GetColumnRef().GetFields()
				}
				if len(fields) == 0 {
					continue
				}
				field := fields[0].GetString_().GetSval()
				if len(fields) == 1 {
					for key, columns := range tableColumnMap {
						for _, column := range columns {
							// 和数据库中获取的表字段相等时，获取来自数据库的tableColumnMap中的表名
							if field == column {
								if validateDifferentTableForOrCondition(differentTableMap, key) {
									return true
								}
							}
						}
					}
				} else {
					if validateDifferentTableForOrCondition(differentTableMap, field) {
						return true
					}
				}
			}
		}
	}
	return false
}

// 规则44方法：validateDifferentTableForOrCondition
func validateDifferentTableForOrCondition(differentTableMap map[string]string, key string) bool {
	if len(differentTableMap) == 0 {
		differentTableMap[key] = key
	} else {
		_, ok := differentTableMap[key]
		if !ok {
			return true
		}
	}
	return false
}

func rule45(ctx context.Context, rule *driverV2.Rule, rawSql string, nextSQL []string) (string, error) {
	nodes, err := pkgParser.ParseSQL(rawSql)
	if err != nil {
		return "", err
	}
	if len(nodes) <= 0 {
		return "", fmt.Errorf("can not find parse tree from SQL")
	}

	// filter out IURD
	switch nodes[0].GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
	case *parser.Node_DeleteStmt:
	case *parser.Node_InsertStmt:
	case *parser.Node_UpdateStmt:
	default:
		return "", nil
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	if pgContext.UsingType == UsingTypeOffline {
		return "", nil
	}
	e := pgContext.Executor

	jsonQueryPlans, err := getExplainResult(pgContext, rawSql, e)
	if err != nil {
		return "", err
	}

	if result := checkFullTableScan(jsonQueryPlans); result {
		return RuleHandlerMap[rule.Name].Message, nil
	}
	return "", nil
}

func checkFullTableScan(queryPlans *[]PlanType) bool {
	for _, queryPlan := range *queryPlans {
		result := checkPlanForFullTableScan(queryPlan)
		if result {
			return true
		}
	}
	return false
}

func checkPlanForFullTableScan(plan PlanType) bool {
	switch plan.NodeType {
	case "Seq Scan":
		return true
	default:
		subPlans := plan.Plans
		if subPlans != nil && len(*subPlans) > 0 {
			// 递归调用检查子计划
			result := checkFullTableScan(subPlans)
			if result {
				return true
			}
		}
	}
	return false
}

func rule47(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:

		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		if stmt.UpdateStmt.WhereClause != nil && stmt.UpdateStmt.WhereClause.GetSubLink() == nil {
			if findWhereNotQuery(stmt.UpdateStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause != nil && stmt.DeleteStmt.WhereClause.GetSubLink() == nil {
			if findWhereNotQuery(stmt.DeleteStmt.WhereClause) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		if findWhereNotQuery(subQuery.WhereClause) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if subQuery.GetLarg() != nil {
			if findWhereNotQuery(subQuery.GetLarg().GetWhereClause()) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		if subQuery.GetRarg() != nil {
			if findWhereNotQuery(subQuery.GetRarg().GetWhereClause()) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}
	return "", nil
}

// 规则47方法：查找where条件中的!=、<>、NOT IN、NOT EXISTS、NOT LIKE以及NOT
func findWhereNotQuery(whereClause *parser.Node) bool {
	if whereClause == nil {
		return false
	}

	isNotOperate := func(names []*parser.Node) bool {
		for _, name := range names {
			if name != nil {
				switch name.GetString_().GetSval() {
				case "<>", "!~~":
					return true
				}
			}
		}
		return false
	}

	if whereClause.GetAExpr() != nil && isNotOperate(whereClause.GetAExpr().GetName()) {
		return true
	}

	if whereClause.GetBoolExpr() != nil && whereClause.GetBoolExpr().GetBoolop() == parser.BoolExprType_NOT_EXPR {
		return true
	}

	if whereClause.GetNullTest() != nil &&
		whereClause.GetNullTest().GetNulltesttype() == parser.NullTestType_IS_NOT_NULL {
		return true
	}
	return false
}

func rule48(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	return validateCreateTableMustField(rule, astSQL, "expect_created_time")
}

func validateCreateTableMustField(rule *driverV2.Rule, astSQL interface{}, mustFiled string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	expectCreatedTimeColumn := rule.Params.GetParam(mustFiled).String()
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		tableElts := stmt.CreateStmt.GetTableElts()
		isExistCreateTimeColumn := false
		for _, elt := range tableElts {
			colName := elt.GetColumnDef().GetColname()
			if strings.Compare(strings.ToLower(colName), strings.ToLower(expectCreatedTimeColumn)) == 0 {
				names := elt.GetColumnDef().GetTypeName().GetNames()
				for _, name := range names {
					if strings.ToUpper(name.GetString_().GetSval()) == "TIMESTAMP" {
						isExistCreateTimeColumn = true
						break
					}
				}
				if isExistCreateTimeColumn {
					break
				}
			}
		}
		if !isExistCreateTimeColumn {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

func rule49(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	return validateCreateTableMustField(rule, astSQL, "expect_modified_time")
}

func rule50(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	expectObjectNameCharacterMaxLength := rule.Params.GetParam("expect_object_name_character_max_length").Int()
	message := fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectObjectNameCharacterMaxLength)
	// 检查以下对象：表、函数、视图、序列、字段等
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		// 校验表长度
		tableName := stmt.CreateStmt.GetRelation().GetRelname()
		if len(tableName) > expectObjectNameCharacterMaxLength {
			return message, nil
		}
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			colName := elt.GetColumnDef().GetColname()
			// 校验列长度
			if len(colName) > expectObjectNameCharacterMaxLength {
				return message, nil
			}
		}
	case *parser.Node_AlterTableStmt:
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddColumn {
				// 校验列长度
				if len(cmdNode.AlterTableCmd.Def.GetColumnDef().GetColname()) > expectObjectNameCharacterMaxLength {
					return message, nil
				}
			}
			break
		}
	case *parser.Node_RenameStmt:
		newName := ""
		renameType := stmt.RenameStmt.RenameType
		switch renameType {
		case parser.ObjectType_OBJECT_TABLE, parser.ObjectType_OBJECT_COLUMN, parser.ObjectType_OBJECT_SEQUENCE:
			newName = stmt.RenameStmt.Newname
		}
		if len(newName) > expectObjectNameCharacterMaxLength {
			return message, nil
		}
	case *parser.Node_IndexStmt:
		indexName := stmt.IndexStmt.GetIdxname()
		if len(indexName) > expectObjectNameCharacterMaxLength {
			return message, nil
		}
	case *parser.Node_CreateFunctionStmt:
		// 校验函数长度
		funcNames := stmt.CreateFunctionStmt.Funcname
		for _, funcName := range funcNames {
			if len(funcName.GetString_().GetSval()) > expectObjectNameCharacterMaxLength {
				return message, nil
			}
		}
	case *parser.Node_AlterFunctionStmt:
		// 校验函数长度
		funcName := stmt.AlterFunctionStmt.Func.String()
		if len(funcName) > expectObjectNameCharacterMaxLength {
			return message, nil
		}
	case *parser.Node_ViewStmt:
		// 校验视图长度
		relName := stmt.ViewStmt.View.Relname
		if len(relName) > expectObjectNameCharacterMaxLength {
			return message, nil
		}
	case *parser.Node_CreateSeqStmt:
		// 校验序列长度
		relName := stmt.CreateSeqStmt.Sequence.Relname
		if len(relName) > expectObjectNameCharacterMaxLength {
			return message, nil
		}
	}

	return "", nil
}

func rule51(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	isCreateForeignKey := false
	isExistIndex := false
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		schemaNameCreate := stmt.CreateStmt.Relation.Schemaname
		tableNameCreate := stmt.CreateStmt.Relation.Relname
		fkColumns := make([]string, 0)
		indexColumns := make([]string, 0)
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			constraints := elt.GetColumnDef().GetConstraints()
			for _, constraint := range constraints {
				if constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_UNIQUE {
					indexColumns = append(indexColumns, elt.GetColumnDef().GetColname())
				}
			}
			if elt.GetConstraint() != nil && elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_FOREIGN {
				fkAttrs := elt.GetConstraint().GetFkAttrs()
				for _, fkAttr := range fkAttrs {
					fkColumns = append(fkColumns, fkAttr.GetString_().GetSval())
				}
				isCreateForeignKey = true
			}
			if elt.GetConstraint() != nil && elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE {
				keys := elt.GetConstraint().GetKeys()
				for _, key := range keys {
					indexColumns = append(indexColumns, key.GetString_().GetSval())
				}
			}
		}
		poses := make([]int, 0)
		for _, fkColumn := range fkColumns {
			for i, indexColumn := range indexColumns {
				if fkColumn == indexColumn {
					poses = append(poses, i)
					break
				}
			}
		}
		if len(fkColumns) > 0 && len(poses) == len(fkColumns) && sort.IntsAreSorted(poses) {
			return "", nil
		}

		// 获取nextSQL中是否有创建索引sql，有相关列创建sql也是校验通过，不走规则的
		isExistIndex, err = validateNextSqlIsExistIndexForForeignColumn(currentSchemaName, schemaNameCreate, tableNameCreate, fkColumns, nextSQL)
		if err != nil {
			return "", err
		}
	case *parser.Node_AlterTableStmt:
		schemaName := stmt.AlterTableStmt.Relation.Schemaname
		tableName := stmt.AlterTableStmt.Relation.Relname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
				if cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_FOREIGN {
					isCreateForeignKey = true
					fkAttrs := cmdNode.AlterTableCmd.GetDef().GetConstraint().GetFkAttrs()
					fkColumns := make([]string, 0)
					for _, fkAttr := range fkAttrs {
						fkColumns = append(fkColumns, fkAttr.GetString_().GetSval())
					}
					// 获取nextSQL中是否有创建索引sql，有相关列创建sql也是校验通过，不走规则的
					isExistIndex, err = validateNextSqlIsExistIndexForForeignColumn(currentSchemaName, schemaName, tableName, fkColumns, nextSQL)
					if err != nil {
						return "", err
					}
					if isExistIndex {
						return "", nil
					}
					if !isExistIndex {
						indexColumnsRows := pgContext.GetTableIndexColumns(schemaName, tableName)
						if len(indexColumnsRows) == 0 {
							indexColumnsRows, err = getIndexColumnsFromDb(schemaName, tableName, pgContext)
							if err != nil {
								return "", err
							}
						}
						for _, indexColumns := range indexColumnsRows {
							// 列在索引中的位置slice
							poses := make([]int, 0)
							splitColumns := strings.Split(indexColumns, ",")
							for _, column := range fkColumns {
								for i, splitColumn := range splitColumns {
									if column == splitColumn {
										poses = append(poses, i)
										break
									}
								}
							}
							if len(fkColumns) > 0 && len(poses) == len(fkColumns) && sort.IntsAreSorted(poses) {
								return "", err
							}
						}
					}
				}
			}
		}
	}

	if isCreateForeignKey && !isExistIndex {
		return RuleHandlerMap[rule.Name].Message, nil
	}
	return "", nil
}

// getIndexColumnsFromDb :从数据库中获取当前表的索引列信息
func getIndexColumnsFromDb(schemaName, tableName string, pgContext *PgContext) ([]string, error) {
	indexColumnsRows := make([]string, 0)
	if pgContext.Executor == nil {
		return indexColumnsRows, nil
	}
	indexesInfo, err := pgContext.GetTableIndexesInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	for _, indexInfo := range indexesInfo {
		indexDDL := indexInfo.IndexDDL
		start := strings.LastIndex(indexDDL, "(") + 1
		end := strings.LastIndex(indexDDL, ")")
		indexColumnsRows = append(indexColumnsRows, strings.Join(strings.Split(indexDDL[start:end], ","), ","))
	}
	return indexColumnsRows, nil
}

// 校验sql列表中是否存在外键列的创建索引
func validateNextSqlIsExistIndexForForeignColumn(currentSchema, schema, table string, columns []string, nextSQL []string) (bool, error) {
	for _, nextSql := range nextSQL {
		nodes, err := pkgParser.ParseSQL(nextSql)
		if err != nil {
			return false, fmt.Errorf("parse SQL failed: %v", err)
		}
		if len(nodes) <= 0 {
			continue
		}
		switch n := nodes[0].GetStmt().GetNode().(type) {
		case *parser.Node_IndexStmt:
			schemaName := n.IndexStmt.Relation.Schemaname
			tableName := n.IndexStmt.Relation.Relname
			indexParams := n.IndexStmt.IndexParams
			// 列在索引中的位置slice
			poses := make([]int, 0)
			for _, column := range columns {
				for i, indexParam := range indexParams {
					indexColumn := indexParam.GetIndexElem().GetName()
					if schemaName == schema && tableName == table && indexColumn == column {
						poses = append(poses, i)
						break
					}
				}
			}
			if len(columns) > 0 && len(poses) == len(columns) && sort.IntsAreSorted(poses) {
				return true, nil
			}
		case *parser.Node_AlterTableStmt:
			schemaName := n.AlterTableStmt.Relation.Schemaname
			tableName := n.AlterTableStmt.Relation.Relname
			if len(schemaName) == 0 {
				schemaName = currentSchema
			}
			for _, cmd := range n.AlterTableStmt.GetCmds() {
				cmdNode, ok := cmd.GetNode().(*parser.Node_AlterTableCmd)
				if !ok {
					continue
				}
				if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
					if cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE {
						keys := cmdNode.AlterTableCmd.GetDef().GetConstraint().GetKeys()
						// 列在索引中的位置slice
						poses := make([]int, 0)
						for _, column := range columns {
							for i, key := range keys {
								if schemaName == schema && tableName == table && key.GetString_().GetSval() == column {
									poses = append(poses, i)
									break
								}
							}
						}
						if len(columns) > 0 && len(poses) == len(columns) && sort.IntsAreSorted(poses) {
							return true, nil
						}
					}
				}
			}
		}
	}
	return false, nil
}

func rule52(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	expectIndexMinCellDivisionPercentage := rule.Params.GetParam("expect_index_min_cell_division_percentage").Int()
	message := fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectIndexMinCellDivisionPercentage)
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_IndexStmt:
		schemaName := stmt.IndexStmt.Relation.Schemaname
		tableName := stmt.IndexStmt.Relation.Relname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		indexParams := stmt.IndexStmt.IndexParams
		for _, indexParam := range indexParams {
			indexColumn := indexParam.GetIndexElem().GetName()
			selectivity, err := pgContext.ComputeIndexCellDivision(schemaName, tableName, indexColumn)
			if err != nil {
				return "", err
			}
			if selectivity < expectIndexMinCellDivisionPercentage {
				return message, nil
			}
		}
	}

	return "", nil
}

func rule53(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
		// 处理：insert into test(id,name) select id,name from test2 where id > 0;中的where条件
		if len(subQueries) == 1 {
			if subQueries[0].FromClause != nil && subQueries[0].WhereClause != nil {
				isPassValidate, err := isFirstIndexInWhere(currentSchemaName, subQueries[0], pgContext)
				if err != nil {
					return "", err
				}
				if !isPassValidate {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
			return "", nil
		}
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		if stmt.UpdateStmt.WhereClause != nil && stmt.UpdateStmt.WhereClause.GetSubLink() == nil {
			schemaName := stmt.UpdateStmt.Relation.Schemaname
			tableName := stmt.UpdateStmt.Relation.Relname
			if len(schemaName) == 0 {
				schemaName = currentSchemaName
			}
			whereClause := stmt.UpdateStmt.WhereClause
			isPassValidate, err := validateIndexFirstColumnInWhere(schemaName, tableName, whereClause, pgContext)
			if err != nil {
				return "", err
			}
			if !isPassValidate {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if stmt.DeleteStmt.WhereClause != nil && stmt.DeleteStmt.WhereClause.GetSubLink() == nil {
			schemaName := stmt.DeleteStmt.Relation.Schemaname
			tableName := stmt.DeleteStmt.Relation.Relname
			if len(schemaName) == 0 {
				schemaName = currentSchemaName
			}
			whereClause := stmt.DeleteStmt.WhereClause
			isPassValidate, err := validateIndexFirstColumnInWhere(schemaName, tableName, whereClause, pgContext)
			if err != nil {
				return "", err
			}
			if !isPassValidate {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		selectStmts := make([]*parser.SelectStmt, 0)
		if subQuery.WhereClause != nil {
			selectStmts = append(selectStmts, subQuery)
		}
		if subQuery.GetLarg() != nil {
			selectStmts = append(selectStmts, subQuery.GetLarg())
		}
		if subQuery.GetRarg() != nil {
			selectStmts = append(selectStmts, subQuery.GetRarg())
		}
		for _, selectStmt := range selectStmts {
			isPassValidate, err := isFirstIndexInWhere(currentSchemaName, selectStmt, pgContext)
			if err != nil {
				return "", err
			}
			if !isPassValidate {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}
	return "", nil
}

func isFirstIndexInWhere(currentSchemaName string, subQuery *parser.SelectStmt, pgContext *PgContext) (bool, error) {
	fromClause := subQuery.FromClause
	for _, from := range fromClause {
		schemaName, tableName := currentSchemaName, ""
		if from.GetRangeVar() != nil {
			schemaName = from.GetRangeVar().Schemaname
			if len(schemaName) == 0 {
				schemaName = currentSchemaName
			}
			tableName = from.GetRangeVar().Relname
		} else {
			if from.GetRangeSubselect() != nil && from.GetRangeSubselect().GetAlias() != nil {
				tableName = from.GetRangeSubselect().GetAlias().GetAliasname()
			}
		}
		if len(tableName) == 0 {
			continue
		}
		firstColumnInWhere, err := validateIndexFirstColumnInWhere(schemaName, tableName, subQuery.WhereClause, pgContext)
		if err != nil {
			return false, err
		}
		return firstColumnInWhere, nil
	}
	return true, nil
}

func validateIndexFirstColumnInWhere(schemaName, tableName string, whereClause *parser.Node, pgContext *PgContext) (bool, error) {
	indexColumnSlice, err := getIndexColumnListFromDb(schemaName, tableName, pgContext)
	if err != nil {
		return false, err
	}

	// 索引不存在不走规则
	if len(indexColumnSlice) == 0 {
		return true, nil
	}

	operates := whereClause.GetAExpr().GetName()
	fields := make([]*parser.Node, 0)
	if whereClause.GetAExpr() != nil {
		if whereClause.GetAExpr().GetLexpr() != nil && whereClause.GetAExpr().GetLexpr().GetColumnRef() != nil {
			fields = append(fields, whereClause.GetAExpr().GetLexpr().GetColumnRef().GetFields()...)
		}
		if whereClause.GetAExpr().GetRexpr() != nil && whereClause.GetAExpr().GetRexpr().GetColumnRef() != nil {
			fields = append(fields, whereClause.GetAExpr().GetRexpr().GetColumnRef().GetFields()...)
		}
	}
	if whereClause.GetBoolExpr() != nil {
		args := whereClause.GetBoolExpr().GetArgs()
		for _, arg := range args {
			if arg.GetAExpr() == nil {
				continue
			}
			if arg.GetAExpr().GetLexpr() != nil && arg.GetAExpr().GetLexpr().GetColumnRef() != nil {
				fields = append(fields, arg.GetAExpr().GetLexpr().GetColumnRef().GetFields()...)
			}
			if arg.GetAExpr().GetRexpr() != nil && arg.GetAExpr().GetRexpr().GetColumnRef() != nil {
				fields = append(fields, arg.GetAExpr().GetRexpr().GetColumnRef().GetFields()...)
			}
		}
	}
	for _, field := range fields {
		columnName := field.GetString_().GetSval()
		// 包含当前条件列的slice
		compositeIndexSlice := make([]string, 0)
		for _, indexColumns := range indexColumnSlice {
			indexColumnArray := strings.Split(indexColumns, ",")
			// 条件列是否存在联合索引中
			isInCompositeIndex := false
			for _, indexColumn := range indexColumnArray {
				if strings.TrimSpace(indexColumn) == columnName {
					isInCompositeIndex = true
					break
				}
			}
			if isInCompositeIndex {
				compositeIndexSlice = append(compositeIndexSlice, indexColumns)
			}
		}
		// 索引列不在查询条件中，走规则
		if len(compositeIndexSlice) == 0 {
			return false, nil
		}
		if len(compositeIndexSlice) > 0 {
			// 判断联合索引的第一个列是否在查询条件中
			isFirstColumnInWhere := false
			for _, compositeIndex := range compositeIndexSlice {
				indexColumnArray := strings.Split(compositeIndex, ",")
				for i, conditionField := range fields {
					// indexColumnArray[0]联合索引的第一列，比如：[column1,column2],column1是第一列
					if indexColumnArray[0] == conditionField.GetString_().GetSval() {
						// 条件列使用了范围查询，返回false，走规则
						if len(operates) != 0 && operates[i] != nil && operates[i].GetString_().GetSval() != "=" {
							return false, nil
						}
						isFirstColumnInWhere = true
						break
					}
				}
				if isFirstColumnInWhere {
					break
				}
			}
			if !isFirstColumnInWhere {
				return false, nil
			}
		}
	}
	return true, nil
}

// getIndexColumnListFromDb :从数据库中获取当前表的索引列信息
func getIndexColumnListFromDb(schemaName, tableName string, pgContext *PgContext) ([]string, error) {
	indexColumnSlice := make([]string, 0)
	if pgContext.Executor == nil {
		return indexColumnSlice, nil
	}
	indexesInfo, err := pgContext.GetTableIndexesInfo(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	for _, indexInfo := range indexesInfo {
		indexDDL := indexInfo.IndexDDL
		// 索引对应的列列表
		columnList := make([]string, 0)
		start := strings.LastIndex(indexDDL, "(") + 1
		end := strings.LastIndex(indexDDL, ")")
		columnList = append(columnList, strings.Split(indexDDL[start:end], ",")...)
		indexColumnSlice = append(indexColumnSlice, strings.Join(columnList, ","))
	}
	return indexColumnSlice, nil
}

func rule54(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateSchemaStmt:
		schemaName := stmt.CreateSchemaStmt.Schemaname
		if !validateObjectName(schemaName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateStmt:
		tableName := stmt.CreateStmt.Relation.Relname
		if !validateObjectName(tableName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			colName := elt.GetColumnDef().GetColname()
			if !validateObjectName(colName) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_CreateTableAsStmt:
		relName := stmt.CreateTableAsStmt.Into.Rel.Relname
		if !validateObjectName(relName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_AlterTableStmt:
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddColumn {
				if !validateObjectName(cmdNode.AlterTableCmd.Def.GetColumnDef().GetColname()) {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
			break
		}
	case *parser.Node_RenameStmt:
		newName := ""
		renameType := stmt.RenameStmt.RenameType
		switch renameType {
		case parser.ObjectType_OBJECT_TABLE, parser.ObjectType_OBJECT_COLUMN, parser.ObjectType_OBJECT_SEQUENCE:
			newName = stmt.RenameStmt.Newname
			if !validateObjectName(newName) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_CreateFunctionStmt:
		// 校验函数名
		funcNames := stmt.CreateFunctionStmt.Funcname
		for _, funcName := range funcNames {
			if !validateObjectName(funcName.GetString_().GetSval()) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_AlterFunctionStmt:
		// 校验函数名
		funcName := stmt.AlterFunctionStmt.Func.String()
		if !validateObjectName(funcName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_ViewStmt:
		// 校验视图名
		relName := stmt.ViewStmt.View.Relname
		if !validateObjectName(relName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_IndexStmt:
		// 校验create index索引名
		indexName := stmt.IndexStmt.GetIdxname()
		if !validateObjectName(indexName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateTrigStmt:
		// 校验触发器名
		triggerName := stmt.CreateTrigStmt.Trigname
		if !validateObjectName(triggerName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateTableSpaceStmt:
		// 校验表空间名
		tablespaceName := stmt.CreateTableSpaceStmt.Tablespacename
		if !validateObjectName(tablespaceName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	case *parser.Node_CreateSeqStmt:
		// 校验序列名
		relName := stmt.CreateSeqStmt.Sequence.Relname
		if !validateObjectName(relName) {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

// validateObjectName :校验对象名为：数据库对象名必须只包含小写字母，下划线，数字。且不能以数字开头
func validateObjectName(objectName string) bool {
	if len(objectName) == 0 {
		return true
	}
	// 正则表达式模式
	pattern := "^[a-z][a-z0-9_]*$"
	// 创建正则表达式对象
	reg := regexp.MustCompile(pattern)
	// 使用正则表达式匹配对象名
	if reg.MatchString(objectName) {
		return true
	} else {
		return false
	}
}

func rule55(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	expectBindVariableMaxNumber := rule.Params.GetParam("expect_bind_variable_max_number").Int()
	message := fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectBindVariableMaxNumber)

	actualBindVariableMaxNumber := 0
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		if len(subQueries) == 0 {
			actualBindVariableMaxNumber += getVariableNumberForTargetList(stmt.UpdateStmt.TargetList)
			actualBindVariableMaxNumber += getVariableNumberForWhereClause(stmt.UpdateStmt.WhereClause)
		}
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if len(subQueries) == 0 {
			actualBindVariableMaxNumber += getVariableNumberForWhereClause(stmt.DeleteStmt.WhereClause)
		}
	}

	for _, subQuery := range subQueries {
		actualBindVariableMaxNumber += getVariableNumberForTargetList(subQuery.TargetList)
		actualBindVariableMaxNumber += getVariableNumberForWhereClause(subQuery.WhereClause)
		if subQuery.GetLarg() != nil {
			actualBindVariableMaxNumber += getVariableNumberForWhereClause(subQuery.GetLarg().GetWhereClause())
		}
		if subQuery.GetRarg() != nil {
			actualBindVariableMaxNumber += getVariableNumberForWhereClause(subQuery.GetRarg().GetWhereClause())
		}
		valueLists := subQuery.GetValuesLists()
		for _, valueList := range valueLists {
			items := valueList.GetList().GetItems()
			for _, item := range items {
				if item.GetParamRef() != nil {
					actualBindVariableMaxNumber++
				}
			}
		}
	}

	if actualBindVariableMaxNumber > expectBindVariableMaxNumber {
		return message, nil
	}
	return "", nil
}

func getVariableNumberForTargetList(targetList []*parser.Node) int {
	actualBindVariableMaxNumber := 0
	for _, target := range targetList {
		paramRef := target.GetResTarget().GetVal().GetParamRef()
		if paramRef != nil {
			actualBindVariableMaxNumber++
		}
	}
	return actualBindVariableMaxNumber
}

func getVariableNumberForWhereClause(whereClause *parser.Node) int {
	actualBindVariableMaxNumber := 0
	if whereClause != nil && whereClause.GetBoolExpr() != nil {
		args := whereClause.GetBoolExpr().GetArgs()
		for _, arg := range args {
			lExpr, rExpr := arg.GetAExpr().GetLexpr(), arg.GetAExpr().GetRexpr()
			if lExpr.GetParamRef() != nil || rExpr.GetParamRef() != nil {
				actualBindVariableMaxNumber++
			}
		}
	}
	return actualBindVariableMaxNumber
}

func rule56(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	expectFixedPrefix := rule.Params.GetParam("expect_fixed_prefix").String()
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		relPersistence := stmt.CreateStmt.GetRelation().Relpersistence
		// t:临时表
		if relPersistence == "t" {
			tableName := stmt.CreateStmt.GetRelation().GetRelname()
			if !strings.HasPrefix(strings.ToLower(tableName), strings.ToLower(expectFixedPrefix)) {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectFixedPrefix), nil
			}
		}
	}

	return "", nil
}

func rule57(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}

	result, err := parser.Scan(sql)
	if err != nil {
		return "", nil
	}
	tokens := result.GetTokens()
	for _, token := range tokens {
		// keyword or reserved
		content := sql[token.Start:token.End]
		switch token.GetKeywordKind() {
		// select/create等属于KeywordKind_UNRESERVED_KEYWORD
		// insert/update/delete/alter/drop等属于KeywordKind_UNRESERVED_KEYWORD
		case parser.KeywordKind_RESERVED_KEYWORD, parser.KeywordKind_UNRESERVED_KEYWORD,
			parser.KeywordKind_TYPE_FUNC_NAME_KEYWORD, parser.KeywordKind_COL_NAME_KEYWORD:
			if isMixedCase(content) {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		case parser.KeywordKind_NO_KEYWORD:
			// 校验count/COUNT
			if strings.ToLower(content) == "count" {
				if isMixedCase(content) {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}
		}
	}

	return "", nil
}

// 规则57方法：校验字符串是否大小写混合，true:是;false:否
func isMixedCase(s string) bool {
	// 正则表达式匹配大小写混合的字符串，[a-z] 匹配小写字母，[A-Z] 匹配大写字母
	regex := regexp.MustCompile("[a-z]+[A-Z]+|[A-Z]+[a-z]+")
	return regex.MatchString(s)
}

func rule59(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	// 表名map，key=表名，value=列map(key=列名,value=列类型)TableColumnsInfo
	tableNameMap := make(map[string]map[string]string)
	// 表别名map，key=表别名，value=表名
	tableAliasMap := make(map[string]string)
	lExprList, rExprList := make([]*parser.Node, 0), make([]*parser.Node, 0)
	for _, subQuery := range subQueries {
		fromClauses := subQuery.FromClause
		for _, fromClause := range fromClauses {
			joinExpr := fromClause.GetJoinExpr()
			if joinExpr != nil {
				msg, err := recursiveSetTableMapForRule59(joinExpr, currentSchemaName, pgContext, tableNameMap, tableAliasMap)
				if err != nil {
					return msg, err
				}
			}
			lExprList, rExprList = getExprForRule59(fromClause.GetJoinExpr())
		}
	}

	for i, expr := range lExprList {
		lColumnType, rColumnType := "", ""
		lExpr, rExpr := expr, rExprList[i]
		if lExpr != nil {
			lFields := lExpr.GetColumnRef().GetFields()
			lColumnType = getColumnTypeForRule59(lFields, tableAliasMap, tableNameMap)
		}
		if rExpr != nil {
			rFields := rExpr.GetColumnRef().GetFields()
			rColumnType = getColumnTypeForRule59(rFields, tableAliasMap, tableNameMap)
		}

		if lColumnType != rColumnType {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}

	return "", nil
}

func recursiveSetTableMapForRule59(joinExpr *parser.JoinExpr, currentSchemaName string, pgContext *PgContext, tableNameMap map[string]map[string]string, tableAliasMap map[string]string) (string, error) {
	lArg, rArg := joinExpr.GetLarg(), joinExpr.GetRarg()
	if lArg != nil && lArg.GetRangeVar() != nil {
		msg, err := setMapForRule59(lArg, currentSchemaName, pgContext, tableNameMap, tableAliasMap)
		if err != nil {
			return msg, err
		}
	}
	if rArg != nil && rArg.GetRangeVar() != nil {
		msg, err := setMapForRule59(rArg, currentSchemaName, pgContext, tableNameMap, tableAliasMap)
		if err != nil {
			return msg, err
		}
	}
	if lArg.GetJoinExpr() != nil {
		msg, err := recursiveSetTableMapForRule59(lArg.GetJoinExpr(), currentSchemaName, pgContext, tableNameMap, tableAliasMap)
		if err != nil {
			return msg, err
		}
	}
	if rArg.GetJoinExpr() != nil {
		msg, err := recursiveSetTableMapForRule59(rArg.GetJoinExpr(), currentSchemaName, pgContext, tableNameMap, tableAliasMap)
		if err != nil {
			return msg, err
		}
	}
	return "", nil
}

func setMapForRule59(arg *parser.Node, currentSchemaName string, pgContext *PgContext, tableNameMap map[string]map[string]string, tableAliasMap map[string]string) (string, error) {
	tableMap, aliasMap, err := setTableMapForRule59(arg.GetRangeVar(), currentSchemaName, pgContext)
	if err != nil {
		return "", err
	}
	for key, value := range tableMap {
		tableNameMap[key] = value
	}
	for key, value := range aliasMap {
		tableAliasMap[key] = value
	}
	return "", nil
}

func setTableMapForRule59(rangeVar *parser.RangeVar, currentSchemaName string, pgContext *PgContext) (
	map[string]map[string]string, map[string]string, error) {
	tableNameMap := make(map[string]map[string]string)
	tableAliasMap := make(map[string]string)
	if rangeVar == nil {
		return tableNameMap, tableAliasMap, nil
	}
	schemaName := rangeVar.Schemaname
	tableName := rangeVar.Relname
	aliasName := ""
	alias := rangeVar.Alias
	if len(schemaName) == 0 {
		schemaName = currentSchemaName
	}
	if alias != nil {
		aliasName = alias.GetAliasname()
		tableAliasMap[aliasName] = tableName
	}
	columnMap := make(map[string]string)
	if _, ok := pgContext.DatabaseInfo.SchemaInfoMap[schemaName]; ok {
		tableInfoList := pgContext.DatabaseInfo.SchemaInfoMap[schemaName].TableInfoList
		for _, tableInfo := range tableInfoList {
			if tableInfo.TableName != tableName {
				continue
			}
			columnInfoList := tableInfo.ColumnInfoList
			for _, columnInfo := range columnInfoList {
				if _, isExist := columnMap[columnInfo.ColumnName]; !isExist {
					columnMap[columnInfo.ColumnName] = columnInfo.ColumnType
				}
			}
		}
	}
	columnsInfoList, err := pgContext.GetTableColumnsInfo(schemaName, tableName)
	if err != nil {
		return tableNameMap, tableAliasMap, err
	}
	for _, columnInfo := range columnsInfoList {
		columnMap[columnInfo.ColumnName] = columnInfo.ColumnType
	}
	tableNameMap[tableName] = columnMap
	return tableNameMap, tableAliasMap, nil
}

func getColumnTypeForRule59(fields []*parser.Node, tableAliasMap map[string]string,
	tableNameMap map[string]map[string]string) string {
	columnType := ""
	// fields长度为2,是有表别名；长度为1,是没有表别名
	if len(fields) == 2 {
		tableAliasName := fields[0].GetString_().GetSval()
		tableName, ok := tableAliasMap[tableAliasName]
		if ok {
			columnMap, ok := tableNameMap[tableName]
			if ok {
				columnTypeDb, ok := columnMap[fields[1].GetString_().GetSval()]
				if ok {
					columnType = columnTypeDb
				}
			}
		}
	} else if len(fields) == 1 {
		columnName := fields[0].GetString_().GetSval()
		for _, columnMapValue := range tableNameMap {
			for columnNameKey, columnTypeValue := range columnMapValue {
				if columnName == columnNameKey {
					columnType = columnTypeValue
					break
				}
			}
		}
	}
	return columnType
}

func getExprForRule59(joinExpr *parser.JoinExpr) ([]*parser.Node, []*parser.Node) {
	lExprList, rExprList := make([]*parser.Node, 0), make([]*parser.Node, 0)
	if joinExpr == nil {
		return lExprList, rExprList
	}
	lExpr, rExpr := joinExpr.GetQuals().GetAExpr().GetLexpr(), joinExpr.GetQuals().GetAExpr().GetRexpr()
	if lExpr != nil {
		lExprList = append(lExprList, lExpr)
	}
	if rExpr != nil {
		rExprList = append(rExprList, rExpr)
	}
	if joinExpr.GetLarg() != nil {
		if joinExpr.GetLarg().GetJoinExpr() != nil {
			lExprListSub, rExprListSub := getExprForRule59(joinExpr.GetLarg().GetJoinExpr())
			if lExprListSub != nil {
				lExprList = append(lExprList, lExprListSub...)
			}
			if rExprListSub != nil {
				rExprList = append(rExprList, rExprListSub...)
			}
		}
	}
	if joinExpr.GetRarg() != nil {
		if joinExpr.GetRarg().GetJoinExpr() != nil {
			lExprListSub, rExprListSub := getExprForRule59(joinExpr.GetRarg().GetJoinExpr())
			if lExprListSub != nil {
				lExprList = append(lExprList, lExprListSub...)
			}
			if rExprListSub != nil {
				rExprList = append(rExprList, rExprListSub...)
			}
		}
	}
	if joinExpr.GetQuals() != nil && joinExpr.GetQuals().GetBoolExpr() != nil {
		args := joinExpr.GetQuals().GetBoolExpr().GetArgs()
		for _, arg := range args {
			lExpr, rExpr = arg.GetAExpr().GetLexpr(), arg.GetAExpr().GetRexpr()
			if lExpr != nil {
				lExprList = append(lExprList, lExpr)
			}
			if rExpr != nil {
				rExprList = append(rExprList, rExpr)
			}
		}
	}
	return lExprList, rExprList
}

func rule60(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateFunctionStmt:
		if !stmt.CreateFunctionStmt.IsProcedure {
			// create function xxx 不是create or replace function xxx
			if !stmt.CreateFunctionStmt.Replace {
				return RuleHandlerMap[rule.Name].Message, nil
			}
			schemaName, functionName := currentSchemaName, ""
			funcName := stmt.CreateFunctionStmt.Funcname
			parameters := stmt.CreateFunctionStmt.Parameters
			if len(funcName) == 2 {
				schemaName = funcName[0].GetString_().GetSval()
				functionName = funcName[1].GetString_().GetSval()
			} else {
				functionName = funcName[0].GetString_().GetSval()
			}

			// 校验无参自定义函数
			if parameters == nil {
				result, err := pgContext.ValidateFunctionExistInDb(pgContext.CurrentDatabase, schemaName, functionName)
				if err != nil {
					return "", err
				}
				if !result {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}

			var data []map[string]sql.NullString
			// 校验有参自定义函数
			data, err = pgContext.GetFunctionInfoList(pgContext.CurrentDatabase, schemaName, functionName)
			if err != nil {
				return "", err
			}
			if len(data) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}

			parameterTypes := make([]string, 0)
			for _, parameter := range parameters {
				var parameterType = ""
				if len(parameter.GetFunctionParameter().GetArgType().GetNames()) == 2 {
					parameterType = parameter.GetFunctionParameter().GetArgType().GetNames()[1].GetString_().GetSval()
				} else {
					parameterType = parameter.GetFunctionParameter().GetArgType().GetNames()[0].GetString_().GetSval()
				}
				if strings.HasPrefix(parameterType, "int") {
					parameterType = "integer"
				} else if strings.HasPrefix(parameterType, "varchar") ||
					strings.HasPrefix(parameterType, "char") {
					parameterType = "character varying"
				} else if strings.ToLower(parameterType) == "bpchar" {
					parameterType = "character"
				} else if strings.HasPrefix(parameterType, "float8") {
					parameterType = "double precision"
				}
				parameterTypes = append(parameterTypes, parameterType)
			}
			parametersStr := ""
			if len(parameterTypes) > 0 {
				parametersStr = strings.ReplaceAll(strings.Join(parameterTypes, ","), " ", "")
			}

			isMatchParameterType := false
			for _, record := range data {
				specificSchema, ok := record["specific_schema"]
				if !ok {
					continue
				}
				specificName, ok := record["specific_name"]
				if !ok {
					continue
				}
				parameters, ok := record["parameters"]
				if !ok {
					continue
				}
				dbParameterStr := strings.ReplaceAll(parameters.String, " ", "")
				if strings.ToLower(schemaName) == strings.ToLower(specificSchema.String) &&
					strings.ToLower(functionName) == strings.ToLower(specificName.String) &&
					strings.ToLower(parametersStr) == strings.ToLower(dbParameterStr) {
					isMatchParameterType = true
					break
				}
			}
			if !isMatchParameterType {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

func rule61(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateFunctionStmt:
		if stmt.CreateFunctionStmt.IsProcedure {
			// create procedure xxx 不是create or replace procedure xxx 直接触发规则
			if !stmt.CreateFunctionStmt.Replace {
				return RuleHandlerMap[rule.Name].Message, nil
			}
			schemaName, functionName := currentSchemaName, ""
			funcName := stmt.CreateFunctionStmt.Funcname
			parameters := stmt.CreateFunctionStmt.Parameters
			if len(funcName) == 2 {
				schemaName = funcName[0].GetString_().GetSval()
				functionName = funcName[1].GetString_().GetSval()
			} else {
				functionName = funcName[0].GetString_().GetSval()
			}
			if parameters == nil {
				result, err := pgContext.ValidateProcedureExistInDb(pgContext.CurrentDatabase, schemaName, functionName)
				if err != nil {
					return "", err
				}
				if !result {
					return RuleHandlerMap[rule.Name].Message, nil
				}
			}

			var procedureInfoList []map[string]sql.NullString
			// 从数据库查询存储过程信息列表
			procedureInfoList, err = pgContext.GetProcedureInfoList(pgContext.CurrentDatabase, schemaName, functionName)
			if err != nil {
				return "", err
			}
			if len(procedureInfoList) == 0 {
				return RuleHandlerMap[rule.Name].Message, nil
			}

			parameterTypes := make([]string, 0)
			parameterModes := make([]string, 0)
			for _, parameter := range parameters {
				var parameterType = ""
				if len(parameter.GetFunctionParameter().GetArgType().GetNames()) == 2 {
					parameterType = parameter.GetFunctionParameter().GetArgType().GetNames()[1].GetString_().GetSval()
				} else {
					parameterType = parameter.GetFunctionParameter().GetArgType().GetNames()[0].GetString_().GetSval()
				}
				if strings.HasPrefix(parameterType, "int") {
					parameterType = "integer"
				} else if strings.HasPrefix(parameterType, "varchar") ||
					strings.HasPrefix(parameterType, "char") {
					parameterType = "character varying"
				} else if strings.ToLower(parameterType) == "bpchar" {
					parameterType = "character"
				} else if strings.HasPrefix(parameterType, "float8") {
					parameterType = "double precision"
				}
				parameterTypes = append(parameterTypes, parameterType)
				parameterMode :=
					strings.TrimPrefix(parameter.GetFunctionParameter().GetMode().String(), "FUNC_PARAM_")
				parameterModes = append(parameterModes, parameterMode)
			}

			parameterTypeStr := ""
			if len(parameterTypes) > 0 {
				parameterTypeStr = strings.ReplaceAll(strings.Join(parameterTypes, ","), " ", "")
			}
			// IN/OUT/INOUT
			parameterModeStr := ""
			if len(parameterModes) > 0 {
				parameterModeStr = strings.Join(parameterModes, ",")
			}

			isMatchParameterType := false
			for _, record := range procedureInfoList {
				specificSchema, ok := record["specific_schema"]
				if !ok {
					continue
				}
				specificName, ok := record["specific_name"]
				if !ok {
					continue
				}
				parameters, ok := record["parameters"]
				if !ok {
					continue
				}
				parametersMode, ok := record["parameters_mode"]
				if !ok {
					continue
				}
				dbParameterStr := strings.ReplaceAll(parameters.String, " ", "")
				dbParameterModeStr := strings.ReplaceAll(parametersMode.String, " ", "")
				// 参数类型和参数模式都匹配才是同一个存储过程
				if strings.ToLower(schemaName) == strings.ToLower(specificSchema.String) &&
					strings.ToLower(functionName) == strings.ToLower(specificName.String) &&
					strings.ToLower(parameterTypeStr) == strings.ToLower(dbParameterStr) &&
					strings.ToLower(parameterModeStr) == strings.ToLower(dbParameterModeStr) {
					isMatchParameterType = true
					break
				}
			}
			if !isMatchParameterType {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	}

	return "", nil
}

func rule62(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	expectMaxIndexNumber := rule.Params.GetParam("expect_max_index_number").Int()
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		tableElts := stmt.CreateStmt.GetTableElts()
		uniqueIndexMap := make(map[string]string)
		for _, elt := range tableElts {
			constraints := elt.GetColumnDef().GetConstraints()
			for _, constraint := range constraints {
				if constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_UNIQUE ||
					constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_PRIMARY {
					uniqueIndexMap[elt.GetColumnDef().GetColname()] = elt.GetColumnDef().GetColname()
				}
			}
			if elt.GetConstraint() != nil && (elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE ||
				elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_PRIMARY) {
				indexColumns := make([]string, 0)
				keys := elt.GetConstraint().GetKeys()
				for _, key := range keys {
					indexColumns = append(indexColumns, key.GetString_().GetSval())
				}
				indexColumnsStr := strings.Join(indexColumns, ",")
				uniqueIndexMap[indexColumnsStr] = indexColumnsStr
			}
		}
		if len(uniqueIndexMap) > expectMaxIndexNumber {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectMaxIndexNumber), nil
		}
	case *parser.Node_AlterTableStmt:
		schemaName := stmt.AlterTableStmt.Relation.Schemaname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		tableName := stmt.AlterTableStmt.Relation.Relname
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
				if cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE ||
					cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_PRIMARY {
					indexCount, err := getIndexCountForRule62(pgContext, schemaName, tableName)
					if err != nil {
						return "", err
					}
					if indexCount+1 > expectMaxIndexNumber {
						return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectMaxIndexNumber), nil
					}
				}
			}
		}
	case *parser.Node_IndexStmt:
		schemaName := stmt.IndexStmt.Relation.Schemaname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		tableName := stmt.IndexStmt.Relation.Relname
		indexCount, err := getIndexCountForRule62(pgContext, schemaName, tableName)
		if err != nil {
			return "", err
		}
		if indexCount+1 > expectMaxIndexNumber {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectMaxIndexNumber), nil
		}
	}

	return "", nil
}

func getIndexCountForRule62(pgContext *PgContext, schemaName, tableName string) (int, error) {
	indexCount := 0
	isExist, err := pgContext.IsExistTable(schemaName, tableName)
	if !isExist {
		return 0, err
	}
	schemaInfo, ok := pgContext.DatabaseInfo.SchemaInfoMap[schemaName]
	if !ok {
		return 0, fmt.Errorf("schema %s not exist", schemaName)
	}
	indexInfoList := schemaInfo.IndexInfoList
	for _, indexInfo := range indexInfoList {
		if indexInfo.TableName == tableName {
			indexCount++
		}
	}
	return indexCount, nil
}

func rule63(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	indexColumnsMap := make(map[string]string)
	notNullColumnsMap := make(map[string]string)
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			constraints := elt.GetColumnDef().GetConstraints()
			for _, constraint := range constraints {
				if constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_UNIQUE ||
					constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_PRIMARY {
					indexColumnsMap[elt.GetColumnDef().GetColname()] = elt.GetColumnDef().GetColname()
				}
				if constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_NOTNULL {
					notNullColumnsMap[elt.GetColumnDef().GetColname()] = elt.GetColumnDef().GetColname()
				}
			}

			if elt.GetConstraint() != nil && (elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE ||
				elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_PRIMARY) {
				keys := elt.GetConstraint().GetKeys()
				for _, key := range keys {
					indexColumnsMap[key.GetString_().GetSval()] = key.GetString_().GetSval()
				}
			}
		}

		for key := range indexColumnsMap {
			_, ok := notNullColumnsMap[key]
			if !ok {
				return RuleHandlerMap[rule.Name].Message, nil
			}
		}
	case *parser.Node_AlterTableStmt:
		schemaName := stmt.AlterTableStmt.Relation.Schemaname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		tableName := stmt.AlterTableStmt.Relation.Relname
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
				if cmdNode.AlterTableCmd.GetDef() != nil && cmdNode.AlterTableCmd.GetDef().GetConstraint() != nil &&
					cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE ||
					cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_PRIMARY {
					keys := cmdNode.AlterTableCmd.GetDef().GetConstraint().GetKeys()
					for _, key := range keys {
						indexColumnsMap[key.GetString_().GetSval()] = key.GetString_().GetSval()
					}
				}
			}
		}
		message, ok := validateNotNullIndexForRule63(indexColumnsMap, pgContext, schemaName, tableName, rule)
		if ok {
			return message, nil
		}
	case *parser.Node_IndexStmt:
		schemaName := stmt.IndexStmt.Relation.Schemaname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		tableName := stmt.IndexStmt.Relation.Relname
		indexParams := stmt.IndexStmt.IndexParams
		for _, indexParam := range indexParams {
			indexColumnsMap[indexParam.GetIndexElem().GetName()] = indexParam.GetIndexElem().GetName()
		}
		message, ok := validateNotNullIndexForRule63(indexColumnsMap, pgContext, schemaName, tableName, rule)
		if ok {
			return message, nil
		}
	}

	return "", nil
}

func validateNotNullIndexForRule63(indexColumnsMap map[string]string, pgContext *PgContext, schemaName string,
	tableName string, rule *driverV2.Rule) (string, bool) {
	if len(indexColumnsMap) > 0 {
		isExist, err := pgContext.IsExistTable(schemaName, tableName)
		if err != nil {
			return "", false
		}
		if isExist {
			schemaInfo, ok := pgContext.DatabaseInfo.SchemaInfoMap[schemaName]
			if ok {
				var currentTableInfo *TableInfo
				tableInfoList := schemaInfo.TableInfoList
				for _, tableInfo := range tableInfoList {
					if tableInfo.TableName == tableName {
						currentTableInfo = tableInfo
						break
					}
				}
				if currentTableInfo == nil {
					return "", true
				}
				columnInfoList := currentTableInfo.ColumnInfoList
				for columnName := range indexColumnsMap {
					isExistNotNullColumn := false
					for _, columnInfo := range columnInfoList {
						if columnName == columnInfo.ColumnName && !columnInfo.IsNullable {
							isExistNotNullColumn = true
							break
						}
					}
					if !isExistNotNullColumn {
						return RuleHandlerMap[rule.Name].Message, true
					}
				}
			}
		}
	}
	return "", false
}

func rule64(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	maxIndexCount := rule.Params.GetParam("max_index_count").Int()
	indexColumnsMap := make(map[string]int)
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			constraints := elt.GetColumnDef().GetConstraints()
			for _, constraint := range constraints {
				if constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_UNIQUE ||
					constraint.GetConstraint().Contype == parser.ConstrType_CONSTR_PRIMARY {
					count, ok := indexColumnsMap[elt.GetColumnDef().GetColname()]
					if ok {
						indexColumnsMap[elt.GetColumnDef().GetColname()] = count + 1
					} else {
						indexColumnsMap[elt.GetColumnDef().GetColname()] = 1
					}
				}
			}

			if elt.GetConstraint() != nil && (elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE ||
				elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_PRIMARY) {
				keys := elt.GetConstraint().GetKeys()
				for _, key := range keys {
					count, ok := indexColumnsMap[key.GetString_().GetSval()]
					if ok {
						indexColumnsMap[key.GetString_().GetSval()] = count + 1
					} else {
						indexColumnsMap[key.GetString_().GetSval()] = 1
					}
				}
			}
		}

		for _, val := range indexColumnsMap {
			if val > maxIndexCount {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxIndexCount), nil
			}
		}
	case *parser.Node_AlterTableStmt:
		schemaName := stmt.AlterTableStmt.Relation.Schemaname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		tableName := stmt.AlterTableStmt.Relation.Relname
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
				if cmdNode.AlterTableCmd.GetDef() != nil && cmdNode.AlterTableCmd.GetDef().GetConstraint() != nil &&
					cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE ||
					cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_PRIMARY {
					keys := cmdNode.AlterTableCmd.GetDef().GetConstraint().GetKeys()
					for _, key := range keys {
						count, ok := indexColumnsMap[key.GetString_().GetSval()]
						if ok {
							indexColumnsMap[key.GetString_().GetSval()] = count + 1
						} else {
							indexColumnsMap[key.GetString_().GetSval()] = 1
						}
					}
				}
			}
		}
		columnIndexCountMap, err := getCountInIndexForRule64(indexColumnsMap, pgContext, schemaName, tableName)
		if err != nil {
			return "", err
		}
		for _, columnInIndexCount := range columnIndexCountMap {
			if columnInIndexCount > maxIndexCount {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxIndexCount), nil
			}
		}
	case *parser.Node_IndexStmt:
		schemaName := stmt.IndexStmt.Relation.Schemaname
		if len(schemaName) == 0 {
			schemaName = currentSchemaName
		}
		tableName := stmt.IndexStmt.Relation.Relname
		indexParams := stmt.IndexStmt.IndexParams
		for _, indexParam := range indexParams {
			count, ok := indexColumnsMap[indexParam.GetIndexElem().GetName()]
			if ok {
				indexColumnsMap[indexParam.GetIndexElem().GetName()] = count + 1
			} else {
				indexColumnsMap[indexParam.GetIndexElem().GetName()] = 1
			}
		}
		columnIndexCountMap, err := getCountInIndexForRule64(indexColumnsMap, pgContext, schemaName, tableName)
		if err != nil {
			return "", err
		}
		for _, columnInIndexCount := range columnIndexCountMap {
			if columnInIndexCount > maxIndexCount {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxIndexCount), nil
			}
		}
	}

	return "", nil
}

func getCountInIndexForRule64(columnsMap map[string]int, pgCtx *PgContext, schema, tableName string) (map[string]int, error) {
	result := make(map[string]int)
	if len(columnsMap) == 0 {
		return result, nil
	}
	isExist, err := pgCtx.IsExistTable(schema, tableName)
	if err != nil {
		return result, err
	}
	if isExist {
		schemaInfo, ok := pgCtx.DatabaseInfo.SchemaInfoMap[schema]
		if !ok {
			return result, nil
		}
		indexInfoList := schemaInfo.IndexInfoList
		if len(indexInfoList) == 0 {
			return result, nil
		}
		for columnName, indexCount := range columnsMap {
			for _, indexInfo := range indexInfoList {
				if indexInfo.TableName != tableName {
					continue
				}
				columnList := indexInfo.ColumnList
				for _, columnStr := range columnList {
					if columnName == columnStr {
						indexCount += 1
						break
					}
				}
			}
			result[columnName] = indexCount
		}
	}
	return result, nil
}

func rule65(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		if subQuery == nil {
			continue
		}
		if subQuery.LimitCount != nil && subQuery.GetLimitOffset() != nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule66(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}

	// 对象名大小写混合的校验
	result, err := parser.Scan(sql)
	if err != nil {
		return "", err
	}

	tokens := result.GetTokens()
	// create or alter
	sqlType := ""
	objectTypeForCreate := ""
	objectTypeForAlter := ""
	objectTypeForReplace := ""
	for i, token := range tokens {
		if token.Token == parser.Token_CREATE {
			sqlType = "create"
		} else if token.Token == parser.Token_ALTER {
			sqlType = "alter"
		} else if token.Token == parser.Token_REPLACE {
			sqlType = "replace"
		}
		if sqlType == "create" {
			objectTypeForCreate = setCreateObjectTypeForRule66(token)
			if len(objectTypeForCreate) > 0 {
				break
			}
		} else if sqlType == "alter" {
			objectTypeForAlter = setAlterObjectTypeForRule66(token, tokens, i)
			if len(objectTypeForAlter) > 0 {
				break
			}
		} else if sqlType == "replace" {
			objectTypeForReplace = setReplaceObjectTypeForRule66(token)
			if len(objectTypeForReplace) > 0 {
				break
			}
		}
	}
	if sqlType != "create" && sqlType != "alter" && sqlType != "replace" {
		return "", nil
	}

	replaceObjectName := ""
	parameters := make([]string, 0)
	for i, token := range tokens {
		isNeedValid := (token.GetKeywordKind() == parser.KeywordKind_NO_KEYWORD && token.Token == parser.Token_IDENT) ||
			(token.GetKeywordKind() == parser.KeywordKind_UNRESERVED_KEYWORD && token.Token == parser.Token_NAME_P)
		if isNeedValid {
			if sqlType == "create" && len(objectTypeForCreate) > 0 {
				if objectTypeForCreate == "TABLE" || objectTypeForCreate == "VIEW" ||
					objectTypeForCreate == "FUNCTION" || objectTypeForCreate == "PROCEDURE" {
					// create table/view/function/procedure xxx as select * from test; as后的数据库对象不校验，break退出循环
					isHasBreakTag := false
					for j := i - 1; j > 0; j-- {
						if (objectTypeForCreate == "VIEW" && tokens[j].Token == parser.Token_AS) ||
							(objectTypeForCreate == "FUNCTION" && tokens[j].Token == parser.Token_RETURNS) ||
							(objectTypeForCreate == "PROCEDURE" && tokens[j].Token == parser.Token_LANGUAGE) {
							isHasBreakTag = true
							break
						}
					}
					if isHasBreakTag {
						break
					}
				}
				if token.Start > -1 && token.End > -1 {
					objectName := sql[token.Start:token.End]
					if isMixedCase(objectName) {
						return RuleHandlerMap[rule.Name].Message, nil
					}
				}
			}
			if sqlType == "alter" && len(objectTypeForAlter) > 0 {
				if tokens[i-1].Token.String() == objectTypeForAlter {
					if token.Start > -1 && token.End > -1 {
						objectName := sql[token.Start:token.End]
						if isMixedCase(objectName) {
							return RuleHandlerMap[rule.Name].Message, nil
						}
					}
				}
			}
		}
		// get objectName for:create or replace view/function/procedure
		res, objectName := getObjectNameForRule66(sqlType, objectTypeForReplace, token, sql)
		if !res {
			break
		}
		if len(objectName) > 0 {
			if objectName == "." {
				replaceObjectName = strings.TrimSuffix(replaceObjectName, ",")
				replaceObjectName += objectName
			} else {
				replaceObjectName += objectName + ","
			}
		}
	}

	replaceObjectName = strings.TrimSuffix(replaceObjectName, ",")
	if len(replaceObjectName) > 0 {
		objectArray := strings.Split(replaceObjectName, ",")
		replaceObjectName = objectArray[0]
		if len(objectArray) > 1 {
			parameters = append(parameters, objectArray[1:]...)
		}
	}

	// 不是create or replace直接返回
	if len(replaceObjectName) == 0 {
		return "", nil
	}

	// 校验create or replace view/function/procedure
	s, err, ok := validateCreateOrReplaceForRule66(ctx, rule, sql, objectTypeForReplace, replaceObjectName, parameters)
	if ok {
		return s, err
	}

	return "", nil
}

func getObjectNameForRule66(sqlType string, objectTypeForReplace string, token *parser.ScanToken, sql string) (bool, string) {
	objectName := ""
	if sqlType == "replace" && len(objectTypeForReplace) > 0 {
		// 遇到AS退出循环
		if token.Token == parser.Token_AS {
			return false, ""
		}
		// 包含schema
		if token.GetKeywordKind() == parser.KeywordKind_NO_KEYWORD && (token.Token == parser.Token_IDENT || token.Token == parser.Token_ASCII_46) {
			if token.Start > -1 && token.End > -1 {
				objectName += sql[token.Start:token.End]
			}
		}
	}
	return true, objectName
}

func validateCreateOrReplaceForRule66(ctx context.Context, rule *driverV2.Rule, sql string, objectTypeForReplace, objectName string, parameters []string) (string, error, bool) {
	node, err := parser.Parse(sql)
	if err != nil {
		return "", err, true
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err, true
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	schemaName := currentSchemaName
	parameterTypes := make([]string, 0)
	parameterModes := make([]string, 0)
	switch stmt := node.Stmts[0].GetStmt().GetNode().(type) {
	case *parser.Node_CreateFunctionStmt:
		// create function/procedure xxx 不是create or replace function/procedure xxx
		if !stmt.CreateFunctionStmt.Replace && isMixedCase(objectName) {
			return RuleHandlerMap[rule.Name].Message, nil, true
		}
		funcName := stmt.CreateFunctionStmt.Funcname
		parameters := stmt.CreateFunctionStmt.Parameters
		if len(funcName) == 2 {
			schemaName = funcName[0].GetString_().GetSval()
		}
		for _, parameter := range parameters {
			var parameterType = ""
			if len(parameter.GetFunctionParameter().GetArgType().GetNames()) == 2 {
				parameterType = parameter.GetFunctionParameter().GetArgType().GetNames()[1].GetString_().GetSval()
			} else {
				parameterType = parameter.GetFunctionParameter().GetArgType().GetNames()[0].GetString_().GetSval()
			}
			if strings.HasPrefix(parameterType, "int") {
				parameterType = "integer"
			} else if strings.HasPrefix(parameterType, "varchar") ||
				strings.HasPrefix(parameterType, "char") {
				parameterType = "character varying"
			} else if strings.ToLower(parameterType) == "bpchar" {
				parameterType = "character"
			} else if strings.HasPrefix(parameterType, "float8") {
				parameterType = "double precision"
			}
			parameterTypes = append(parameterTypes, parameterType)
		}
		// 存储过程有参数模式(IN/OUT/INOUT)
		if stmt.CreateFunctionStmt.IsProcedure {
			for _, parameter := range parameters {
				parameterMode := strings.TrimPrefix(parameter.GetFunctionParameter().GetMode().String(), "FUNC_PARAM_")
				parameterModes = append(parameterModes, parameterMode)
			}
		}
	}

	// 判断“视图/自定义函数/存储过程”是否存在，存在不触发规则，不存在触发规则校验
	switch objectTypeForReplace {
	case "VIEW":
		views, err := pgContext.GetViewListForSchema(schemaName)
		if err != nil {
			return "", err, true
		}
		viewName := objectName
		if strings.Contains(viewName, ".") {
			viewName = viewName[strings.Index(viewName, ".")+1:]
		}
		viewName = strings.ToLower(viewName)
		isExistView := false
		for _, view := range views {
			if strings.ToLower(view) == viewName {
				isExistView = true
				break
			}
		}
		if !isExistView && isMixedCase(objectName) {
			return RuleHandlerMap[rule.Name].Message, nil, true
		}
	case "FUNCTION":
		functionName := objectName
		if strings.Contains(functionName, ".") {
			functionName = functionName[strings.Index(functionName, ".")+1:]
		}
		functionName = strings.ToLower(functionName)
		// if:无参自定义函数;else:有参自定义函数
		if len(parameterTypes) == 0 {
			result, err := pgContext.ValidateFunctionExistInDb(pgContext.CurrentDatabase, schemaName, functionName)
			if err != nil {
				return "", err, true
			}
			if !result && isMixedCase(objectName) {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
		} else {
			isMatchParameterType, s, err := validateHasParameterFunctionForRule66(pgContext, schemaName, functionName, parameterTypes)
			if err != nil {
				return s, err, true
			}
			// 校验自定义函数名
			if !isMatchParameterType && isMixedCase(objectName) {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
			// 校验参数名
			if !isMatchParameterType {
				for _, parameter := range parameters {
					if isMixedCase(parameter) {
						return RuleHandlerMap[rule.Name].Message, nil, true
					}
				}
			}
		}
	case "PROCEDURE":
		procedureName := objectName
		if strings.Contains(procedureName, ".") {
			procedureName = procedureName[strings.Index(procedureName, ".")+1:]
		}
		procedureName = strings.ToLower(procedureName)
		// if:无参存储过错;else:有参存储过错
		if len(parameterTypes) == 0 {
			result, err := pgContext.ValidateProcedureExistInDb(pgContext.CurrentDatabase, schemaName, procedureName)
			if err != nil {
				return "", err, true
			}
			if !result && isMixedCase(objectName) {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
		} else {
			isMatchParameterTypeAndMode, s, err := validateHasParameterProcedureForRule66(pgContext, schemaName, procedureName, parameterTypes, parameterModes)
			if err != nil {
				return s, err, true
			}
			// 校验存储过程名
			if !isMatchParameterTypeAndMode && isMixedCase(objectName) {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
			// 校验参数名
			if !isMatchParameterTypeAndMode {
				for _, parameter := range parameters {
					if isMixedCase(parameter) {
						return RuleHandlerMap[rule.Name].Message, nil, true
					}
				}
			}
		}
	}
	return "", nil, false
}

func validateHasParameterFunctionForRule66(pgContext *PgContext, schemaName string, functionName string, parameterTypes []string) (bool, string, error) {
	data, err := pgContext.GetFunctionInfoList(pgContext.CurrentDatabase, schemaName, functionName)
	if err != nil {
		return false, "", err
	}
	parametersStr := ""
	if len(parameterTypes) > 0 {
		parametersStr = strings.ReplaceAll(strings.Join(parameterTypes, ","), " ", "")
	}
	isMatchParameterType := false
	for _, record := range data {
		parameters, ok := record["parameters"]
		if !ok {
			continue
		}
		dbParameterStr := strings.ReplaceAll(parameters.String, " ", "")
		if strings.ToLower(parametersStr) == strings.ToLower(dbParameterStr) {
			isMatchParameterType = true
			break
		}
	}
	return isMatchParameterType, "", nil
}

func validateHasParameterProcedureForRule66(pgContext *PgContext, schemaName string, procedureName string, parameterTypes []string, parameterModes []string) (bool, string, error) {
	data, err := pgContext.GetProcedureInfoList(pgContext.CurrentDatabase, schemaName, procedureName)
	if err != nil {
		return false, "", err
	}

	parameterTypeStr := ""
	if len(parameterTypes) > 0 {
		parameterTypeStr = strings.ReplaceAll(strings.Join(parameterTypes, ","), " ", "")
	}
	// IN/OUT/INOUT
	parameterModeStr := ""
	if len(parameterModes) > 0 {
		parameterModeStr = strings.Join(parameterModes, ",")
	}

	isMatchParameterTypeAndMode := false
	for _, record := range data {
		parameters, ok := record["parameters"]
		if !ok {
			continue
		}
		parametersMode, ok := record["parameters_mode"]
		if !ok {
			continue
		}
		dbParameterStr := strings.ReplaceAll(parameters.String, " ", "")
		dbParameterModeStr := strings.ReplaceAll(parametersMode.String, " ", "")
		// 参数类型和参数模式都匹配才是同一个存储过程
		if strings.ToLower(parameterTypeStr) == strings.ToLower(dbParameterStr) &&
			strings.ToLower(parameterModeStr) == strings.ToLower(dbParameterModeStr) {
			isMatchParameterTypeAndMode = true
			break
		}
	}
	return isMatchParameterTypeAndMode, "", nil
}

func setCreateObjectTypeForRule66(token *parser.ScanToken) string {
	objectTypeForCreate := ""
	switch token.Token {
	case parser.Token_SCHEMA:
		objectTypeForCreate = parser.Token_SCHEMA.String()
	case parser.Token_TABLE:
		objectTypeForCreate = parser.Token_TABLE.String()
	case parser.Token_VIEW:
		objectTypeForCreate = parser.Token_VIEW.String()
	case parser.Token_INDEX:
		objectTypeForCreate = parser.Token_INDEX.String()
	case parser.Token_SEQUENCE:
		objectTypeForCreate = parser.Token_SEQUENCE.String()
	case parser.Token_FUNCTION:
		objectTypeForCreate = parser.Token_FUNCTION.String()
	case parser.Token_PROCEDURE:
		objectTypeForCreate = parser.Token_PROCEDURE.String()
	case parser.Token_TRIGGER:
		objectTypeForCreate = parser.Token_TRIGGER.String()
	case parser.Token_TABLESPACE:
		objectTypeForCreate = parser.Token_TABLESPACE.String()
	}
	return objectTypeForCreate
}

func setAlterObjectTypeForRule66(token *parser.ScanToken, tokens []*parser.ScanToken, i int) string {
	objectTypeForAlter := ""
	switch token.Token {
	case parser.Token_RENAME, parser.Token_OWNER:
		objectTypeForAlter = "TO"
	case parser.Token_SET, parser.Token_ADD_P:
		objectTypeForAlter = tokens[i+1].Token.String()
	}
	return objectTypeForAlter
}

func setReplaceObjectTypeForRule66(token *parser.ScanToken) string {
	objectTypeForReplace := ""
	switch token.Token {
	case parser.Token_VIEW:
		objectTypeForReplace = parser.Token_VIEW.String()
	case parser.Token_FUNCTION:
		objectTypeForReplace = parser.Token_FUNCTION.String()
	case parser.Token_PROCEDURE:
		objectTypeForReplace = parser.Token_PROCEDURE.String()
	}
	return objectTypeForReplace
}

func rule67(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	expectFixedPrefix := rule.Params.GetParam("expect_fixed_prefix").String()
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			if elt.GetConstraint() != nil && elt.GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE {
				constraintName := elt.GetConstraint().GetConname()
				if !strings.HasPrefix(strings.ToLower(constraintName), strings.ToLower(expectFixedPrefix)) {
					return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectFixedPrefix), nil
				}
			}
		}
	case *parser.Node_AlterTableStmt:
		var cmdNode *parser.Node_AlterTableCmd
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok = cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddConstraint {
				if cmdNode.AlterTableCmd.GetDef() != nil && cmdNode.AlterTableCmd.GetDef().GetConstraint() != nil &&
					cmdNode.AlterTableCmd.GetDef().GetConstraint().GetContype() == parser.ConstrType_CONSTR_UNIQUE {
					constraintName := cmdNode.AlterTableCmd.GetDef().GetConstraint().GetConname()
					if !strings.HasPrefix(strings.ToLower(constraintName), strings.ToLower(expectFixedPrefix)) {
						return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectFixedPrefix), nil
					}
				}
			}
		}
	case *parser.Node_IndexStmt:
		if stmt.IndexStmt.Unique {
			idxName := stmt.IndexStmt.Idxname
			if !strings.HasPrefix(strings.ToLower(idxName), strings.ToLower(expectFixedPrefix)) {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectFixedPrefix), nil
			}
		}
	case *parser.Node_RenameStmt:
		renameType := stmt.RenameStmt.RenameType
		if renameType == parser.ObjectType_OBJECT_INDEX {
			schemaName := stmt.RenameStmt.Relation.Schemaname
			if len(schemaName) == 0 {
				schemaName = currentSchemaName
			}
			relIndexName := stmt.RenameStmt.Relation.Relname
			var hasValid = false
			// pgContext校验唯一索引前缀
			infoMap, ok := pgContext.DatabaseInfo.SchemaInfoMap[schemaName]
			if ok {
				indexInfoList := infoMap.IndexInfoList
				for _, indexInfo := range indexInfoList {
					if indexInfo.IndexName == strings.ToLower(relIndexName) {
						hasValid = true
						if !indexInfo.IsUnique {
							continue
						}
						if !strings.HasPrefix(strings.ToLower(stmt.RenameStmt.Newname), strings.ToLower(expectFixedPrefix)) {
							return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectFixedPrefix), nil
						}
					}
				}
			}
			if !hasValid {
				// 校验数据库中唯一索引前缀
				def, err := pgContext.GetIndexDef(schemaName, relIndexName)
				if err != nil {
					return "", err
				}
				if strings.HasPrefix(def, "CREATE UNIQUE INDEX") {
					if !strings.HasPrefix(strings.ToLower(stmt.RenameStmt.Newname), strings.ToLower(expectFixedPrefix)) {
						return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectFixedPrefix), nil
					}
				}
			}
		}
	}

	return "", nil
}

func rule68(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	maxInsertRows := rule.Params.GetParam("max_insert_rows").Int()
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_InsertStmt:
		selectStmt := stmt.InsertStmt.GetSelectStmt()
		if selectStmt.GetSelectStmt() == nil {
			return "", nil
		}
		// audit sql:insert into test.test(id,name) values (1,'aa'),(2,'bb');
		if len(selectStmt.GetSelectStmt().GetValuesLists()) > maxInsertRows {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxInsertRows), nil
		}
	}

	return "", nil
}

func rule69(ctx context.Context, rule *driverV2.Rule, rawSql string, nextSQL []string) (string, error) {
	nodes, err := pkgParser.ParseSQL(rawSql)
	if err != nil {
		return "", err
	}
	if len(nodes) <= 0 {
		return "", fmt.Errorf("can not find parse tree from SQL")
	}

	// filter out IURD
	switch nodes[0].GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
	case *parser.Node_DeleteStmt:
	case *parser.Node_InsertStmt:
	case *parser.Node_UpdateStmt:
	default:
		return "", nil
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	if pgContext.UsingType == UsingTypeOffline {
		return "", nil
	}
	e := pgContext.Executor

	jsonQueryPlans, err := getExplainResult(pgContext, rawSql, e)
	if err != nil {
		return "", err
	}

	if _, err = checkFullIndexScanForRule69(jsonQueryPlans); err != nil {
		return RuleHandlerMap[rule.Name].Message, nil
	}

	return "", nil
}

func checkFullIndexScanForRule69(jsonQueryPlans *[]PlanType) (bool, error) {
	for _, jsonQueryPlan := range *jsonQueryPlans {
		result, err := checkPlanForFullIndexScanForRule69(jsonQueryPlan)
		if err != nil {
			return true, err
		}
		if result {
			return false, nil
		}
	}
	return false, nil
}

func checkPlanForFullIndexScanForRule69(plan PlanType) (bool, error) {
	switch plan.NodeType {
	case "Index Scan":
		return true, fmt.Errorf("exists full inex scan")
	default:
		subPlans := plan.Plans
		if subPlans != nil && len(*subPlans) > 0 {
			// 递归调用检查子计划
			result, err := checkFullIndexScanForRule69(subPlans)
			if err != nil {
				return true, err
			}
			if result {
				return false, nil
			}
		}
	}
	return false, nil
}

func rule70(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	maxInParametersNumber := rule.Params.GetParam("max_in_parameters_number").Int()
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		whereClause := stmt.UpdateStmt.WhereClause
		if whereClause != nil {
			message, ok := validateInParametersNumberForRule70(whereClause, maxInParametersNumber, rule)
			if ok {
				return message, nil
			}
		}
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		whereClause := stmt.DeleteStmt.WhereClause
		if whereClause != nil {
			message, ok := validateInParametersNumberForRule70(whereClause, maxInParametersNumber, rule)
			if ok {
				return message, nil
			}
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		whereClause := subQuery.WhereClause
		message, ok := validateInParametersNumberForRule70(whereClause, maxInParametersNumber, rule)
		if ok {
			return message, nil
		}
	}
	return "", nil
}

func validateInParametersNumberForRule70(whereClause *parser.Node, maxInParametersNumber int,
	rule *driverV2.Rule) (string, bool) {
	if whereClause != nil && whereClause.GetAExpr() != nil &&
		whereClause.GetAExpr().GetKind() == parser.A_Expr_Kind_AEXPR_IN {
		lList, rList := whereClause.GetAExpr().GetLexpr().GetList(), whereClause.GetAExpr().GetRexpr().GetList()
		if lList != nil && len(lList.GetItems()) > maxInParametersNumber {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxInParametersNumber), true
		}
		if rList != nil && len(rList.GetItems()) > maxInParametersNumber {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxInParametersNumber), true
		}
	}
	return "", false
}

func rule71(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		if subQuery == nil {
			continue
		}
		if subQuery.GetOp() == parser.SetOperation_SETOP_UNION && !subQuery.GetAll() {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		lArg, rArg := subQuery.Larg, subQuery.Rarg
		if lArg != nil && lArg.GetOp() == parser.SetOperation_SETOP_UNION && !lArg.GetAll() {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		if rArg != nil && rArg.GetOp() == parser.SetOperation_SETOP_UNION && !rArg.GetAll() {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule72(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	specifiedFunctionsSet := rule.Params.GetParam("specified_functions_set").String()
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			if validateFunctionForRule72(list.GetResTarget().GetVal().GetFuncCall(), specifiedFunctionsSet) {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, specifiedFunctionsSet), nil
			}
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		targetList := subQuery.GetTargetList()
		for _, target := range targetList {
			if target.GetResTarget() != nil && target.GetResTarget().GetVal() != nil {
				funcCall := target.GetResTarget().GetVal().GetFuncCall()
				if validateFunctionForRule72(funcCall, specifiedFunctionsSet) {
					return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, specifiedFunctionsSet), nil
				}
			}
		}
		whereClause := subQuery.WhereClause
		if whereClause != nil {
			aExpr := whereClause.GetAExpr()
			if aExpr != nil {
				lExpr, rExpr := aExpr.GetLexpr(), aExpr.GetRexpr()
				if lExpr != nil {
					if validateFunctionForRule72(lExpr.GetFuncCall(), specifiedFunctionsSet) {
						return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, specifiedFunctionsSet), nil
					}
				}
				if rExpr != nil {
					if validateFunctionForRule72(rExpr.GetFuncCall(), specifiedFunctionsSet) {
						return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, specifiedFunctionsSet), nil
					}
				}
			}
		}
		havingClause := subQuery.HavingClause
		if havingClause != nil && havingClause.GetAExpr() != nil {
			lExpr, rExpr := havingClause.GetAExpr().GetLexpr(), havingClause.GetAExpr().GetRexpr()
			if lExpr != nil {
				if validateFunctionForRule72(lExpr.GetFuncCall(), specifiedFunctionsSet) {
					return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, specifiedFunctionsSet), nil
				}
			}
			if rExpr != nil {
				if validateFunctionForRule72(rExpr.GetFuncCall(), specifiedFunctionsSet) {
					return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, specifiedFunctionsSet), nil
				}
			}
		}
		sortClauses := subQuery.SortClause
		for _, sortClause := range sortClauses {
			if validateFunctionForRule72(sortClause.GetSortBy().GetNode().GetFuncCall(), specifiedFunctionsSet) {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, specifiedFunctionsSet), nil
			}
		}
	}
	return "", nil
}

func validateFunctionForRule72(funcCall *parser.FuncCall, specifiedFunctionsSet string) bool {
	if funcCall == nil {
		return false
	}
	specifiedFunctions := strings.Split(specifiedFunctionsSet, ",")
	funcNames := funcCall.GetFuncname()
	for _, funcName := range funcNames {
		functionName := funcName.GetString_().GetSval()
		for _, specifiedFunction := range specifiedFunctions {
			if strings.ToLower(functionName) == strings.ToLower(specifiedFunction) {
				return true
			}
		}
	}
	args := funcCall.GetArgs()
	if len(args) == 0 {
		return false
	}
	functions := getFunctionsFromArgsForRule72(args)
	for _, function := range functions {
		for _, specifiedFunction := range specifiedFunctions {
			if strings.ToLower(function) == strings.ToLower(specifiedFunction) {
				return true
			}
		}
	}
	return false
}

func getFunctionsFromArgsForRule72(args []*parser.Node) []string {
	if len(args) == 0 {
		return nil
	}
	functions := make([]string, 0)
	for _, arg := range args {
		funcNames := arg.GetFuncCall().GetFuncname()
		for _, funcName := range funcNames {
			functions = append(functions, funcName.GetString_().GetSval())
			if funcName.GetFuncCall() != nil && len(funcName.GetFuncCall().GetArgs()) > 0 {
				functions = append(functions, getFunctionsFromArgsForRule72(funcName.GetFuncCall().GetArgs())...)
			}
		}
	}
	return functions
}

func rule73(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	maxJoinNumber := rule.Params.GetParam("max_join_number").Int()
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		fromClause := subQuery.FromClause
		for _, from := range fromClause {
			if from == nil {
				continue
			}
			joinCount := getJoinCountForRule73(from.GetJoinExpr())
			// 实际join的表个数=join个数+1
			if joinCount > 0 {
				joinCount++
			}
			if joinCount > maxJoinNumber {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxJoinNumber), nil
			}
		}
	}
	return "", nil
}

func getJoinCountForRule73(joinExpr *parser.JoinExpr) int {
	joinCount := 0
	if joinExpr == nil {
		return 0
	}
	if joinExpr != nil {
		joinCount++
		if joinExpr.Larg != nil {
			subJoinCount := getJoinCountForRule73(joinExpr.Larg.GetJoinExpr())
			joinCount += subJoinCount
		}
		if joinExpr.Rarg != nil {
			subJoinCount := getJoinCountForRule73(joinExpr.Rarg.GetJoinExpr())
			joinCount += subJoinCount
		}
	}
	return joinCount
}

func rule74(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}

	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	expectLongFieldLength := rule.Params.GetParam("expect_long_field_length").Int()
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	// 表map: key=schema.tableName, value=map[columnName]columnName
	tableMap := make(map[string]map[string]string)
	// 表别名map: key=表别名, value=schema.tableName
	tableAliasMap := make(map[string]string)
	for _, subQuery := range subQueries {
		op := subQuery.Op
		// if: union sql else: single sql
		if op == parser.SetOperation_SETOP_UNION {
			lArg, rArg := subQuery.Larg, subQuery.Rarg
			message, err, done := validateSingleSqlForRule74(lArg, tableAliasMap, tableMap, currentSchemaName, pgContext, expectLongFieldLength, rule)
			if done {
				return message, err
			}
			message, err, done = validateSingleSqlForRule74(rArg, tableAliasMap, tableMap, currentSchemaName, pgContext, expectLongFieldLength, rule)
			if done {
				return message, err
			}
		} else {
			message, err, done := validateSingleSqlForRule74(subQuery, tableAliasMap, tableMap, currentSchemaName, pgContext, expectLongFieldLength, rule)
			if done {
				return message, err
			}
		}
	}
	return "", nil
}

func validateSingleSqlForRule74(subQuery *parser.SelectStmt, tableAliasMap map[string]string, tableMap map[string]map[string]string, currentSchemaName string, pgContext *PgContext,
	expectLongFieldLength int, rule *driverV2.Rule) (string, error, bool) {
	fromClause := subQuery.FromClause
	for _, from := range fromClause {
		tableAliasMap, tableMap = setTableLongColumnMapForRule74(from, currentSchemaName, pgContext, expectLongFieldLength)
	}
	// 处理distinct
	if len(subQuery.DistinctClause) > 0 {
		targetList := subQuery.TargetList
		for _, target := range targetList {
			columnRef := target.GetResTarget().GetVal().GetColumnRef()
			if columnRef == nil {
				funcCall := target.GetResTarget().GetVal().GetFuncCall()
				if funcCall == nil {
					continue
				}
				args := funcCall.GetArgs()
				for _, arg := range args {
					fields := arg.GetColumnRef().GetFields()
					if validateColumnLengthForRule74(fields, tableAliasMap, tableMap) {
						return RuleHandlerMap[rule.Name].Message, nil, true
					}
				}
			}
			fields := columnRef.GetFields()
			if validateColumnLengthForRule74(fields, tableAliasMap, tableMap) {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
		}
	}
	// 处理group by
	groupClause := subQuery.GroupClause
	for _, group := range groupClause {
		fields := group.GetColumnRef().GetFields()
		if validateColumnLengthForRule74(fields, tableAliasMap, tableMap) {
			return RuleHandlerMap[rule.Name].Message, nil, true
		}
	}
	// 处理order by
	sortClause := subQuery.SortClause
	for _, sort := range sortClause {
		fields := sort.GetSortBy().GetNode().GetColumnRef().GetFields()
		if validateColumnLengthForRule74(fields, tableAliasMap, tableMap) {
			return RuleHandlerMap[rule.Name].Message, nil, true
		}
	}
	return "", nil, false
}

func setTableLongColumnMapForRule74(from *parser.Node, currentSchemaName string, pgContext *PgContext, expectLongFieldLength int) (tableAliasMap map[string]string, tableMap map[string]map[string]string) {
	tableAliasMap = make(map[string]string)
	tableMap = make(map[string]map[string]string)
	rangeVar := from.GetRangeVar()
	if rangeVar == nil {
		if from.GetRangeSubselect() != nil {
			rangeVar = from.GetRangeSubselect().GetSubquery().GetRangeVar()
		}
	}
	if rangeVar == nil {
		return
	}
	schemaName, tableName := rangeVar.Schemaname, rangeVar.Relname
	if len(schemaName) == 0 {
		schemaName = currentSchemaName
	}
	realTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
	tableAlias := ""
	if rangeVar.Alias != nil {
		tableAlias = rangeVar.Alias.Aliasname
		tableAliasMap[tableAlias] = realTableName
	}
	pgContext.IsExistTable(schemaName, tableName)
	if _, ok := pgContext.DatabaseInfo.SchemaInfoMap[schemaName]; !ok {
		pgContext.DatabaseInfo.SchemaInfoMap[schemaName] = &SchemaInfo{
			SchemaName:    schemaName,
			TableInfoList: make([]*TableInfo, 0),
			IndexInfoList: make([]*IndexInfo, 0),
		}
	}
	tableInfoList := pgContext.DatabaseInfo.SchemaInfoMap[schemaName].TableInfoList
	for _, tableInfo := range tableInfoList {
		if tableInfo.TableName == tableName {
			columnsMap := make(map[string]string)
			columnInfoList := tableInfo.ColumnInfoList
			for _, columnInfo := range columnInfoList {
				if columnInfo.ColumnLength <= expectLongFieldLength {
					continue
				}
				columnsMap[columnInfo.ColumnName] = columnInfo.ColumnName
			}
			tableMap[realTableName] = columnsMap
		}
	}
	return
}

func validateColumnLengthForRule74(fields []*parser.Node, tableAliasMap map[string]string, tableMap map[string]map[string]string) bool {
	tableAlias := ""
	columnName := ""
	if len(fields) == 0 {
		return false
	}
	if len(fields) == 2 {
		tableAlias = fields[0].GetString_().GetSval()
		columnName = fields[1].GetString_().GetSval()
		tableName, ok := tableAliasMap[tableAlias]
		if ok {
			columnsMap, ok := tableMap[tableName]
			if ok {
				_, ok := columnsMap[columnName]
				if ok {
					return true
				}
			}
		}
	} else {
		columnName = fields[0].GetString_().GetSval()
		for _, columnsValue := range tableMap {
			for key := range columnsValue {
				if key == columnName {
					return true
				}
			}
		}
	}
	return false
}

func rule75(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	maxLimitRows := rule.Params.GetParam("max_rows").Int()
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		// 过滤sql：insert into test.test(id,name,age) values(1,'aa',100);
		if stmt.InsertStmt.SelectStmt.GetSelectStmt() != nil &&
			len(stmt.InsertStmt.SelectStmt.GetSelectStmt().GetValuesLists()) > 0 {
			return "", nil
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		limitCount := subQuery.LimitCount
		if limitCount == nil {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxLimitRows), nil
		}
		iVal := limitCount.GetAConst().GetIval().GetIval()
		if int(iVal) > maxLimitRows {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxLimitRows), nil
		}
	}
	return "", nil
}

func rule76(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		sortTypeMap := make(map[string]string)
		sortClause := subQuery.SortClause
		for _, clause := range sortClause {
			sortType := clause.GetSortBy().GetSortbyDir().String()
			if sortType == parser.SortByDir_SORTBY_DEFAULT.String() {
				sortType = parser.SortByDir_SORTBY_ASC.String()
			}
			if _, ok := sortTypeMap[sortType]; !ok {
				sortTypeMap[sortType] = sortType
			}
		}
		if len(sortTypeMap) > 1 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func rule77(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	selectColumnTargetList := make([]*parser.Node, 0)
	expectedSubQueryNestingLayers := rule.Params.GetParam("expected_subQuery_nesting_layers").Int()
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		targetList := stmt.SelectStmt.GetTargetList()
		for _, target := range targetList {
			if target != nil && target.GetResTarget() != nil &&
				target.GetResTarget().GetVal() != nil && target.GetResTarget().GetVal().GetSubLink() != nil &&
				target.GetResTarget().GetVal().GetSubLink().GetSubselect() != nil &&
				target.GetResTarget().GetVal().GetSubLink().GetSubselect().GetSelectStmt() != nil {
				selectColumnTargetList = append(selectColumnTargetList, target)
				// 从1开始计算：因为子查询嵌套的最内层的subLink为空没有计算在内，所以这里加1才符合实际
				var count = 1
				// 这里selectStmt不为空就不加1,因为select最外层不计算为子查询嵌套层次
				// 比如：select (select (select (select (select id from test.test limit 1))) from test.test limit 1) from test.test
				// 一共4层sql，这里子查询嵌套层次为3
				selectStmt := target.GetResTarget().GetVal().GetSubLink().GetSubselect().GetSelectStmt()
				count += computeSubQueryCountForRule77(selectStmt)
				if count > expectedSubQueryNestingLayers {
					return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedSubQueryNestingLayers), nil
				}
			}
		}
		whereClause := stmt.SelectStmt.WhereClause
		if whereClause != nil {
			result := validateWhereSubQueryForRule77(whereClause, expectedSubQueryNestingLayers)
			if result {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedSubQueryNestingLayers), nil
			}
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		whereClause := stmt.UpdateStmt.WhereClause
		if whereClause != nil {
			result := validateWhereSubQueryForRule77(whereClause, expectedSubQueryNestingLayers)
			if result {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedSubQueryNestingLayers), nil
			}
		}
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		whereClause := stmt.DeleteStmt.WhereClause
		if whereClause != nil {
			result := validateWhereSubQueryForRule77(whereClause, expectedSubQueryNestingLayers)
			if result {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedSubQueryNestingLayers), nil
			}
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		targetList := subQuery.GetTargetList()
		for _, target := range targetList {
			hasSelectColumnTarget := false
			for _, selectColumnTarget := range selectColumnTargetList {
				if target == selectColumnTarget {
					hasSelectColumnTarget = true
					break
				}
			}
			if hasSelectColumnTarget {
				continue
			}
			if target != nil && target.GetResTarget() != nil &&
				target.GetResTarget().GetVal() != nil && target.GetResTarget().GetVal().GetSubLink() != nil &&
				target.GetResTarget().GetVal().GetSubLink().GetSubselect() != nil &&
				target.GetResTarget().GetVal().GetSubLink().GetSubselect().GetSelectStmt() != nil {
				// 从1开始计算：因为子查询嵌套的最内层的subLink为空没有计算在内，所以这里加1才符合实际
				var count = 1
				selectStmt := target.GetResTarget().GetVal().GetSubLink().GetSubselect().GetSelectStmt()
				// 最外层子查询
				if selectStmt != nil {
					count++
				}
				count += computeSubQueryCountForRule77(selectStmt)
				if count > expectedSubQueryNestingLayers {
					return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedSubQueryNestingLayers), nil
				}
			}
		}
		fromClause := subQuery.FromClause
		for _, from := range fromClause {
			if from != nil && from.GetRangeSubselect() != nil && from.GetRangeSubselect().GetSubquery() != nil &&
				from.GetRangeSubselect().GetSubquery().GetSelectStmt() != nil {
				// 从1开始计算：因为子查询嵌套的最内层的subLink为空没有计算在内，所以这里加1才符合实际
				var count = 1
				count += computeSubQueryCountForRule77(from.GetRangeSubselect().GetSubquery().GetSelectStmt())
				if count > expectedSubQueryNestingLayers {
					return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedSubQueryNestingLayers), nil
				}
			}
		}
		whereClause := subQuery.WhereClause
		if whereClause != nil {
			result := validateWhereSubQueryForRule77(whereClause, expectedSubQueryNestingLayers)
			if result {
				return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, expectedSubQueryNestingLayers), nil
			}
		}
	}

	return "", nil
}

func validateWhereSubQueryForRule77(whereClause *parser.Node, expectedSubQueryLayers int) bool {
	if whereClause.GetRangeSubselect() != nil && whereClause.GetRangeSubselect().GetSubquery() != nil &&
		whereClause.GetRangeSubselect().GetSubquery().GetSelectStmt() != nil {
		// 从1开始计算：因为子查询嵌套的最内层的subLink为空没有计算在内，所以这里加1才符合实际
		count := 1
		count += computeSubQueryCountForRule77(whereClause.GetRangeSubselect().GetSubquery().GetSelectStmt())
		if count > expectedSubQueryLayers {
			return true
		}
	}
	if whereClause.GetSubLink() != nil && whereClause.GetSubLink().GetSubselect() != nil &&
		whereClause.GetSubLink().GetSubselect().GetSelectStmt() != nil {
		// 从1开始计算：因为子查询嵌套的最内层的subLink为空没有计算在内，所以这里加1才符合实际
		count := 1
		count += computeSubQueryCountForRule77(whereClause.GetSubLink().GetSubselect().GetSelectStmt())
		if count > expectedSubQueryLayers {
			return true
		}
	}
	return false
}

func computeSubQueryCountForRule77(selectStmt *parser.SelectStmt) int {
	count := 0
	if selectStmt == nil {
		return count
	}
	targetList := selectStmt.GetTargetList()
	for _, target := range targetList {
		subLink := target.GetResTarget().GetVal().GetSubLink()
		if subLink != nil {
			count++
			if subLink.GetSubselect() != nil && subLink.GetSubselect().GetSelectStmt() != nil {
				subCount := computeSubQueryCountForRule77(subLink.GetSubselect().GetSelectStmt())
				count += subCount
			}
		}
	}
	return count
}

func rule78(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	pgContext, err := getPgContextFromCtx(ctx)
	if err != nil {
		return "", err
	}
	currentSchemaName := pgContext.DatabaseInfo.CurrentSchema

	// 所有的where字句
	whereClauseList := make([]*parser.Node, 0)
	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		whereClause := stmt.UpdateStmt.WhereClause
		if whereClause != nil {
			whereClauseList = append(whereClauseList, whereClause)
		}
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		whereClause := stmt.DeleteStmt.WhereClause
		if whereClause != nil {
			whereClauseList = append(whereClauseList, whereClause)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	// 表名map，key=表名，value=列map(key=列名,value=列类型)TableColumnsInfo
	tableNameMap := make(map[string]map[string]string)
	// 表别名map，key=表别名，value=表名
	tableAliasMap := make(map[string]string)

	for _, subQuery := range subQueries {
		fromClauses := subQuery.FromClause
		for _, fromClause := range fromClauses {
			// 处理join:select t1.id,t2.id from table1 t1 join on table2 t2 on t1.id = t2.id
			joinExpr := fromClause.GetJoinExpr()
			if joinExpr != nil {
				msg, err := recursiveSetTableMapForRule59(joinExpr, currentSchemaName, pgContext, tableNameMap, tableAliasMap)
				if err != nil {
					return msg, err
				}
			}
			// 处理非join:select * from table1,table2
			msg, err := setMapForRule59(fromClause, currentSchemaName, pgContext, tableNameMap, tableAliasMap)
			if err != nil {
				return msg, err
			}
			// 处理子查询
			rangeSubSelect := fromClause.GetRangeSubselect()
			if rangeSubSelect != nil && rangeSubSelect.GetSubquery() != nil {
				aliasName := ""
				alias := rangeSubSelect.GetAlias()
				if alias != nil {
					aliasName = alias.GetAliasname()
				}
				selectStmt := rangeSubSelect.GetSubquery().GetSelectStmt()
				if selectStmt != nil {
					tableNames := make([]string, 0)
					clauses := selectStmt.FromClause
					for _, clause := range clauses {
						tableMap, _, err := setTableMapForRule59(clause.GetRangeVar(), currentSchemaName, pgContext)
						if err != nil {
							return "", err
						}
						for tableName := range tableMap {
							tableNames = append(tableNames, tableName)
						}
					}
					if len(tableNames) > 0 {
						tableAliasMap[aliasName] = strings.Join(tableNames, ",")
					}
				}
			}
		}
		whereClause := subQuery.WhereClause
		if whereClause != nil {
			whereClauseList = append(whereClauseList, whereClause)
		}
	}

	for _, whereClause := range whereClauseList {
		aExpr := whereClause.GetAExpr()
		msg, err, done := validateWhereColumnTypeSameForRule78(aExpr, tableAliasMap, tableNameMap, rule)
		if done {
			return msg, err
		}
		boolExpr := whereClause.GetBoolExpr()
		if boolExpr != nil {
			args := boolExpr.GetArgs()
			for _, arg := range args {
				aExpr = arg.GetAExpr()
				msg, err, done := validateWhereColumnTypeSameForRule78(aExpr, tableAliasMap, tableNameMap, rule)
				if done {
					return msg, err
				}
			}
		}
	}

	return "", nil
}

func getColumnTypeForRule78(fields []*parser.Node, tableAliasMap map[string]string,
	tableNameMap map[string]map[string]string) string {
	columnType := ""
	// fields长度为2,是有表别名；长度为1,是没有表别名
	if len(fields) == 2 {
		tableAliasName := fields[0].GetString_().GetSval()
		tableNames, ok := tableAliasMap[tableAliasName]
		if ok {
			tableNameArray := strings.Split(tableNames, ",")
			for _, tableName := range tableNameArray {
				columnMap, ok := tableNameMap[tableName]
				if ok {
					columnTypeDb, ok := columnMap[fields[1].GetString_().GetSval()]
					if ok {
						columnType = columnTypeDb
					}
				}
			}
		}
	} else if len(fields) == 1 {
		columnName := fields[0].GetString_().GetSval()
		for _, columnMapValue := range tableNameMap {
			for columnNameKey, columnTypeValue := range columnMapValue {
				if columnName == columnNameKey {
					columnType = columnTypeValue
					break
				}
			}
		}
	}
	if len(columnType) > 0 {
		// convert column type
		return convertDataTypeForRule78(columnType)
	}
	return columnType
}

func convertDataTypeForRule78(columnType string) string {
	realColumnType := ""
	switch columnType {
	case "smallint", "integer", "bigint":
		realColumnType = "integer"
	case "character", "character varying", "text":
		realColumnType = "string"
	case "real", "double_precision", "numeric":
		realColumnType = "float"
	default:
		realColumnType = ""
	}
	return realColumnType
}

func getValueTypeForRule78(val *parser.A_Const) string {
	if val == nil {
		return ""
	}

	valueType := ""
	if val.GetIval() != nil {
		valueType = "integer"
	} else if val.GetSval() != nil {
		valueType = "string"
	} else if val.GetFval() != nil {
		valueType = "float"
	} else {
		valueType = ""
	}

	return valueType
}

func validateWhereColumnTypeSameForRule78(aExpr *parser.A_Expr, tableAliasMap map[string]string,
	tableNameMap map[string]map[string]string, rule *driverV2.Rule) (string, error, bool) {
	if aExpr != nil {
		lExpr, rExpr := aExpr.GetLexpr(), aExpr.GetRexpr()
		lFields, rFields := make([]*parser.Node, 0), make([]*parser.Node, 0)
		lAConst, rAConst := &parser.A_Const{}, &parser.A_Const{}
		lColumnType, rColumnType := "", ""
		lValueType, rValueType := "", ""
		if lExpr != nil {
			lFields = lExpr.GetColumnRef().GetFields()
			lAConst = lExpr.GetAConst()
		}
		if rExpr != nil {
			rFields = rExpr.GetColumnRef().GetFields()
			rAConst = rExpr.GetAConst()
		}
		// where条件：column1 = column2
		if len(lFields) > 0 && len(rFields) > 0 {
			lColumnType = getColumnTypeForRule78(lFields, tableAliasMap, tableNameMap)
			rColumnType = getColumnTypeForRule78(rFields, tableAliasMap, tableNameMap)
			if lColumnType != rColumnType {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
		}
		// where条件：column1 = '1'
		if len(lFields) > 0 && rAConst != nil {
			lColumnType = getColumnTypeForRule78(lFields, tableAliasMap, tableNameMap)
			rValueType = getValueTypeForRule78(rAConst)
			if lColumnType != rValueType {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
		}
		// where条件：'1' = column2
		if lAConst != nil && len(rFields) > 0 {
			lValueType = getValueTypeForRule78(lAConst)
			rColumnType = getColumnTypeForRule78(rFields, tableAliasMap, tableNameMap)
			if lValueType != rColumnType {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
		}
		// where条件：1 = '1'
		if lAConst != nil && rAConst != nil {
			lValueType = getValueTypeForRule78(lAConst)
			rValueType = getValueTypeForRule78(rAConst)
			if lValueType != rValueType {
				return RuleHandlerMap[rule.Name].Message, nil, true
			}
		}
	}
	return "", nil, false
}

func rule79(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule is required")
	}
	node, ok := astSQL.(*parser.RawStmt)
	if !ok {
		return "", fmt.Errorf("invalid type of ast of rule[%v]", rule.Name)
	}

	var subQueries []*parser.SelectStmt
	switch stmt := node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(node.GetStmt())...)
	case *parser.Node_InsertStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.InsertStmt.SelectStmt)...)
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.InsertStmt.GetWithClause())...)
	case *parser.Node_UpdateStmt:
		fromClause := stmt.UpdateStmt.FromClause
		for _, from := range fromClause {
			subQueries = append(subQueries, *utils.ExtractSubQueries(from)...)
		}
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.UpdateStmt.WhereClause)...)
		targetList := stmt.UpdateStmt.TargetList
		for _, list := range targetList {
			subQueries = append(subQueries, *utils.ExtractSubQueries(list)...)
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.UpdateStmt.GetWithClause())...)
	case *parser.Node_DeleteStmt:
		subQueries = append(subQueries, *utils.ExtractSubQueries(stmt.DeleteStmt.WhereClause)...)
		if stmt.DeleteStmt.WhereClause == nil {
			return RuleHandlerMap[rule.Name].Message, nil
		}
		subQueries = append(subQueries, *utils.HandleWithClause(stmt.DeleteStmt.GetWithClause())...)
	}

	for _, subQuery := range subQueries {
		if subQuery.LimitCount != nil && len(subQuery.SortClause) == 0 {
			return RuleHandlerMap[rule.Name].Message, nil
		}
	}
	return "", nil
}

func setCurrentSchemaFromVariableSetStmt(ctx context.Context, stmt *parser.Node_VariableSetStmt) (currentSchemaName string, err error) {
	args := stmt.VariableSetStmt.GetArgs()
	for _, arg := range args {
		valNode := arg.Node.(*parser.Node_AConst)
		if valNode == nil {
			continue
		}
		currentSchemaName = valNode.AConst.GetSval().GetSval()
		if len(currentSchemaName) == 0 {
			continue
		}
		err := setCurrentSchemaToCtx(ctx, currentSchemaName)
		if err != nil {
			return "", err
		}
		break
	}
	return currentSchemaName, nil
}

func findCommentForColumns(ctx context.Context, targetSchemaName, currentSchemaName, targetTableName string, targetColumns []string, sqls []string) (columnsWithoutComment []string, err error) {
	if len(sqls) <= 0 {
		return targetColumns, nil
	}

	columnsWithComment := make(map[string]struct{})
	for _, sql := range sqls {
		nodes, err := pkgParser.ParseSQL(sql)
		if err != nil {
			return nil, fmt.Errorf("parse SQL failed: %v", err)
		}
		if len(nodes) <= 0 {
			continue
		}

		stmt, ok := nodes[0].GetStmt().GetNode().(*parser.Node_VariableSetStmt)
		if ok {
			currentSchemaName, err = setCurrentSchemaFromVariableSetStmt(ctx, stmt)
			continue
		}

		stmtComment, ok := nodes[0].GetStmt().GetNode().(*parser.Node_CommentStmt)
		if !ok {
			continue
		}

		if parser.ObjectType_OBJECT_COLUMN != stmtComment.CommentStmt.GetObjtype() {
			continue
		}

		items := stmtComment.CommentStmt.GetObject().GetList().GetItems()
		err = gatherCommentOfColumnsFromOneSql(ctx, targetSchemaName, currentSchemaName, targetTableName, columnsWithComment, items)
		if err != nil {
			return nil, fmt.Errorf("gatherCommentOfColumnsFromOneSql failed: %v", err)
		}
	}

	for _, col := range targetColumns {
		if _, ok := columnsWithComment[col]; !ok {
			columnsWithoutComment = append(columnsWithoutComment, col)
		}
	}

	return columnsWithoutComment, nil
}

func gatherCommentOfColumnsFromOneSql(ctx context.Context, targetSchemaName, currentSchemaName, targetTableName string, columnsWithComment map[string]struct{}, items []*parser.Node) error {
	//e.g. COMMENT ON COLUMN schema.table.column IS 'test'  // item[0] -- schema  item[1] -- table  item[2] -- column
	//e.g. COMMENT ON COLUMN table.column IS 'test'  // item[0] -- table  item[1] -- column
	var column *parser.Node_String_
	if len(items) == 3 { // 显式指定schema
		itemNamePt, ok := items[0].GetNode().(*parser.Node_String_)
		if !ok || itemNamePt == nil || itemNamePt.String_ == nil || itemNamePt.String_.GetSval() != targetSchemaName { // 匹配schema名
			return nil
		}

		itemNamePt, ok = items[1].GetNode().(*parser.Node_String_)
		if !ok || itemNamePt == nil || itemNamePt.String_ == nil || itemNamePt.String_.GetSval() != targetTableName { // 匹配表名
			return nil
		}

		column, ok = items[2].GetNode().(*parser.Node_String_)
		if !ok || column == nil || column.String_ == nil {
			return nil
		}

	} else if len(items) == 2 { // 没有显式指定schema
		itemNamePt, ok := items[0].GetNode().(*parser.Node_String_)
		if !ok || itemNamePt == nil || itemNamePt.String_ == nil || itemNamePt.String_.GetSval() != targetTableName { // 匹配表名
			return nil
		}

		// 没有显式指定schema，使用当前schema
		// 如果要匹配的schema不为空，且当前schema为空，说明没有获取过当前schema，需要获取当前schema
		if targetSchemaName != "" && currentSchemaName == "" {
			e, err := getExecutorFromCtx(ctx)
			if err != nil {
				return err
			}
			currentSchemaName, err = getCurrentSchema(e)
			if err != nil {
				return err
			}
			err = setCurrentSchemaToCtx(ctx, currentSchemaName)
			if err != nil {
				return err
			}
		}

		if targetSchemaName != currentSchemaName {
			return nil
		}

		column, ok = items[1].GetNode().(*parser.Node_String_)
		if !ok || column == nil || column.String_ == nil {
			return nil
		}
	} else {
		return nil
	}

	columnsWithComment[column.String_.GetSval()] = struct{}{}
	return nil
}

func getCurrentSchema(e *executor.Executor) (string, error) {
	return e.GetCurrentSchema()
}

func setCurrentSchemaToCtx(ctx context.Context, schema string) error {
	c, ok := ctx.Value(CtxKeyRuleHandlerCtx).(ruleHandlerContext)
	if !ok {
		return fmt.Errorf("cannot get executor from context")
	}
	// 设置上下文中当前schema
	c.PgContext.DatabaseInfo.CurrentSchema = schema
	if _, ok := c.PgContext.DatabaseInfo.SchemaInfoMap[schema]; !ok {
		schemaInfo := SchemaInfo{
			SchemaName:    schema,
			TableInfoList: make([]*TableInfo, 0),
			IndexInfoList: make([]*IndexInfo, 0),
		}
		c.PgContext.DatabaseInfo.SchemaInfoMap[schema] = &schemaInfo
	}
	if c.CurrentSchema == nil {
		c.CurrentSchema = &schema
		return nil
	}
	*c.CurrentSchema = schema
	return nil
}
