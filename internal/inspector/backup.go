package inspector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	pg_query "github.com/pganalyze/pg_query_go/v2"
)

func (i *driverImpl) Backup(ctx context.Context, req *driverV2.BackupReq) (*driverV2.BackupRes, error) {
	if i.executor == nil {
		return &driverV2.BackupRes{ExecuteResult: "暂不支持不连库备份"}, nil
	}

	tree, err := pg_query.Parse(req.Sql)
	if err != nil {
		return nil, fmt.Errorf("parse sql error: %v", err)
	}
	if len(tree.Stmts) == 0 {
		return nil, fmt.Errorf("empty sql")
	}
	stmt := tree.Stmts[0]

	var backupSqls []string
	var executeResult string

	// Normalize backup strategy: sqle SDK's driver_grpc_server.go passes
	// req.BackupStrategy.String() (protobuf enum name, CamelCase like
	// "OriginalRow"), while driverV2.BackupStrategyXxx constants are
	// snake_case ("original_row"). Accept both forms for compatibility.
	strategy := normalizeBackupStrategy(req.BackupStrategy)

	switch strategy {
	case driverV2.BackupStrategyOriginalRow:
		backupSqls, executeResult, err = i.getOriginalRowBackupSQL(ctx, stmt, req.BackupMaxRows)
	case driverV2.BackupStrategyReverseSql:
		rollbackSQL, rollbackErr := i.getRollbackSQL(ctx, stmt)
		if rollbackErr != nil {
			var unsupported UnSupportSqlType
			if errors.As(rollbackErr, &unsupported) {
				executeResult = rollbackErr.Error()
			} else {
				err = rollbackErr
			}
		} else {
			backupSqls = splitSQLStatements(rollbackSQL)
		}
	case driverV2.BackupStrategyNone, driverV2.BackupStrategyManually:
		// do nothing
	default:
		executeResult = fmt.Sprintf("不支持的备份类型: %s", req.BackupStrategy)
	}

	if err != nil {
		return nil, err
	}

	if executeResult == "" {
		if len(backupSqls) == 0 {
			executeResult = "无影响范围或不支持回滚，无备份回滚语句"
		} else {
			executeResult = "备份成功"
		}
	}

	return &driverV2.BackupRes{BackupSql: backupSqls, ExecuteResult: executeResult}, nil
}

func (i *driverImpl) RecommendBackupStrategy(ctx context.Context, req *driverV2.RecommendBackupStrategyReq) (*driverV2.RecommendBackupStrategyRes, error) {
	tree, err := pg_query.Parse(req.Sql)
	if err != nil {
		return nil, err
	}
	if len(tree.Stmts) == 0 {
		return nil, fmt.Errorf("empty sql")
	}

	stmt := tree.Stmts[0]
	node := stmt.GetStmt().GetNode()

	var strategy, tip string
	var tablesRefer, schemasRefer []string

	switch s := node.(type) {
	case *pg_query.Node_DeleteStmt:
		strategy = driverV2.BackupStrategyOriginalRow
		tip = "删除行操作，建议使用行备份，完整保存被删除行的原始数据"
		schema, table := extractRelation(s.DeleteStmt.GetRelation())
		tablesRefer = []string{table}
		schemasRefer = []string{schema}
	case *pg_query.Node_UpdateStmt:
		strategy = driverV2.BackupStrategyOriginalRow
		tip = "更新行操作，推荐使用行备份，保存更新前的完整行数据"
		schema, table := extractRelation(s.UpdateStmt.GetRelation())
		tablesRefer = []string{table}
		schemasRefer = []string{schema}
	case *pg_query.Node_InsertStmt:
		strategy = driverV2.BackupStrategyReverseSql
		tip = "插入行操作，推荐备份为反向SQL"
		schema, table := extractRelation(s.InsertStmt.GetRelation())
		tablesRefer = []string{table}
		schemasRefer = []string{schema}
	case *pg_query.Node_CreateStmt:
		strategy = driverV2.BackupStrategyReverseSql
		tip = "建表操作，推荐备份为反向SQL"
		schema, table := extractRelation(s.CreateStmt.GetRelation())
		tablesRefer = []string{table}
		schemasRefer = []string{schema}
	case *pg_query.Node_DropStmt:
		if s.DropStmt.RemoveType == pg_query.ObjectType_OBJECT_TABLE {
			strategy = driverV2.BackupStrategyManually
			tip = "删表操作，建议手工备份"
		} else {
			strategy = driverV2.BackupStrategyReverseSql
			tip = "删除索引操作，推荐备份为反向SQL"
		}
	case *pg_query.Node_IndexStmt:
		strategy = driverV2.BackupStrategyReverseSql
		tip = "创建索引操作，推荐备份为反向SQL"
	case *pg_query.Node_AlterTableStmt:
		strategy = driverV2.BackupStrategyReverseSql
		tip = "表结构变更操作，推荐备份为反向SQL"
		schema, table := extractRelation(s.AlterTableStmt.GetRelation())
		tablesRefer = []string{table}
		schemasRefer = []string{schema}
	case *pg_query.Node_RenameStmt:
		strategy = driverV2.BackupStrategyReverseSql
		tip = "重命名操作，推荐备份为反向SQL"
	case *pg_query.Node_SelectStmt:
		strategy = driverV2.BackupStrategyNone
		tip = "SELECT语句，无需备份"
	default:
		strategy = driverV2.BackupStrategyManually
		tip = "暂不支持备份该SQL，请手工备份"
	}

	return &driverV2.RecommendBackupStrategyRes{
		BackupStrategy:    strategy,
		BackupStrategyTip: tip,
		TablesRefer:       tablesRefer,
		SchemasRefer:      schemasRefer,
	}, nil
}

func extractRelation(rel *pg_query.RangeVar) (string, string) {
	schema := rel.GetSchemaname()
	if schema == "" {
		schema = defaultSchema
	}
	return schema, rel.GetRelname()
}

func (i *driverImpl) getOriginalRowBackupSQL(ctx context.Context, rawStmt *pg_query.RawStmt, backupMaxRows uint64) ([]string, string, error) {
	node := rawStmt.GetStmt().GetNode()
	switch stmt := node.(type) {
	case *pg_query.Node_DeleteStmt:
		return i.getOriginalRowForDelete(ctx, stmt, backupMaxRows)
	case *pg_query.Node_UpdateStmt:
		return i.getOriginalRowForUpdate(ctx, stmt, backupMaxRows)
	case *pg_query.Node_InsertStmt:
		// INSERT's original_row is equivalent to reverse_sql: generate reverse DELETE
		rollbackSQL, err := i.getRollbackSQLForInsertStmt(ctx, stmt)
		if err != nil {
			var unsupported UnSupportSqlType
			if errors.As(err, &unsupported) {
				return nil, err.Error(), nil
			}
			return nil, "", err
		}
		return splitSQLStatements(rollbackSQL), "", nil
	default:
		// DDL: reuse getRollbackSQL
		rollbackSQL, err := i.getRollbackSQL(ctx, rawStmt)
		if err != nil {
			var unsupported UnSupportSqlType
			if errors.As(err, &unsupported) {
				return nil, err.Error(), nil
			}
			return nil, "", err
		}
		return splitSQLStatements(rollbackSQL), "", nil
	}
}

func (i *driverImpl) getOriginalRowForDelete(ctx context.Context, stmt *pg_query.Node_DeleteStmt, backupMaxRows uint64) ([]string, string, error) {
	relation := stmt.DeleteStmt.GetRelation()
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = defaultSchema
	}
	table := relation.GetRelname()

	whereClause := stmt.DeleteStmt.GetWhereClause()
	querySQL, err := executor.GetRecordListQuerySQL(schema, table, whereClause)
	if err != nil {
		return nil, "", err
	}

	if backupMaxRows > 0 {
		affectRowNum, err := i.getAffectRowNum(querySQL)
		if err != nil {
			return nil, "", err
		}
		if affectRowNum > int(backupMaxRows) {
			return nil, fmt.Sprintf("受影响行数超过最大备份行数限制%d", backupMaxRows), nil
		}
	}

	rowList, err := i.executor.GetRecordListWithNullInfo(ctx, querySQL)
	if err != nil {
		return nil, "", err
	}
	if len(rowList) == 0 {
		return nil, "", nil
	}

	columnOrder := getColumnOrder(rowList[0])
	return generateInsertIntoSQL(schema, table, rowList, columnOrder), "", nil
}

func (i *driverImpl) getOriginalRowForUpdate(ctx context.Context, stmt *pg_query.Node_UpdateStmt, backupMaxRows uint64) ([]string, string, error) {
	relation := stmt.UpdateStmt.GetRelation()
	schema := relation.GetSchemaname()
	if schema == "" {
		schema = defaultSchema
	}
	table := relation.GetRelname()

	whereClause := stmt.UpdateStmt.GetWhereClause()
	querySQL, err := executor.GetRecordListQuerySQL(schema, table, whereClause)
	if err != nil {
		return nil, "", err
	}

	if backupMaxRows > 0 {
		affectRowNum, err := i.getAffectRowNum(querySQL)
		if err != nil {
			return nil, "", err
		}
		if affectRowNum > int(backupMaxRows) {
			return nil, fmt.Sprintf("受影响行数超过最大备份行数限制%d", backupMaxRows), nil
		}
	}

	rowList, err := i.executor.GetRecordListWithNullInfo(ctx, querySQL)
	if err != nil {
		return nil, "", err
	}
	if len(rowList) == 0 {
		return nil, "", nil
	}

	pkList, err := i.GetPkListByCache(ctx, schema, table)
	if err != nil {
		return nil, "", err
	}

	columnOrder := getColumnOrder(rowList[0])
	if len(pkList) > 0 {
		return generateUpsertSQL(schema, table, rowList, columnOrder, pkList), "", nil
	}
	return generateInsertIntoSQL(schema, table, rowList, columnOrder), "", nil
}

func getColumnOrder(row map[string]sql.NullString) []string {
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func generateInsertIntoSQL(schema, table string, rows []map[string]sql.NullString, columnOrder []string) []string {
	sqls := make([]string, 0, len(rows))
	colList := strings.Join(columnOrder, ", ")
	for _, row := range rows {
		values := make([]string, 0, len(columnOrder))
		for _, col := range columnOrder {
			ns := row[col]
			if !ns.Valid {
				values = append(values, "NULL")
			} else {
				escaped := strings.ReplaceAll(ns.String, "'", "''")
				values = append(values, fmt.Sprintf("'%s'", escaped))
			}
		}
		s := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s);", schema, table, colList, strings.Join(values, ", "))
		sqls = append(sqls, s)
	}
	return sqls
}

// generateUpsertSQL generates INSERT ... ON CONFLICT (pk) DO UPDATE SET ... statements
// for UPDATE's original_row backup, so that rollback works even when the primary key row still exists.
func generateUpsertSQL(schema, table string, rows []map[string]sql.NullString, columnOrder []string, pkColumns []string) []string {
	sqls := make([]string, 0, len(rows))
	colList := strings.Join(columnOrder, ", ")

	pkSet := make(map[string]struct{}, len(pkColumns))
	for _, pk := range pkColumns {
		pkSet[pk] = struct{}{}
	}

	// Build the ON CONFLICT DO UPDATE SET clause for non-pk columns
	var updateParts []string
	for _, col := range columnOrder {
		if _, isPK := pkSet[col]; !isPK {
			updateParts = append(updateParts, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
		}
	}

	conflictClause := ""
	if len(updateParts) > 0 {
		conflictClause = fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s",
			strings.Join(pkColumns, ", "), strings.Join(updateParts, ", "))
	} else {
		// All columns are primary keys, use DO NOTHING
		conflictClause = fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", strings.Join(pkColumns, ", "))
	}

	for _, row := range rows {
		values := make([]string, 0, len(columnOrder))
		for _, col := range columnOrder {
			ns := row[col]
			if !ns.Valid {
				values = append(values, "NULL")
			} else {
				escaped := strings.ReplaceAll(ns.String, "'", "''")
				values = append(values, fmt.Sprintf("'%s'", escaped))
			}
		}
		s := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)%s;",
			schema, table, colList, strings.Join(values, ", "), conflictClause)
		sqls = append(sqls, s)
	}
	return sqls
}

func splitSQLStatements(sql string) []string {
	if sql == "" {
		return nil
	}
	parts := strings.Split(sql, "\n")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// normalizeBackupStrategy accepts both the snake_case form
// (driverV2.BackupStrategyXxx values like "original_row") and the protobuf
// enum-name form ("OriginalRow", "ReverseSql", "None", "Manually") that
// vendored sqle SDK's driver_grpc_server.go produces via
// req.BackupStrategy.String(). Returns the canonical snake_case constant; an
// empty string for unknown inputs so the caller's default branch can produce
// a localized error message.
func normalizeBackupStrategy(s string) string {
	switch s {
	case driverV2.BackupStrategyNone,
		driverV2.BackupStrategyReverseSql,
		driverV2.BackupStrategyOriginalRow,
		driverV2.BackupStrategyManually:
		return s
	case "None":
		return driverV2.BackupStrategyNone
	case "ReverseSql":
		return driverV2.BackupStrategyReverseSql
	case "OriginalRow":
		return driverV2.BackupStrategyOriginalRow
	case "Manually":
		return driverV2.BackupStrategyManually
	}
	return s
}
