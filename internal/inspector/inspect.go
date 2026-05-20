package inspector

import (
	"context"
	"database/sql"
	_ "database/sql/driver"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/actiontech/dms/pkg/dms-common/i18nPkg"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	pkgParser "github.com/actiontech/sqle-pg-plugin/pkg/parser"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	hclog "github.com/hashicorp/go-hclog"
	_ "github.com/jackc/pgx/v4"
	parser "github.com/pganalyze/pg_query_go/v2"
)

const (
	defaultDatabase      = "postgres"
	defaultcurrentSchema = "public"
)

// pg data type map
var pgDataTypesMap = map[string]string{
	"bool":          "bool",
	"name":          "name",
	"int8":          "int8",
	"int2":          "int2",
	"int4":          "int4",
	"regproc":       "regproc",
	"text":          "text",
	"oid":           "oid",
	"float4":        "float4",
	"float8":        "float8",
	"money":         "money",
	"bpchar":        "bpchar",
	"varchar":       "varchar",
	"date":          "date",
	"time":          "time",
	"timestamp":     "timestamp",
	"timestamptz":   "timestamptz",
	"interval":      "interval",
	"timetz":        "timetz",
	"numeric":       "numeric",
	"regprocedure":  "regprocedure",
	"regoper":       "regoper",
	"regoperator":   "regoperator",
	"regclass":      "regclass",
	"regcollation":  "regcollation",
	"regtype":       "regtype",
	"regrole":       "regrole",
	"regnamespace":  "regnamespace",
	"regconfig":     "regconfig",
	"regdictionary": "regdictionary",
	"geometry":      "geometry",
}

var errNoConnection = fmt.Errorf("the driver hasn't been connected to database instance")
var errNoInitPgContext = fmt.Errorf("the pgContext hasn't been initialized")

type driverImpl struct {
	executor        *executor.Executor
	currentDatabase string
	currentSchema   string
	*driverPkg.DriverImpl

	// rollback config
	maxRollbackRowNumConf int
	isEnableRollbackConf  bool

	// 当前pg conn的进程id
	currentPgProcessId int

	// pg context
	pgContext *PgContext
	// 是否开启基础对象校验
	isEnableValidateBasicObject bool

	// cache for the current database metadata
	pkList         map[string] /*schema*/ map[string] /*table*/ []string /*primary key*/
	indexDef       map[string] /*schema*/ map[string] /*index*/ string   /*index def*/
	tableIndexDef  map[string] /*schema*/ map[string] /*table*/ []string /*index def*/
	tableColumnDef map[string] /*schema*/ map[string] /*table*/ []string /*column def*/
}

func NewDriverImpl(l hclog.Logger, dt driverPkg.Dialector, ah *driverPkg.AuditHandler, cfg *driverV2.Config) (driverV2.Driver, error) {
	di := &driverPkg.DriverImpl{
		Log:    l,
		Config: cfg,
		Ah:     ah,
		Dt:     dt,
	}
	var e *executor.Executor
	currentDatabase := ""
	currentSchema := ""
	currentPgProcessId := 0
	var pgContext *PgContext
	var err error
	if cfg.DSN != nil {
		inputDatabaseName := cfg.DSN.DatabaseName
		db, conn, err := dt.Open(cfg.DSN)
		if err != nil {
			return nil, err
		}
		di.DB = db     // will be closed by DriverImpl.Close
		di.Conn = conn // will be closed by DriverImpl.Close
		e, err = executor.NewExecutor(l, db, conn, cfg.DSN)
		if err != nil {
			return nil, err
		}

		// 获取当前Conn的pid，中止上线时使用
		pgPid, err := getConnPid(conn)
		if err != nil {
			return nil, err
		}
		currentPgProcessId = pgPid

		// 当前数据库
		currentDatabase = cfg.DSN.DatabaseName
		// 获取当前schema
		err = db.QueryRow("select current_schema()").Scan(&currentSchema)
		if err != nil {
			return nil, err
		}

		if len(inputDatabaseName) > 0 {
			// 在线上下文
			pgContext, err = NewPgContext(currentDatabase, currentSchema, e)
		} else {
			// 离线上下文
			pgContext, err = NewPgContext(defaultDatabase, defaultcurrentSchema, nil)
		}
		if err != nil {
			return nil, err
		}
	} else {
		// 离线上下文
		pgContext, err = NewPgContext(defaultDatabase, defaultcurrentSchema, nil)
		if err != nil {
			return nil, err
		}
	}

	maxRollbackRows := -1
	var isEnableRollback bool
	for _, rule := range cfg.Rules {
		if rule.Name == RuleId24 {
			maxRollbackRows = rule.Params.GetParam("max_affected_rows").Int()
		}
		if rule.Name == RuleId25 {
			isEnableRollback = true
		}
	}

	return &driverImpl{
		executor:                    e,
		currentDatabase:             currentDatabase,
		currentSchema:               currentSchema,
		DriverImpl:                  di,
		maxRollbackRowNumConf:       maxRollbackRows,
		isEnableRollbackConf:        isEnableRollback,
		currentPgProcessId:          currentPgProcessId,
		pgContext:                   pgContext,
		isEnableValidateBasicObject: true,
		pkList:                      make(map[string]map[string][]string),
		indexDef:                    make(map[string]map[string]string),
		tableIndexDef:               make(map[string]map[string][]string),
		tableColumnDef:              make(map[string]map[string][]string),
	}, nil
}

// getConnPid ： 获取当前Conn的pg对应的进程id
func getConnPid(conn *sql.Conn) (int, error) {
	var currentPgProcessId int
	// 创建一个新的上下文，设置超时时间为5秒
	dbConnCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := conn.QueryContext(dbConnCtx, "select pg_backend_pid()")
	if err != nil {
		return 0, err
	}
	if rows == nil {
		return 0, fmt.Errorf("get pg conn rows no data")
	}
	if rows.Next() {
		err = rows.Scan(&currentPgProcessId)
		if err != nil {
			return 0, err
		}
	}
	return currentPgProcessId, nil
}

func NewMockDriver(cfg *driverV2.Config, database, schema string, ah *driverPkg.AuditHandler, conn *executor.Executor) *driverImpl {
	d := &driverImpl{
		currentDatabase:             database,
		currentSchema:               schema,
		isEnableRollbackConf:        true,
		isEnableValidateBasicObject: true,
		maxRollbackRowNumConf:       -1,
		pkList:                      make(map[string]map[string][]string),
		indexDef:                    make(map[string]map[string]string),
		tableIndexDef:               make(map[string]map[string][]string),
		tableColumnDef:              make(map[string]map[string][]string),
	}
	d.DriverImpl = &driverPkg.DriverImpl{
		Log: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Trace,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
		Config: cfg,
		Ah:     ah,
		Dt:     nil,
		DB:     nil,
		Conn:   nil,
	}
	if conn != nil {
		d.executor = conn
	}
	schemaInfoMap := make(map[string]*SchemaInfo)
	schemaInfoMap[schema] = &SchemaInfo{
		SchemaName:    schema,
		TableInfoList: make([]*TableInfo, 0),
		IndexInfoList: make([]*IndexInfo, 0),
	}
	databaseInfo := &DatabaseInfo{
		DatabaseName:  database,
		CurrentSchema: schema,
		SchemaInfoMap: schemaInfoMap,
	}
	d.pgContext = &PgContext{
		CurrentDatabase:      database,
		Executor:             conn,
		DatabaseInfo:         databaseInfo,
		PgDbTypeNameMap:      pgDataTypesMap,
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
	}
	return d
}

func SqlParserFunc(sql string) (interface{}, error) {
	nodes, err := pkgParser.ParseSQL(sql)
	if err != nil {
		return nil, err
	}
	if len(nodes) <= 0 {
		return nil, fmt.Errorf("can not find parse tree from SQL")
	}
	return nodes[0], nil
}

// 校验postgresql数据类型
func validateDataType(ast *parser.RawStmt, dbTypeNameMap map[string]string) error {
	// 伪类型(数据库pg_type中不存在这些数据类型，但是ddl时可以使用的类型)
	pseudoTypes := []string{"bigserial", "decimal", "mood", "serial", "serial2", "serial4", "serial8", "smallserial", "int"}
	for _, dataType := range pseudoTypes {
		// 将伪类型加到dbTypeNameMap中,如果存在就跳过
		if _, ok := dbTypeNameMap[dataType]; !ok {
			dbTypeNameMap[dataType] = dataType
		}
	}

	var validateTypeNames = make([]string, 0)
	switch stmt := ast.GetStmt().GetNode().(type) {
	case *parser.Node_CreateStmt:
		tableElts := stmt.CreateStmt.GetTableElts()
		for _, elt := range tableElts {
			if elt.GetColumnDef() == nil {
				continue
			}
			names := elt.GetColumnDef().GetTypeName().Names
			typeName, err := getTypeName(names)
			if err != nil {
				return err
			}
			validateTypeNames = append(validateTypeNames, typeName)
		}
	case *parser.Node_AlterTableStmt:
		for _, cmd := range stmt.AlterTableStmt.GetCmds() {
			cmdNode, ok := cmd.GetNode().(*parser.Node_AlterTableCmd)
			if !ok {
				continue
			}
			// alter table test.test add column new_column varchar1(100);
			// ALTER TABLE test.test_alter_column_type ALTER COLUMN id TYPE VARCHAR(255) USING id::int;
			// 校验：ALTER COLUMN id TYPE VARCHAR(255)
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AddColumn ||
				cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AlterColumnType {
				colDef := cmdNode.AlterTableCmd.GetDef().GetColumnDef()
				names := colDef.GetTypeName().GetNames()
				typeName, err := getTypeName(names)
				if err != nil {
					return err
				}
				validateTypeNames = append(validateTypeNames, typeName)
			}
			// ALTER TABLE test.test_alter_column_type ALTER COLUMN id TYPE VARCHAR(255) USING id::int;
			// 校验：USING id::int
			if cmdNode.AlterTableCmd.GetSubtype() == parser.AlterTableType_AT_AlterColumnType {
				rawDefault := cmdNode.AlterTableCmd.GetDef().GetColumnDef().GetRawDefault()
				if rawDefault != nil && rawDefault.GetTypeCast() != nil {
					names := rawDefault.GetTypeCast().GetTypeName().GetNames()
					typeName, err := getTypeName(names)
					if err != nil {
						return err
					}
					validateTypeNames = append(validateTypeNames, typeName)
				}
			}
		}
	default:
		return nil
	}

	if len(validateTypeNames) > 0 {
		err := validateCreateOrAlterTableColumType(validateTypeNames, dbTypeNameMap)
		if err != nil {
			return fmt.Errorf("validate create or alter table column type error:%s", err)
		}
	}
	return nil
}

// 获取names对应的数据类型名
// names为列类型数组，如果长度为2,第一个元素值为：pg_catalog，第二个元素值为：数据类型,如：int4；如果长度为1,只有一个元素：值为数据类型
// create table时，names长度为2；alter table时，names长度为1.
// 长度为2的测试sql：CREATE TABLE my_schedule (id SERIAL PRIMARY KEY,day weekday,task VARCHAR(255));
func getTypeName(names []*parser.Node) (string, error) {
	columnDataType := ""
	if len(names) >= 2 {
		columnDataType = names[1].GetString_().GetStr()
	} else if len(names) == 1 {
		columnDataType = names[0].GetString_().GetStr()
	} else {
		return "", fmt.Errorf("unknown column data type:%v", names)
	}
	return strings.ToLower(columnDataType), nil
}

// 校验数据类型是否是postgresql的数据类型，包括：基础类型和自定义类型
// 成功返回：nil；失败时返回：不存在数据类型的字符串，用英文逗号拼接，例如：xxxx,yyyy
func validateCreateOrAlterTableColumType(typeNames []string, dbTypeNameMap map[string]string) error {
	errorMsg := make([]string, 0)
	for _, typeName := range typeNames {
		if _, ok := dbTypeNameMap[typeName]; !ok {
			errorMsg = append(errorMsg, typeName)
		}
	}
	if len(errorMsg) > 0 {
		return fmt.Errorf("postgresql not exists data type:%s", strings.Join(errorMsg, ","))
	}
	return nil
}

const (
	CtxKeyRuleHandlerCtx = "rule-handler-ctx"
)

type ruleHandlerContext struct {
	CurrentSchema *string
	Executor      *executor.Executor
	pgContext     *PgContext
}

func (c *ruleHandlerContext) GetExecutor() *executor.Executor {
	if c == nil {
		return nil
	}
	return c.Executor
}

func (c *ruleHandlerContext) GetCurrentSchema() string {
	if c == nil || c.CurrentSchema == nil {
		return ""
	}

	return *c.CurrentSchema
}

func (c *ruleHandlerContext) GetPgContext() *PgContext {
	if c == nil || c.pgContext == nil {
		return nil
	}
	return c.pgContext
}

/*
审核sql

	在审核SQL时，如果出现SQL审核的基础校验报错，则写日志并且跳过该条SQL的审核。
	若是审核规则报错，则仅跳过该规则。这样可以不阻塞流程。
	TODO 后续需要增加用户可感知的提示。
*/
func (p *driverImpl) Audit(ctx context.Context, sqls []string) ([]*driverV2.AuditResults, error) {
	var (
		err           error
		ast           interface{}
		result        *driverV2.AuditResult
		dbTypeNameMap map[string]string
	)
	results := make([]*driverV2.AuditResults, 0, len(sqls))
	if p.pgContext.UsingType == UsingTypeOnline {
		dbTypeNameMap, err = p.executor.GetDataTypeNameMap(nil, p.currentSchema)
		if err != nil {
			return results, err
		}
		// 将补充的PG数据类型字典合并到已有的数据类型字典中
		for key, value := range pgDataTypesMap {
			if _, exist := dbTypeNameMap[key]; !exist {
				dbTypeNameMap[key] = value
			}
		}
	} else {
		// 来自预定义的pg数据类型
		dbTypeNameMap = pgDataTypesMap
	}
	p.pgContext.PgDbTypeNameMap = dbTypeNameMap

	for i, auditSql := range sqls {
		ast, err = SqlParserFunc(auditSql)
		ruleResults := driverV2.NewAuditResults()
		if err != nil {
			p.Log.Error("parse auditSql failed auditSql %v err %v", auditSql, err)
			results = append(results, ruleResults)
			continue
		}
		err = validateDataType(ast.(*parser.RawStmt), dbTypeNameMap)
		if err != nil {
			p.Log.Error("validate auditSql failed auditSql %v err %v", auditSql, err)
			results = append(results, ruleResults)
			continue
		}

		// 校验基础对象
		if p.isEnableValidateBasicObject {
			validateBasicObject := ValidateBasicObject{PgContext: p.pgContext, RawStmt: ast.(*parser.RawStmt)}
			validateResult, validateErr := validateBasicObject.Validate()
			if validateErr != nil {
				p.Log.Error("validate auditSql failed auditSql %v err %v", auditSql, err)
				results = append(results, ruleResults)
				continue
			}
			if validateResult.Level != driverV2.RuleLevelNull {
				ruleResults.Results = append(ruleResults.Results, validateResult)
			}
		}

		for _, rule := range p.Config.Rules {
			ruleHandler := RuleHandlerMap[rule.Name]
			hasAstSQLHandler := ruleHandler.AstSQLHandler != nil
			hasRawSQLHandler := ruleHandler.RawSQLHandler != nil
			hasHandler := hasAstSQLHandler || hasRawSQLHandler

			isAllowOfflineRule := ruleHandler.IsAllowOfflineRule(ast.(*parser.RawStmt))
			hasExecutor := p.executor != nil
			if !hasHandler || (!isAllowOfflineRule && !hasExecutor) {
				continue
			}

			handlerCtx := ruleHandlerContext{
				CurrentSchema: &p.currentSchema,
				Executor:      p.executor,
				pgContext:     p.pgContext,
			}
			ctx = context.WithValue(ctx, CtxKeyRuleHandlerCtx, handlerCtx)
			result, err = p.Ah.Audit(ctx, rule, auditSql, sqls[i+1:])
			if err != nil {
				// Extract Desc from I18nRuleInfo for error logging
				descI18n := getRuleDescI18n(rule)
				log.Printf("规则[%s]审核错误:%s", rule.Name, err)
				ruleResults.AddResultWithError(rule.Level, rule.Name, err.Error(), true, descI18n)
				continue
			}
			if result.Level != "" {
				ruleResults.Results = append(ruleResults.Results, result)
			}
		}
		results = append(results, ruleResults)

		if p.isEnableValidateBasicObject {
			// 处理pg插件上下文
			handlePgContext := HandlePgContext{PgContext: p.pgContext, RawStmt: ast.(*parser.RawStmt)}
			err = handlePgContext.Handle()
			if err != nil {
				p.Log.Error("handle pg context failed auditSql %v err %v", auditSql, err)
				continue
			}
		}
	}
	return results, nil
}

func (i *driverImpl) Parse(ctx context.Context, sqlText string) ([]driverV2.Node, error) {
	nodes, err := pkgParser.ParseSQL(sqlText)
	if err != nil {
		return nil, err
	}
	ns := make([]driverV2.Node, 0, len(nodes))

	for _, n := range nodes {
		typ := sqlType(n.Stmt.GetNode())
		//sqlText, _ := parser.Deparse(&parser.ParseResult{Stmts: []*parser.RawStmt{n}})
		sqlContent := ""
		// 如果解析的sql语句没有分号结尾，这里的StmtLen=0
		if n.StmtLen == 0 {
			sqlContent = sqlText[n.StmtLocation:]
		} else {
			sqlContent = sqlText[n.StmtLocation : n.StmtLocation+n.StmtLen]
		}
		fingerprint, innerErr := pkgParser.Fingerprint(sqlContent)
		if innerErr != nil {
			return nil, innerErr
		}
		ns = append(ns, driverV2.Node{Text: sqlContent, Type: typ, Fingerprint: fingerprint})
	}
	return ns, nil
}

func sqlType(typ interface{}) string {
	switch typ.(type) {
	case *parser.Node_CreateStmt,
		*parser.Node_CreateTableAsStmt,
		*parser.Node_CreatedbStmt,
		*parser.Node_CreateRoleStmt,
		*parser.Node_CreateAmStmt,
		*parser.Node_DropStmt,
		*parser.Node_DropdbStmt,
		*parser.Node_DropRoleStmt,
		*parser.Node_DropTableSpaceStmt,
		*parser.Node_AlterDatabaseStmt,
		*parser.Node_AlterTableStmt,
		*parser.Node_IndexStmt,
		*parser.Node_TruncateStmt,
		*parser.Node_CommentStmt,
		*parser.Node_RenameStmt:
		return driverV2.SQLTypeDDL

	case *parser.Node_FieldSelect,
		*parser.Node_InsertStmt,
		*parser.Node_UpdateStmt,
		*parser.Node_DeleteStmt,
		*parser.Node_CallStmt,
		*parser.Node_ExplainStmt:
		return driverV2.SQLTypeDML
	case *parser.Node_SelectStmt:
		return driverV2.SQLTypeDQL
	}
	return ""
}

/*
[{
    "Plan": {
        "Plans": [{}]
	}
    "Planning": {
        "Local Hit Blocks": 0,
        "Temp Read Blocks": 0,
        "Local Read Blocks": 0,
        "Shared Hit Blocks": 0,
        "Shared Read Blocks": 0,
        "Temp Written Blocks": 0,
        "Local Dirtied Blocks": 0,
        "Local Written Blocks": 0,
        "Shared Dirtied Blocks": 0,
        "Shared Written Blocks": 0
    },
    "Triggers": [],
    "Planning Time": 0.0,
    "Execution Time": 0.0
}]
*/

type PlanType struct {
	NodeType            string      `json:"Node Type"`
	PlanRows            int64       `json:"Plan Rows"`
	PlanWidth           int64       `json:"Plan Width"`
	TotalCost           float64     `json:"Total Cost"`
	ActualRows          int64       `json:"Actual Rows"`
	ActualLoops         int64       `json:"Actual Loops"`
	StartupCost         float64     `json:"Startup Cost"`
	AsyncCapable        bool        `json:"Async Capable"`
	ParallelAware       bool        `json:"Parallel Aware"`
	SortKey             []string    `json:"Sort Key"`
	Output              []string    `json:"Output"`
	Alias               string      `json:"Alias"`
	Schema              string      `json:"Schema"`
	InnerUnique         bool        `json:"Inner Unique"`
	SubplansRemoved     int64       `json:"Subplans Removed"`
	PlannedPartitions   int64       `json:"Planned Partitions"`
	JoinType            string      `json:"Join Type"`
	HashCond            string      `json:"Hash Cond"`
	ParentRelationship  string      `json:"Parent Relationship"`
	RelationName        string      `json:"Relation Name"`
	Strategy            string      `json:"Strategy"`
	PartialMode         string      `json:"Partial Mode"`
	ScanDirection       string      `json:"Scan Direction"`
	IndexName           string      `json:"Index Name"`
	IndexCond           string      `json:"Index Cond"`
	GroupKey            []string    `json:"Group Key"`
	SortSpaceUsed       int64       `json:"Sort Space Used"`
	WorkersPlanned      int64       `json:"Workers Planned"`
	LocalHitBlocks      int64       `json:"Local Hit Blocks"`
	TempReadBlocks      int64       `json:"Temp Read Blocks"`
	WorkersLaunched     int64       `json:"Workers Launched"`
	ActualTotalTime     float64     `json:"Actual Total Time"`
	LocalReadBlocks     int64       `json:"Local Read Blocks"`
	SharedHitBlocks     int64       `json:"Shared Hit Blocks"`
	SharedReadBlocks    int64       `json:"Shared Read Blocks"`
	ActualStartupTime   float64     `json:"Actual Startup Time"`
	TempWrittenBlocks   int64       `json:"Temp Written Blocks"`
	LocalDirtiedBlocks  int64       `json:"Local Dirtied Blocks"`
	LocalWrittenBlocks  int64       `json:"Local Written Blocks"`
	SharedDirtiedBlocks int64       `json:"Shared Dirtied Blocks"`
	SharedWrittenBlocks int64       `json:"Shared Written Blocks"`
	Plans               *[]PlanType `json:"Plans"`
}

func (i *driverImpl) ExtractTableFromSQL(ctx context.Context, sql string) ([]*driverV2.Table, error) {
	// check sql
	if sql == "" {
		return nil, errors.New("the SQL should not be empty")
	}
	// only support dml
	if isDML, err := i.isDML(sql); err != nil {
		return nil, fmt.Errorf("the SQL is not DML: %v", err)
	} else if !isDML {
		return nil, driverV2.ErrSQLIsNotSupported
	}

	node, err := pkgParser.ParseSQL(sql)
	if err != nil {
		return nil, fmt.Errorf("parse SQL error: %v", err)
	}

	type schemaTable struct {
		Schema string
		Table  string
	}

	var schemaTables []schemaTable
	addSchemaTables := func(tables []schemaTable) {
		for _, table := range tables {
			if table.Schema == "" {
				table.Schema = "public"
			}
			schemaTables = append(schemaTables, table)
		}
	}

	var getMultiTables func(fromClauses []*parser.Node)
	getMultiTables = func(fromClauses []*parser.Node) {
		for _, t := range fromClauses {
			var selectStmt *parser.Node_SelectStmt
			subSelect, ok := t.GetNode().(*parser.Node_RangeSubselect)
			if ok && subSelect != nil {
				selectStmt, ok = subSelect.RangeSubselect.GetSubquery().GetNode().(*parser.Node_SelectStmt)
				if !ok || selectStmt == nil {
					continue
				}
				getMultiTables(selectStmt.SelectStmt.GetFromClause())
				continue
			}

			schema := t.GetRangeVar().GetSchemaname()
			if schema == "" {
				schema = "public"
			}
			schemaTables = append(schemaTables, schemaTable{
				Schema: schema,
				Table:  t.GetRangeVar().GetRelname(),
			})
		}
	}

	switch stmt := node[0].GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		if stmt.SelectStmt.Larg != nil && stmt.SelectStmt.Op == parser.SetOperation_SETOP_UNION {
			var names []schemaTable
			f := func(selectStmt *parser.SelectStmt) {
				if selectStmt.FromClause != nil {
					for _, node := range selectStmt.GetFromClause() {
						names = append(names, schemaTable{
							Schema: node.GetRangeVar().Schemaname,
							Table:  node.GetRangeVar().Relname,
						})
					}
				}
			}
			getUnionSchemaTable(f, stmt.SelectStmt)
			addSchemaTables(names)
		} else {
			if stmt.SelectStmt.GetFromClause() == nil {
				break
			}

			getMultiTables(stmt.SelectStmt.GetFromClause())
		}
	case *parser.Node_UpdateStmt:
		getMultiTables(stmt.UpdateStmt.GetFromClause())
	case *parser.Node_InsertStmt:
		addSchemaTables([]schemaTable{{stmt.InsertStmt.GetRelation().Schemaname, stmt.InsertStmt.GetRelation().Relname}})
	case *parser.Node_DeleteStmt:
		addSchemaTables([]schemaTable{{stmt.DeleteStmt.GetRelation().Schemaname, stmt.DeleteStmt.GetRelation().Relname}})
	case *parser.Node_VariableShowStmt:
		if stmt.VariableShowStmt != nil {
			addSchemaTables([]schemaTable{{"", stmt.VariableShowStmt.Name}})
		}
	default:
		return nil, fmt.Errorf("the sql is `%v`, we don't support analysing this sql", sql)
	}

	tables := make([]*driverV2.Table, len(schemaTables))
	for j, schemaTable := range schemaTables {
		tables[j] = &driverV2.Table{
			Name:   schemaTable.Table,
			Schema: schemaTable.Schema,
		}
	}
	return tables, nil
}

func (i *driverImpl) GetTableMeta(ctx context.Context, table *driverV2.Table) (*driverV2.TableMeta, error) {
	if i.executor == nil {
		return nil, errNoConnection
	}

	columnsInfo, err := i.getTableColumnsInfo(i.executor, table.Schema, table.Name)
	if err != nil {
		return nil, fmt.Errorf("get table columns info failed, err: %v", err)
	}

	indexesInfo, err := i.getTableIndexesInfo(i.executor, table.Schema, table.Name)
	if err != nil {
		return nil, fmt.Errorf("get table indexes info failed, err: %v", err)
	}

	tableDDL, err := i.GetTableDDL(ctx, table.Schema, table.Name)
	if err != nil {
		return nil, fmt.Errorf("get table ddl failed, err: %v", err)
	}

	return &driverV2.TableMeta{
		ColumnsInfo:    columnsInfo,
		IndexesInfo:    indexesInfo,
		CreateTableSQL: tableDDL,
		Message:        "",
	}, nil
}

func (i *driverImpl) getTableColumnsInfo(conn *executor.Executor, schema, tableName string) (driverV2.ColumnsInfo, error) {
	columns := []driverV2.TabularDataHead{
		{
			Name:     "COLUMN_NAME",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("列名"),
		},
		{
			Name:     "Data_Type",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("列类型"),
		},
		{
			Name:     "CHARACTER_SET_NAME",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("列字符集"),
		},
		{
			Name:     "IS_NULLABLE",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("是否可以为空"),
		},
		{
			Name:     "COLUMN_DEFAULT",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("列默认值"),
		},
	}

	queryColumns := make([]string, len(columns))
	for i, c := range columns {
		queryColumns[i] = c.Name
	}
	records, err := conn.GetTableColumnsInfo(schema, tableName)
	if err != nil {
		return driverV2.ColumnsInfo{}, err
	}

	rows := make([][]string, len(records))
	for i, record := range records {
		row := []string{
			record.ColumnName,
			record.ColumnType,
			record.CharacterSetName,
			record.IsNullable,
			record.ColumnDefault,
		}
		rows[i] = row
	}

	ret := driverV2.ColumnsInfo{}
	ret.Columns = columns
	ret.Rows = rows
	return ret, nil
}

func (i *driverImpl) getTableIndexesInfo(conn *executor.Executor, schema, tableName string) (driverV2.IndexesInfo, error) {
	columns := []driverV2.TabularDataHead{
		{
			Name:     "column_name",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("列名"),
		},
		{
			Name:     "key_name",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("索引名"),
		},
		{
			Name:     "unique",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("唯一性"),
		},
		{
			Name:     "index_type",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("索引类型"),
		},
	}

	indexRecords, err := conn.GetTableIndexesInfo(schema, tableName)
	if err != nil {
		return driverV2.IndexesInfo{}, fmt.Errorf("get table indexes info failed, err: %v", err)
	}

	rows := make([][]string, len(indexRecords))
	for i, record := range indexRecords {
		stmts, err := pkgParser.ParseSQL(record.IndexDDL)
		if err != nil {
			return driverV2.IndexesInfo{}, fmt.Errorf("parse index ddl failed, err: %v", err)
		}
		nodeIndexStmt, ok := stmts[0].GetStmt().GetNode().(*parser.Node_IndexStmt)
		if !ok {
			return driverV2.IndexesInfo{}, fmt.Errorf("parse index ddl failed, err: %v", err)
		}

		var columnNames []string
		for _, paramStmt := range nodeIndexStmt.IndexStmt.GetIndexParams() {
			columnNames = append(columnNames, paramStmt.GetIndexElem().GetName())
		}

		rows[i] = []string{
			strings.Join(columnNames, ","),
			record.IndexName,
			strconv.FormatBool(nodeIndexStmt.IndexStmt.GetUnique()),
			nodeIndexStmt.IndexStmt.GetAccessMethod(),
		}
	}

	ret := driverV2.IndexesInfo{}
	ret.Columns = columns
	ret.Rows = rows

	return ret, nil
}

func (i *driverImpl) Explain(ctx context.Context, conf *driverV2.ExplainConf) (*driverV2.ExplainResult, error) {
	// check sql
	// only support dml
	if isDML, err := i.isDML(conf.Sql); err != nil {
		return nil, err
	} else if !isDML {
		return nil, driverV2.ErrSQLIsNotSupported
	}

	resColumn := []driverV2.TabularDataHead{
		{
			Name:     "explain_plan",
			I18nDesc: i18nPkg.ConvertStr2I18nAsDefaultLang("执行计划"),
		},
	}

	if i.executor == nil {
		return nil, errNoConnection
	}
	rows, err := i.executor.GetTableFormExecutionPlan(conf.Sql)
	if err != nil {
		return nil, err
	}

	respRows := make([][]string, len(rows))
	for k, row := range rows {
		respRows[k] = []string{row}
	}

	res := driverV2.ExplainClassicResult{
		TabularData: driverV2.TabularData{
			Columns: resColumn,
			Rows:    respRows,
		},
	}

	return &driverV2.ExplainResult{
		ClassicResult: res,
	}, nil
}

func getUnionSchemaTable(f func(selectStmt *parser.SelectStmt), selectStmt *parser.SelectStmt) {
	if selectStmt.FromClause != nil {
		f(selectStmt)
	}

	LArg := selectStmt.Larg
	rArg := selectStmt.Rarg

	if LArg != nil {
		getUnionSchemaTable(f, LArg)
	}

	if rArg != nil {
		getUnionSchemaTable(f, rArg)
	}

	return
}

func (i *driverImpl) isDML(sql string) (bool, error) {
	//get tables from sql
	node, err := pkgParser.ParseSQL(sql)
	if err != nil {
		return false, err
	}

	sqlType := sqlType(node[0].GetStmt().GetNode())
	if sqlType == driverV2.SQLTypeDML || sqlType == driverV2.SQLTypeDQL {
		return true, nil
	}

	return false, nil
}

func recursionCheckEp(plans []PlanType, rule *driverV2.Rule, expectMaxScanRowNumber int, checkFn func(PlanType, *driverV2.Rule, int) bool, needCheck bool) bool {
	if !needCheck {
		return false
	}
	for _, plan := range plans {
		if !checkFn(plan, rule, expectMaxScanRowNumber) {
			return false
		}
		if plan.Plans == nil {
			return true
		}
		for _, subPlan := range *plan.Plans {
			needCheck = recursionCheckEp([]PlanType{subPlan}, rule, expectMaxScanRowNumber, checkFn, needCheck)
		}
	}
	return needCheck
}

func (i *driverImpl) EstimateSQLAffectRows(ctx context.Context, sql string) (*driverV2.EstimatedAffectRows, error) {
	if i.executor == nil {
		return &driverV2.EstimatedAffectRows{
			ErrMessage: errNoConnection.Error(),
		}, nil
	}

	estimatedCount, err := i.getAffectRowNum(sql)
	if err != nil {
		return &driverV2.EstimatedAffectRows{ErrMessage: fmt.Sprintf("get estimate affected rows failed: %v", err)}, nil
	}

	return &driverV2.EstimatedAffectRows{
		Count: int64(estimatedCount),
	}, nil
}

func (i *driverImpl) getAffectRowNum(sql string) (int, error) {
	convertedSql, count, err := i.convertSql(sql)
	if err != nil {
		return 0, err
	}

	if convertedSql == "" { // 不需要通过查询获取影响行数
		return count, nil
	}

	estimatedCount, err := i.executor.EstimateAffectedRows(convertedSql)
	if err != nil {
		return 0, err
	}

	return estimatedCount, nil
}

func checkConvertedSql(convertedSql string) error {
	convertedNodes, err := pkgParser.ParseSQL(convertedSql)
	if err != nil {
		return fmt.Errorf("parse converted sql failed: %v, sql: %v", err, convertedSql)
	}
	if len(convertedNodes) != 1 {
		return fmt.Errorf("converted sql is not a single sql: %v", convertedSql)
	}
	stmt, ok := convertedNodes[0].GetStmt().GetNode().(*parser.Node_SelectStmt)
	if !ok {
		return fmt.Errorf("converted sql is not a select statement. sql: %v", convertedSql)
	}
	if len(stmt.SelectStmt.GetTargetList()) != 1 {
		return fmt.Errorf("converted sql is not a one target select statement. sql: %v", convertedSql)
	}

	funcCall := stmt.SelectStmt.TargetList[0].GetResTarget().GetVal().GetFuncCall()
	funcNames := funcCall.GetFuncname()
	if len(funcNames) != 1 {
		return fmt.Errorf("converted sql is not a one target select statement. sql: %v", convertedSql)
	}
	if !(funcNames[0].GetString_().GetStr() == "count" && funcCall.AggStar) {
		return fmt.Errorf("converted sql is not a count(*) select statement. sql: %v", convertedSql)
	}

	return nil
}

func (i *driverImpl) convertSql(sql string) (string, int, error) {
	isDml, err := i.isDML(sql)
	if err != nil {
		return "", 0, err
	}
	if !isDml {
		return "", 0, errors.New("only support estimating affected rows of DML")
	}

	astTree, err := parser.Parse(sql)
	if err != nil {
		return "", 0, err
	}
	if len(astTree.Stmts) != 1 {
		return "", 0, fmt.Errorf("got more than one sql")
	}

	selectCountTarget := []*parser.Node{
		{
			Node: &parser.Node_ResTarget{
				ResTarget: &parser.ResTarget{
					Val: &parser.Node{
						Node: &parser.Node_FuncCall{
							FuncCall: &parser.FuncCall{
								AggStar: true,
								Funcname: []*parser.Node{
									{
										Node: &parser.Node_String_{
											String_: &parser.String{
												Str: "count"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	rawStmt := astTree.Stmts[0]
	switch stmt := rawStmt.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt:
		if stmt.SelectStmt.Op == parser.SetOperation_SETOP_UNION {
			return "", 0, errors.New("do not support union clause")
		}
		if stmt.SelectStmt.WithClause != nil {
			return "", 0, errors.New("do not support with clause")
		}
		if stmt.SelectStmt.WindowClause != nil {
			return "", 0, errors.New("do not support window clause")
		}
		if stmt.SelectStmt.TargetList == nil {
			return "", 0, errors.New("unsupported SQL type")
		}
		newStmt := &parser.Node_SelectStmt{
			SelectStmt: &parser.SelectStmt{
				TargetList:  selectCountTarget,
				FromClause:  stmt.SelectStmt.FromClause,
				WhereClause: stmt.SelectStmt.WhereClause,
				SortClause:  stmt.SelectStmt.SortClause,
				LimitOffset: stmt.SelectStmt.LimitOffset,
				LimitCount:  stmt.SelectStmt.LimitCount,
				LimitOption: stmt.SelectStmt.LimitOption,
			},
		}
		rawStmt.Stmt.Node = newStmt
	case *parser.Node_InsertStmt:
		if stmt.InsertStmt.WithClause != nil {
			return "", 0, errors.New("do not support with clause")
		}
		selectStmt, ok := stmt.InsertStmt.GetSelectStmt().GetNode().(*parser.Node_SelectStmt)
		if !ok {
			return "", 0, errors.New("can not find value")
		}

		valueList := selectStmt.SelectStmt.GetValuesLists()
		if valueList != nil {
			return "", len(valueList), nil
		}

		fromClause := selectStmt.SelectStmt.GetFromClause()
		if fromClause != nil {
			newSelectStmt := &parser.Node_SelectStmt{
				SelectStmt: &parser.SelectStmt{
					TargetList: selectCountTarget,
					FromClause: fromClause,
				},
			}
			rawStmt.Stmt.Node = newSelectStmt
		} else {
			return "", 0, errors.New("unsupported SQL type")
		}
	case *parser.Node_UpdateStmt:
		if stmt.UpdateStmt.WithClause != nil {
			return "", 0, errors.New("do not support with clause")
		}
		newSelectStmt := &parser.Node_SelectStmt{
			SelectStmt: &parser.SelectStmt{
				TargetList: selectCountTarget,
				FromClause: []*parser.Node{
					{
						Node: &parser.Node_RangeVar{
							RangeVar: &parser.RangeVar{
								Schemaname: stmt.UpdateStmt.GetRelation().GetSchemaname(),
								Relname:    stmt.UpdateStmt.GetRelation().GetRelname(),
								Inh:        true,
							},
						},
					},
				},
				WhereClause: stmt.UpdateStmt.WhereClause,
			},
		}
		rawStmt.Stmt.Node = newSelectStmt
	case *parser.Node_DeleteStmt:
		if stmt.DeleteStmt.WithClause != nil {
			return "", 0, errors.New("do not support with clause")
		}
		if stmt.DeleteStmt.UsingClause != nil {
			return "", 0, errors.New("do not support using clause")
		}
		newSelectStmt := &parser.Node_SelectStmt{
			SelectStmt: &parser.SelectStmt{
				TargetList: selectCountTarget,
				FromClause: []*parser.Node{
					{
						Node: &parser.Node_RangeVar{
							RangeVar: &parser.RangeVar{
								Schemaname: stmt.DeleteStmt.GetRelation().GetSchemaname(),
								Relname:    stmt.DeleteStmt.GetRelation().GetRelname(),
								Inh:        true,
							},
						},
					},
				},
				WhereClause: stmt.DeleteStmt.WhereClause,
			},
		}
		rawStmt.Stmt.Node = newSelectStmt
	default:
		return "", 0, errors.New("unsupported SQL type")
	}

	astTree.Stmts = []*parser.RawStmt{rawStmt}
	convertedSql, err := parser.Deparse(astTree)
	if err != nil {
		return "", 0, fmt.Errorf("deparse converted ast failed: %v", err)
	}

	if err = checkConvertedSql(convertedSql); err != nil {
		return "", 0, err
	}

	return convertedSql, 0, nil
}

// GenRollbackSQL get rollback sql
func (i *driverImpl) GenRollbackSQL(ctx context.Context, sql string) (string /*rollback sql*/, string /*unSupport reason*/, error) {
	if i.isEnableRollbackConf == false {
		return "", "", nil
	}

	tree, err := parser.Parse(sql)
	if err != nil {
		println(fmt.Sprintf("parse sql failed: %v", err))
		return "", "", nil
	}
	if len(tree.Stmts) != 1 {
		println("parse sql failed: got more than one sql")
		return "", "", nil
	}

	stmt := tree.Stmts[0]
	rollbackSQL, err := i.getRollbackSQL(ctx, stmt)
	if err != nil && errors.As(err, &UnSupportSqlType{}) {
		return "", err.Error(), nil
	}
	if err != nil {
		println(fmt.Sprintf("get rollback sql failed: %v", err))
		return "", "", nil
	}

	return rollbackSQL, "", nil
}

func (i *driverImpl) GetTableDDL(ctx context.Context, schema string, table string) (string, error) {
	columnDefList, err := i.GetTableColumnListByCache(ctx, schema, table)
	if err != nil {
		return "", err
	}
	columnDefStr := strings.Join(columnDefList, ",\n")

	indexDefList, err := i.GetTableIndexByCache(ctx, schema, table)
	if err != nil {
		return "", err
	}
	indexDefStr := strings.Join(indexDefList, "\n\n")

	return fmt.Sprintf("CREATE TABLE %s.%s (\n%s);\n\n%s", schema, table, columnDefStr, indexDefStr), nil
}

func (i *driverImpl) GetTableIndexByCache(ctx context.Context, schemaName, table string) ([]string, error) {
	indexDef := i.tableIndexDef[schemaName]
	if indexDef == nil {
		indexDefList, err := i.executor.GetTableIndexDef(ctx, schemaName, table)
		if err != nil {
			return nil, err
		}
		i.tableIndexDef[schemaName] = make(map[string][]string)
		i.tableIndexDef[schemaName][table] = indexDefList
	} else if indexDef[table] == nil {
		indexDefList, err := i.executor.GetTableIndexDef(ctx, schemaName, table)
		if err != nil {
			return nil, err
		}
		i.tableIndexDef[schemaName][table] = indexDefList
	}
	return i.tableIndexDef[schemaName][table], nil
}

func (i *driverImpl) GetIndexDefByCache(ctx context.Context, schemaName, indexName string) (string, error) {
	schemaTableIndexMap := i.indexDef[schemaName]
	if schemaTableIndexMap == nil {
		indexDef, err := i.executor.GetIndexDef(ctx, schemaName, indexName)
		if err != nil {
			return "", err
		}
		i.indexDef[schemaName] = make(map[string]string)
		i.indexDef[schemaName][indexName] = indexDef
	} else if schemaTableIndexMap[indexName] == "" {
		indexDef, err := i.executor.GetIndexDef(ctx, schemaName, indexName)
		if err != nil {
			return "", err
		}
		i.indexDef[schemaName][indexName] = indexDef
	}

	return i.indexDef[schemaName][indexName], nil
}

func (i *driverImpl) GetPkListByCache(ctx context.Context, schemaName, tableName string) ([]string, error) {
	tablePkListMap := i.pkList[schemaName]
	if tablePkListMap == nil {
		pkList, err := i.executor.GetPkList(ctx, schemaName, tableName)
		if err != nil {
			return nil, err
		}
		i.pkList[schemaName] = make(map[string][]string)
		i.pkList[schemaName][tableName] = pkList
	} else if tablePkListMap[tableName] == nil {
		pkList, err := i.executor.GetPkList(ctx, schemaName, tableName)
		if err != nil {
			return nil, err
		}
		i.pkList[schemaName][tableName] = pkList
	}

	return i.pkList[schemaName][tableName], nil
}

func (i *driverImpl) GetTableColumnListByCache(ctx context.Context, schemaName string, tableName string) ([]string, error) {
	tableColumnDef := i.tableColumnDef[schemaName]
	if tableColumnDef == nil {
		columnList, err := i.executor.GetTableColumnDefList(ctx, schemaName, tableName)
		if err != nil {
			return nil, err
		}
		i.tableColumnDef[schemaName] = make(map[string][]string)
		i.tableColumnDef[schemaName][tableName] = columnList
	} else if tableColumnDef[tableName] == nil {
		columnList, err := i.executor.GetTableColumnDefList(ctx, schemaName, tableName)
		if err != nil {
			return nil, err
		}
		i.tableColumnDef[schemaName][tableName] = columnList
	}

	return i.tableColumnDef[schemaName][tableName], nil
}

// KillProcess :中止上线方法
func (i *driverImpl) KillProcess(ctx context.Context) (*driverV2.KillProcessInfo, error) {
	currentPgPid := i.currentPgProcessId
	if currentPgPid == 0 {
		return &driverV2.KillProcessInfo{
			ErrMessage: fmt.Sprintf("the pg process %d to be killed does not exist", currentPgPid),
		}, fmt.Errorf("the pg process %d to be killed does not exist", currentPgPid)
	}

	if i.Config == nil {
		return &driverV2.KillProcessInfo{
			ErrMessage: "Config parameter is empty.",
		}, fmt.Errorf("config parameter is empty")
	}
	dsn := i.Config.DSN
	if dsn == nil {
		return &driverV2.KillProcessInfo{
			ErrMessage: "DSN parameter is empty.",
		}, fmt.Errorf("DSN parameter is empty")
	}

	// 开启一个新的数据库连接，为了去kill传入的pg进程
	db, _, err := i.Dt.Open(dsn)
	if err != nil {
		return nil, err
	}
	defer func(db *sql.DB) {
		routineErr := db.Close()
		if routineErr != nil {
			i.Log.Error("close db error %s", routineErr)
		}
	}(db)

	// 超时时间为20秒
	dbCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var result bool
	err = db.QueryRowContext(dbCtx, "select pg_terminate_backend($1)", currentPgPid).Scan(&result)
	if err != nil {
		log.Printf("执行KillProcess失败:%s\n", err)
		return &driverV2.KillProcessInfo{
			ErrMessage: "执行KillProcess失败",
		}, err
	}

	if !result {
		return &driverV2.KillProcessInfo{
			ErrMessage: fmt.Sprintf("the pg process %d to be killed does not exist", currentPgPid),
		}, fmt.Errorf("the pg process %d to be killed does not exist", currentPgPid)
	}

	return &driverV2.KillProcessInfo{
		ErrMessage: "",
	}, nil
}

// getRuleDescI18n extracts Desc from I18nRuleInfo as i18nPkg.I18nStr.
// This is a temporary helper until proper i18n is implemented.
func getRuleDescI18n(rule *driverV2.Rule) i18nPkg.I18nStr {
	descI18n := make(i18nPkg.I18nStr)
	for langTag, info := range rule.I18nRuleInfo {
		descI18n[langTag] = info.Desc
	}
	return descI18n
}
