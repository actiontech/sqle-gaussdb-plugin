package inspector

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestRecommendBackupStrategy(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)

	cases := []struct {
		Name           string
		SQL            string
		ExpectStrategy string
		ExpectTables   []string
		ExpectSchemas  []string
	}{
		{
			Name:           "DELETE recommends original_row",
			SQL:            "DELETE FROM public.t WHERE id=1",
			ExpectStrategy: driverV2.BackupStrategyOriginalRow,
			ExpectTables:   []string{"t"},
			ExpectSchemas:  []string{"public"},
		},
		{
			Name:           "UPDATE recommends original_row",
			SQL:            "UPDATE public.t SET name='x'",
			ExpectStrategy: driverV2.BackupStrategyOriginalRow,
			ExpectTables:   []string{"t"},
			ExpectSchemas:  []string{"public"},
		},
		{
			Name:           "INSERT recommends reverse_sql",
			SQL:            "INSERT INTO public.t VALUES (1)",
			ExpectStrategy: driverV2.BackupStrategyReverseSql,
			ExpectTables:   []string{"t"},
			ExpectSchemas:  []string{"public"},
		},
		{
			Name:           "CREATE TABLE recommends reverse_sql",
			SQL:            "CREATE TABLE public.t (id int)",
			ExpectStrategy: driverV2.BackupStrategyReverseSql,
			ExpectTables:   []string{"t"},
			ExpectSchemas:  []string{"public"},
		},
		{
			Name:           "DROP TABLE recommends manual",
			SQL:            "DROP TABLE public.t",
			ExpectStrategy: driverV2.BackupStrategyManually,
			ExpectTables:   nil,
			ExpectSchemas:  nil,
		},
		{
			Name:           "ALTER TABLE recommends reverse_sql",
			SQL:            "ALTER TABLE public.t ADD COLUMN c int",
			ExpectStrategy: driverV2.BackupStrategyReverseSql,
			ExpectTables:   []string{"t"},
			ExpectSchemas:  []string{"public"},
		},
		{
			Name:           "SELECT recommends none",
			SQL:            "SELECT * FROM public.t",
			ExpectStrategy: driverV2.BackupStrategyNone,
			ExpectTables:   nil,
			ExpectSchemas:  nil,
		},
		{
			Name:           "CREATE INDEX recommends reverse_sql",
			SQL:            "CREATE INDEX idx ON public.t (id)",
			ExpectStrategy: driverV2.BackupStrategyReverseSql,
			ExpectTables:   nil,
			ExpectSchemas:  nil,
		},
		{
			Name:           "RENAME TABLE recommends reverse_sql",
			SQL:            "ALTER TABLE public.t RENAME TO t2",
			ExpectStrategy: driverV2.BackupStrategyReverseSql,
			ExpectTables:   nil,
			ExpectSchemas:  nil,
		},
		{
			Name:           "DELETE without schema defaults to public",
			SQL:            "DELETE FROM t WHERE id=1",
			ExpectStrategy: driverV2.BackupStrategyOriginalRow,
			ExpectTables:   []string{"t"},
			ExpectSchemas:  []string{"public"},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			res, err := i.RecommendBackupStrategy(context.Background(), &driverV2.RecommendBackupStrategyReq{Sql: c.SQL})
			assert.NoError(t, err)
			assert.Equal(t, c.ExpectStrategy, res.BackupStrategy)
			assert.Equal(t, c.ExpectTables, res.TablesRefer)
			assert.Equal(t, c.ExpectSchemas, res.SchemasRefer)
		})
	}
}

func TestBackup_OfflineMode(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, nil) // no executor
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "DELETE FROM t WHERE id=1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "暂不支持不连库备份", res.ExecuteResult)
}

func TestBackup_StrategyNone(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyNone,
		Sql:            "DELETE FROM t WHERE id=1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "无影响范围或不支持回滚，无备份回滚语句", res.ExecuteResult)
	assert.Empty(t, res.BackupSql)
}

func TestBackup_StrategyManual(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyManually,
		Sql:            "DELETE FROM t WHERE id=1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "无影响范围或不支持回滚，无备份回滚语句", res.ExecuteResult)
	assert.Empty(t, res.BackupSql)
}

func TestBackup_UnsupportedStrategy(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: "unknown_strategy",
		Sql:            "DELETE FROM t WHERE id=1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "不支持的备份类型: unknown_strategy", res.ExecuteResult)
}

func TestNormalizeBackupStrategy(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// snake_case (driverV2 constants) pass through unchanged
		{driverV2.BackupStrategyNone, driverV2.BackupStrategyNone},
		{driverV2.BackupStrategyReverseSql, driverV2.BackupStrategyReverseSql},
		{driverV2.BackupStrategyOriginalRow, driverV2.BackupStrategyOriginalRow},
		{driverV2.BackupStrategyManually, driverV2.BackupStrategyManually},
		// protobuf enum name form (passed by vendored driver_grpc_server.go
		// via req.BackupStrategy.String()) is normalized to snake_case
		{"None", driverV2.BackupStrategyNone},
		{"ReverseSql", driverV2.BackupStrategyReverseSql},
		{"OriginalRow", driverV2.BackupStrategyOriginalRow},
		{"Manually", driverV2.BackupStrategyManually},
		// unknown strings pass through (caller's default branch handles them)
		{"unknown_strategy", "unknown_strategy"},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeBackupStrategy(c.in)
		if got != c.want {
			t.Errorf("normalizeBackupStrategy(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestBackup_StrategyCamelCase covers the regression where the vendored sqle
// SDK's driver_grpc_server.go forwards req.BackupStrategy.String() (protobuf
// enum name "OriginalRow") instead of the snake_case constant
// ("original_row"). The plugin must accept both forms so that BackupStrategy
// from the EE server is routed to the correct branch.
//
// Without an executor we reach the offline-mode early return before the
// strategy switch; with a populated executor the OriginalRow path requires
// query mocking that is covered by the existing
// TestBackup_OriginalRow_Delete*/Update* / ReverseSql* / OfflineMode tests.
// Here we only assert that the CamelCase "OriginalRow" value does NOT fall
// through to the unsupported branch.
func TestBackup_StrategyCamelCase_OriginalRow_OfflineMode(t *testing.T) {
	// executor == nil triggers the offline branch before strategy switch
	i := &driverImpl{}
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: "OriginalRow",
		Sql:            "DELETE FROM t WHERE id=1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "暂不支持不连库备份", res.ExecuteResult)
	assert.Empty(t, res.BackupSql)
}

func TestBackup_StrategyCamelCase_None(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: "None",
		Sql:            "DELETE FROM t WHERE id=1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "无影响范围或不支持回滚，无备份回滚语句", res.ExecuteResult)
	assert.Empty(t, res.BackupSql)
}

func TestBackup_StrategyCamelCase_Manually(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: "Manually",
		Sql:            "DELETE FROM t WHERE id=1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "无影响范围或不支持回滚，无备份回滚语句", res.ExecuteResult)
	assert.Empty(t, res.BackupSql)
}

func TestBackup_ReverseSql_CreateTable(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyReverseSql,
		Sql:            "CREATE TABLE public.t (id int)",
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	assert.Equal(t, []string{"DROP TABLE IF EXISTS public.t;"}, res.BackupSql)
}

func TestBackup_OriginalRow_CreateTable(t *testing.T) {
	// DDL with original_row falls back to reverse SQL
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "CREATE TABLE public.t (id int)",
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	assert.Equal(t, []string{"DROP TABLE IF EXISTS public.t;"}, res.BackupSql)
}

func TestBackup_OriginalRow_Delete(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	// getAffectRowNum: convertSql produces count query
	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(1))
	// GetRecordListWithNullInfo
	mock.ExpectQuery("SELECT * FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"id", "name"}).AddRow(1, "hello"))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "DELETE FROM public.t WHERE id = 1",
		BackupMaxRows:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	assert.Equal(t, []string{"INSERT INTO public.t (id, name) VALUES ('1', 'hello');"}, res.BackupSql)
}

func TestBackup_OriginalRow_Delete_NullAndEmpty(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(1))
	// Return a row with NULL value for 'name' and empty string for 'desc'
	mock.ExpectQuery("SELECT * FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"desc", "id", "name"}).
			AddRow("", 1, nil))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "DELETE FROM public.t WHERE id = 1",
		BackupMaxRows:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	// columns are sorted alphabetically: desc, id, name
	assert.Equal(t, []string{"INSERT INTO public.t (desc, id, name) VALUES ('', '1', NULL);"}, res.BackupSql)
}

func TestBackup_OriginalRow_Delete_SingleQuoteEscape(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT * FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"id", "val"}).AddRow(1, "it's a test"))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "DELETE FROM public.t WHERE id = 1",
		BackupMaxRows:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	// PostgreSQL escapes single quotes as ''
	assert.Equal(t, []string{"INSERT INTO public.t (id, val) VALUES ('1', 'it''s a test');"}, res.BackupSql)
}

func TestBackup_OriginalRow_Delete_ExceedsMaxRows(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id > 0").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(100))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "DELETE FROM public.t WHERE id > 0",
		BackupMaxRows:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, "受影响行数超过最大备份行数限制10", res.ExecuteResult)
	assert.Empty(t, res.BackupSql)
}

func TestBackup_OriginalRow_Update(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT * FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"id", "name"}).AddRow(1, "old_val"))
	// GetPkListByCache
	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}).AddRow("id"))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "UPDATE public.t SET name='new_val' WHERE id = 1",
		BackupMaxRows:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	assert.Equal(t, []string{"INSERT INTO public.t (id, name) VALUES ('1', 'old_val') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;"}, res.BackupSql)
}

func TestBackup_OriginalRow_Update_NoPK(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT * FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"id", "name"}).AddRow(1, "old_val"))
	// Empty pk list
	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "UPDATE public.t SET name='new_val' WHERE id = 1",
		BackupMaxRows:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	// No PK -> falls back to plain INSERT
	assert.Equal(t, []string{"INSERT INTO public.t (id, name) VALUES ('1', 'old_val');"}, res.BackupSql)
}

func TestBackup_ReverseSql_Update(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	// getRollbackSQLForUpdateStmt needs: pk list, affect row count, record list
	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}).AddRow("id"))
	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT * FROM public.t WHERE id = 1").
		WillReturnRows(mock.NewRows([]string{"id", "name"}).AddRow(1, "old_val"))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyReverseSql,
		Sql:            "UPDATE public.t SET name='new_val' WHERE id = 1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	// Should generate reverse UPDATE, NOT INSERT INTO
	assert.Equal(t, []string{"UPDATE public.t SET name = 'old_val' WHERE id = '1';"}, res.BackupSql)
}

func TestBackup_OriginalRow_Insert_FallsBackToReverseSql(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	// INSERT original_row needs pk list
	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}).AddRow("id"))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "INSERT INTO public.t (id, name) VALUES (1, 'a')",
	})
	assert.NoError(t, err)
	assert.Equal(t, "备份成功", res.ExecuteResult)
	assert.Equal(t, []string{"DELETE FROM public.t WHERE id = '1';"}, res.BackupSql)
}

func TestBackup_OriginalRow_Insert_NoPrimaryKey(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	// Return empty pk list
	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "INSERT INTO public.t (id, name) VALUES (1, 'a')",
	})
	assert.NoError(t, err)
	// No primary key -> unsupported message
	assert.Contains(t, res.ExecuteResult, "不支持生成回滚语句")
	assert.Empty(t, res.BackupSql)
}

func TestBackup_OriginalRow_Delete_NoRows(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT count(*) FROM public.t WHERE id = 999").
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT * FROM public.t WHERE id = 999").
		WillReturnRows(mock.NewRows([]string{"id", "name"}))

	i := NewMockDriver(nil, "", "", nil, e)
	res, err := i.Backup(context.Background(), &driverV2.BackupReq{
		BackupStrategy: driverV2.BackupStrategyOriginalRow,
		Sql:            "DELETE FROM public.t WHERE id = 999",
		BackupMaxRows:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, "无影响范围或不支持回滚，无备份回滚语句", res.ExecuteResult)
	assert.Empty(t, res.BackupSql)
}

func TestGenerateInsertIntoSQL(t *testing.T) {
	cases := []struct {
		Name        string
		Schema      string
		Table       string
		Rows        []map[string]sql.NullString
		ColumnOrder []string
		Expected    []string
	}{
		{
			Name:   "single row with all valid values",
			Schema: "public",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"id": {String: "1", Valid: true}, "name": {String: "hello", Valid: true}},
			},
			ColumnOrder: []string{"id", "name"},
			Expected:    []string{"INSERT INTO public.t (id, name) VALUES ('1', 'hello');"},
		},
		{
			Name:   "NULL vs empty string",
			Schema: "public",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"id": {String: "1", Valid: true}, "name": {Valid: false}, "desc": {String: "", Valid: true}},
			},
			ColumnOrder: []string{"desc", "id", "name"},
			Expected:    []string{"INSERT INTO public.t (desc, id, name) VALUES ('', '1', NULL);"},
		},
		{
			Name:   "single quote escaping",
			Schema: "public",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"id": {String: "1", Valid: true}, "val": {String: "it's", Valid: true}},
			},
			ColumnOrder: []string{"id", "val"},
			Expected:    []string{"INSERT INTO public.t (id, val) VALUES ('1', 'it''s');"},
		},
		{
			Name:   "multiple rows",
			Schema: "myschema",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"a": {String: "1", Valid: true}},
				{"a": {String: "2", Valid: true}},
			},
			ColumnOrder: []string{"a"},
			Expected: []string{
				"INSERT INTO myschema.t (a) VALUES ('1');",
				"INSERT INTO myschema.t (a) VALUES ('2');",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			result := generateInsertIntoSQL(c.Schema, c.Table, c.Rows, c.ColumnOrder)
			assert.Equal(t, c.Expected, result)
		})
	}
}

func TestGenerateUpsertSQL(t *testing.T) {
	cases := []struct {
		Name        string
		Schema      string
		Table       string
		Rows        []map[string]sql.NullString
		ColumnOrder []string
		PkColumns   []string
		Expected    []string
	}{
		{
			Name:   "single row with one pk column",
			Schema: "public",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"id": {String: "1", Valid: true}, "name": {String: "old", Valid: true}},
			},
			ColumnOrder: []string{"id", "name"},
			PkColumns:   []string{"id"},
			Expected:    []string{"INSERT INTO public.t (id, name) VALUES ('1', 'old') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;"},
		},
		{
			Name:   "composite pk",
			Schema: "public",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"a": {String: "1", Valid: true}, "b": {String: "2", Valid: true}, "c": {String: "val", Valid: true}},
			},
			ColumnOrder: []string{"a", "b", "c"},
			PkColumns:   []string{"a", "b"},
			Expected:    []string{"INSERT INTO public.t (a, b, c) VALUES ('1', '2', 'val') ON CONFLICT (a, b) DO UPDATE SET c = EXCLUDED.c;"},
		},
		{
			Name:   "all columns are pk -> DO NOTHING",
			Schema: "public",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"id": {String: "1", Valid: true}},
			},
			ColumnOrder: []string{"id"},
			PkColumns:   []string{"id"},
			Expected:    []string{"INSERT INTO public.t (id) VALUES ('1') ON CONFLICT (id) DO NOTHING;"},
		},
		{
			Name:   "NULL value in non-pk column",
			Schema: "public",
			Table:  "t",
			Rows: []map[string]sql.NullString{
				{"id": {String: "1", Valid: true}, "name": {Valid: false}},
			},
			ColumnOrder: []string{"id", "name"},
			PkColumns:   []string{"id"},
			Expected:    []string{"INSERT INTO public.t (id, name) VALUES ('1', NULL) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;"},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			result := generateUpsertSQL(c.Schema, c.Table, c.Rows, c.ColumnOrder, c.PkColumns)
			assert.Equal(t, c.Expected, result)
		})
	}
}

func TestSplitSQLStatements(t *testing.T) {
	cases := []struct {
		Input    string
		Expected []string
	}{
		{"", nil},
		{"DELETE FROM t WHERE id=1;", []string{"DELETE FROM t WHERE id=1;"}},
		{"DELETE FROM t WHERE id=1;\nDELETE FROM t WHERE id=2;", []string{"DELETE FROM t WHERE id=1;", "DELETE FROM t WHERE id=2;"}},
		{"  \n  ", []string{}},
	}
	for _, c := range cases {
		result := splitSQLStatements(c.Input)
		assert.Equal(t, c.Expected, result)
	}
}
