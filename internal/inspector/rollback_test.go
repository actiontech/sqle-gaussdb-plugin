package inspector

import (
	"context"
	"os"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/hashicorp/go-hclog"
	parser "github.com/pganalyze/pg_query_go/v2"
	"github.com/stretchr/testify/assert"
)

// for no need to mock executor query
var defaultExecWithoutExec *executor.Executor

func init() {
	exec, _, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		panic(err)
	}

	defaultExecWithoutExec = exec
}

type testRollback struct {
	Name      string
	SQL       string
	ExpectSQL string
}

func TestGetRollbackSQL(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	args := []testRollback{
		{
			Name:      "alter table",
			SQL:       `alter table tb1 add column c1 int`,
			ExpectSQL: `ALTER TABLE public.tb1 DROP COLUMN c1;`,
		},
		{
			Name:      "create table",
			SQL:       `create table tb1 (a int)`,
			ExpectSQL: `DROP TABLE IF EXISTS public.tb1;`,
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt := tree.GetStmts()[0]

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQL(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

// test getRollbackSQLForCreateStatement
// 1. create table https://www.postgresql.org/docs/current/sql-createtable.html
// 2. drop table https://www.postgresql.org/docs/current/sql-droptable.html
func Test_getRollbackSQLForCreateStmt(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	args := []testRollback{
		{
			Name:      "create table",
			SQL:       "create table tb1 (a int);",
			ExpectSQL: "DROP TABLE IF EXISTS public.tb1;",
		},
		{
			Name:      "create table without table name",
			SQL:       "create table t1.tb1 (a int);",
			ExpectSQL: "DROP TABLE IF EXISTS t1.tb1;",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_CreateStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForCreateStmt(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

// 1. create index https://www.postgresql.org/docs/current/sql-createindex.html
// 2. drop index https://www.postgresql.org/docs/current/sql-dropindex.html
func Test_getRollbackSQLForIndexStmt(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	args := []testRollback{
		{
			Name:      "create index",
			SQL:       "create index idx1 on tb1 (a);",
			ExpectSQL: "DROP INDEX IF EXISTS public.idx1;",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_IndexStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForCreateIndexStmt(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

// test getRollbackSQLForRename
func Test_getRollbackSQLForRename(t *testing.T) {
	i := NewMockDriver(nil, "", "", nil, defaultExecWithoutExec)
	args := []testRollback{
		{
			Name:      "rename table with schema name",
			SQL:       "alter table t1.tb1 rename to tb2",
			ExpectSQL: "ALTER TABLE t1.tb2 RENAME TO tb1;",
		},
		{
			Name:      "rename table without schema name",
			SQL:       "alter table tb1 rename to tb2",
			ExpectSQL: "ALTER TABLE public.tb2 RENAME TO tb1;",
		},
		{
			Name:      "rename column without schema name",
			SQL:       "alter table t1 rename column c1 to c2",
			ExpectSQL: "ALTER TABLE public.t1 RENAME COLUMN c2 TO c1;",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_RenameStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForRename(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

func Test_getRollbackSQLForAlterTable(t *testing.T) {
	exec, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		panic(err)
	}

	mock.
		ExpectQuery("SELECT column_name, udt_name, character_maximum_length, is_nullable, column_default FROM information_schema.columns WHERE table_name = $1 AND table_schema = $2 AND column_name = $3;").
		WillReturnRows(mock.NewRows([]string{"column_name", "udt_name", "character_maximum_length", "is_nullable", "column_default"}).
			AddRow("c1", "varchar", 10, "YES", "'test'::character varying"))

	mock.
		ExpectQuery(`
SELECT indexdef
FROM pg_indexes
WHERE schemaname = $1
AND indexname = $2;
`).WillReturnRows(mock.NewRows([]string{"indexdef"}).
		AddRow("CREATE INDEX idx ON public.t1 USING btree (test_name)"))

	i := NewMockDriver(nil, "", "", nil, exec)
	args := []testRollback{
		{
			Name:      "add column without schema",
			SQL:       "alter table tb1 add column c1 int",
			ExpectSQL: "ALTER TABLE public.tb1 DROP COLUMN c1;",
		},
		{
			Name:      "add column with schema",
			SQL:       "alter table public.tb1 add column c1 int",
			ExpectSQL: "ALTER TABLE public.tb1 DROP COLUMN c1;",
		},
		{
			Name:      "add constraint without schema",
			SQL:       "alter table t1 add constraint idx_id unique (id)",
			ExpectSQL: "DROP INDEX public.idx_id;",
		},
		{
			Name:      "add constraint with schema",
			SQL:       "alter table public.t1 add constraint idx_id unique (id)",
			ExpectSQL: "DROP INDEX public.idx_id;",
		},
		{
			Name:      "drop column",
			SQL:       "alter table t1 drop column c1",
			ExpectSQL: "ALTER TABLE public.t1 ADD COLUMN c1 varchar(10) DEFAULT 'test'::character varying;",
		},
		{
			Name:      "drop constraint",
			SQL:       "alter table t1 drop constraint idx",
			ExpectSQL: "CREATE INDEX idx ON public.t1 USING btree (test_name)",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_AlterTableStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForAlterTable(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

// 1. create table https://www.postgresql.org/docs/current/sql-createtable.html
// 2. drop table https://www.postgresql.org/docs/current/sql-droptable.html
// 3. create index https://www.postgresql.org/docs/current/sql-createindex.html
// 4. drop index https://www.postgresql.org/docs/current/sql-dropindex.html
func Test_getRollbackSQLForDropStmt(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		panic(err)
	}

	mock.
		ExpectQuery(`
SELECT column_name, udt_name, character_maximum_length, is_nullable, column_default FROM information_schema.columns WHERE table_name = $1 AND table_schema = $2
`).WillReturnRows(mock.NewRows([]string{"column_name", "udt_name", "character_maximum_length", "is_nullable", "column_default"}).
		AddRow("c1", "varchar", 10, "YES", "'test'::character varying"))

	mock.ExpectQuery(`
SELECT PG_GET_INDEXDEF(indexrelid) AS index_def
FROM pg_index
WHERE indrelid = $1 ::regclass`).WillReturnRows(mock.NewRows([]string{"index_def"}).AddRow("CREATE INDEX idx ON public.t1 USING btree (c1)"))

	mock.
		ExpectQuery(`
SELECT indexdef
FROM pg_indexes
WHERE schemaname = $1
AND indexname = $2;
`).WillReturnRows(mock.NewRows([]string{"indexdef"}).
		AddRow("CREATE INDEX idx ON public.t1 USING btree (test_name)"))

	i := NewMockDriver(nil, "", "", nil, e)
	args := []testRollback{
		{
			Name:      "drop table",
			SQL:       "drop table IF EXISTS public.t1",
			ExpectSQL: "CREATE TABLE public.t1 (\nc1 varchar(10) DEFAULT 'test'::character varying);\n\nCREATE INDEX idx ON public.t1 USING btree (c1)",
		},
		{
			Name:      "drop index",
			SQL:       "drop index idx",
			ExpectSQL: "CREATE INDEX idx ON public.t1 USING btree (test_name)",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_DropStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForDropStmt(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

// test insert rollback sql
func Test_getRollbackSQLForInsertStmt(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		panic(err)
	}

	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}).
			AddRow("a"))

	i := NewMockDriver(nil, "", "", nil, e)

	args := []testRollback{
		{
			Name:      "insert into",
			SQL:       "insert into tb1 (a) values (1)",
			ExpectSQL: "DELETE FROM public.tb1 WHERE a = '1';",
		},
		{
			Name:      "insert into multi rows",
			SQL:       "insert into tb1 (a,b) values (1,'a'), (2,'b')",
			ExpectSQL: "DELETE FROM public.tb1 WHERE a = '1';\nDELETE FROM public.tb1 WHERE a = '2';",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_InsertStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForInsertStmt(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

// test update rollback sql
// https://www.postgresql.org/docs/current/sql-update.html
func Test_getRollbackSQLForUpdateStmt(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		panic(err)
	}

	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}).
			AddRow("a"))

	mock.ExpectQuery("SELECT count(*) FROM public.tb1 WHERE a = '1'").
		WillReturnRows(mock.NewRows([]string{"count"}).
			AddRow(21))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a = '1'").WillReturnRows(mock.NewRows([]string{"a", "b"}).AddRow(1, 13))

	mock.ExpectQuery("SELECT count(*) FROM public.tb1 WHERE a IN (2, 3)").
		WillReturnRows(mock.NewRows([]string{"count"}).
			AddRow(21))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a IN (2, 3)").WillReturnRows(mock.NewRows([]string{"a", "b"}).AddRow(2, 14).AddRow(3, 15))

	mock.ExpectQuery("SELECT count(*) FROM public.tb1 WHERE a = '1'").
		WillReturnRows(mock.NewRows([]string{"count"}).
			AddRow(21))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a = '1'").WillReturnRows(mock.NewRows([]string{"a", "b"}).AddRow(1, 13))

	mock.ExpectQuery("SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE i.indrelid = $1 ::regclass AND a.attnum = ANY (i.indkey) AND i.indisprimary;").
		WillReturnRows(mock.NewRows([]string{"attname"}).
			AddRow("a"))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a = '1'").WillReturnRows(mock.NewRows([]string{"a", "b"}).AddRow(1, 13))

	mock.ExpectQuery("SELECT count(*) FROM public.tb1 WHERE a IN (2, 3)").
		WillReturnRows(mock.NewRows([]string{"count"}).
			AddRow(21))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a IN (2, 3)").WillReturnRows(mock.NewRows([]string{"a", "b"}).AddRow(2, 14).AddRow(3, 15))

	mock.ExpectQuery("SELECT count(*) FROM public.tb1 WHERE a = '1'").
		WillReturnRows(mock.NewRows([]string{"count"}).
			AddRow(21))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a = '1'").WillReturnRows(mock.NewRows([]string{"a", "b"}).AddRow(1, 13))

	i := NewMockDriver(nil, "test", "public", nil, e)

	args := []testRollback{
		{
			Name:      "update",
			SQL:       "update tb1 set b = 1 where a = '1'",
			ExpectSQL: "UPDATE public.tb1 SET b = '13' WHERE a = '1';",
		},
		{
			Name:      "update multi rows",
			SQL:       "update tb1 set b = 1 where a in (2, 3)",
			ExpectSQL: "UPDATE public.tb1 SET b = '14' WHERE a = '2';\nUPDATE public.tb1 SET b = '15' WHERE a = '3';",
		},
		// change a primary key column in update statement
		{
			Name:      "update primary key",
			SQL:       "update tb1 set a = '119' where a = '1'",
			ExpectSQL: "UPDATE public.tb1 SET a = '1' WHERE a = '119';",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_UpdateStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForUpdateStmt(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}

// test delete rollback sql
func Test_getRollbackSQLForDeleteStmt(t *testing.T) {
	e, mock, err := executor.NewMockExecutor(hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	}))
	if err != nil {
		panic(err)
	}

	mock.ExpectQuery("SELECT count(*) FROM public.tb1 WHERE a = 1").
		WillReturnRows(mock.NewRows([]string{"count"}).
			AddRow(21))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a = 1").WillReturnRows(mock.NewRows([]string{"a"}).AddRow(1))

	mock.ExpectQuery("SELECT count(*) FROM public.tb1 WHERE a IN (1, 2)").
		WillReturnRows(mock.NewRows([]string{"count"}).
			AddRow(21))

	mock.ExpectQuery("SELECT * FROM public.tb1 WHERE a IN (1, 2)").WillReturnRows(mock.NewRows([]string{"a"}).AddRow(1).AddRow(2))

	i := NewMockDriver(nil, "", "", nil, e)

	args := []testRollback{
		{
			Name:      "delete",
			SQL:       "delete from tb1 where a = 1",
			ExpectSQL: "INSERT INTO public.tb1 (a) VALUES ('1');",
		},
		{
			Name:      "delete multi rows",
			SQL:       "delete from tb1 where a in (1, 2)",
			ExpectSQL: "INSERT INTO public.tb1 (a) VALUES ('1'),('2');",
		},
	}

	for _, arg := range args {
		tree, err := parser.Parse(arg.SQL)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		stmt, ok := tree.GetStmts()[0].GetStmt().GetNode().(*parser.Node_DeleteStmt)
		assert.True(t, ok)

		t.Run(arg.Name, func(t *testing.T) {
			sql, err := i.getRollbackSQLForDeleteStmt(context.Background(), stmt)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectSQL, sql)
		})
	}
}
