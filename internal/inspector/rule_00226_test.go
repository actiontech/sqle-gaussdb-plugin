package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00226(t *testing.T) {
	ruleName := SQLE00226
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 测试空表插入后表大小是否超过阈值",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'data');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟分区不分片表大小不超过 32GB",
		},
		{
			desc:          "case 2: 测试空表插入后表大小是否超过阈值",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'data');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟分区不分片表大小超过 32GB",
		},
		{
			desc:          "case 3: 测试空表插入后表大小是否超过阈值",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'data');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟原生分区表不超过 32GB",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context

			d, ctx = setupForRuleSpecific(t, ruleName, _DefaultParams, test.mockDesc)

			result, err := d.Audit(ctx, []string{test.sql})

			if test.expectError {
				assert.Error(t, err, test.desc)
			} else {
				assert.NoError(t, err, test.desc)
				assertRuleResult(t, test.desc, test.sql, test.expectResults, result)
			}
		})
	}
}

// Custom setup function for specific mock scenarios
func setupForRuleSpecific(t *testing.T, ruleName string, params params.Params, mockDesc string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 根据mock描述，设置不同的数据库模拟行为
	switch mockDesc {
	case "模拟分区不分片表大小不超过 32GB":
		mock.ExpectQuery(`SELECT c.relkind, c.relpartkind
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = $1
  AND c.relname = $2;`).WithArgs("exist_db", "exist_tb_1").
			WillReturnRows(mock.NewRows([]string{"relkind", "relpartkind"}).AddRow("r", "n"))
		mock.ExpectQuery(`SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
		FROM pg_class c JOIN pg_namespace nsp ON c.relnamespace = nsp.oid WHERE nsp.nspname = 'exist_db' AND c.relname IN ('exist_tb_1')`).
			WillReturnRows(mock.NewRows([]string{"nspname", "relname", "pg_relation_size"}).AddRow("exist_db", "exist_tb_1", "12"))

	case "模拟分区不分片表大小超过 32GB":
		mock.ExpectQuery(`SELECT c.relkind, c.relpartkind
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = $1
  AND c.relname = $2;`).WithArgs("exist_db", "exist_tb_1").
			WillReturnRows(mock.NewRows([]string{"relkind", "relpartkind"}).AddRow("r", "n"))
		mock.ExpectQuery(`SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
		FROM pg_class c JOIN pg_namespace nsp ON c.relnamespace = nsp.oid WHERE nsp.nspname = 'exist_db' AND c.relname IN ('exist_tb_1')`).
			WillReturnRows(mock.NewRows([]string{"nspname", "relname", "pg_relation_size"}).AddRow("exist_db", "exist_tb_1", "33"))

	case "模拟原生分区表不超过 32GB":
		mock.ExpectQuery(`SELECT c.relkind, c.relpartkind
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = $1
  AND c.relname = $2;`).WithArgs("exist_db", "exist_tb_1").
			WillReturnRows(mock.NewRows([]string{"relkind", "relpartkind"}).AddRow("p", "n"))
		mock.ExpectQuery("EXPLAIN (FORMAT JSON) INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'data');").
			WillReturnRows(mock.NewRows([]string{"QUERY PLAN"}).AddRow(`[{
			"Plan": {
				"Node Type": "Remote Fast Query Execution",
				"Node List": ["dn001", "dn002"]
			}
		}]`))
		mock.ExpectQuery(`
 SELECT nsp.nspname AS schema_name,c.relname AS child_name
                          FROM pg_class p1
                          JOIN pg_inherits i1 ON p1.oid = i1.inhparent
                          JOIN pg_class c ON i1.inhrelid = c.oid
                          JOIN pg_namespace nsp on p1.relnamespace = nsp.oid
                          WHERE nsp.nspname = $1 and p1.relname = $2;`).WithArgs("exist_db", "exist_tb_1").
			WillReturnRows(mock.NewRows([]string{"schema_name", "child_name"}).
				AddRow("exist_db", "exist_tb_sub_1").
				AddRow("exist_db", "exist_tb_sub_2"))
		mock.ExpectQuery(`EXECUTE DIRECT ON (dn001) 'SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = ''exist_db''
AND   c.relname IN (''exist_tb_sub_1'',''exist_tb_sub_2'');'`).
			WillReturnRows(mock.NewRows([]string{"nspname", "relname", "pg_relation_size"}).AddRow("exist_db", "exist_tb_1", "1"))
		mock.ExpectQuery(`EXECUTE DIRECT ON (dn001) 'SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = ''exist_db''
AND   c.relname IN (''exist_tb_sub_1'',''exist_tb_sub_2'');'`).
			WillReturnRows(mock.NewRows([]string{"nspname", "relname", "pg_relation_size"}).AddRow("exist_db", "exist_tb_1", "32"))
		mock.ExpectQuery(`EXECUTE DIRECT ON (dn001) 'SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = ''exist_db''
AND   c.relname IN (''exist_tb_sub_1'',''exist_tb_sub_2'');'`).
			WillReturnRows(mock.NewRows([]string{"nspname", "relname", "pg_relation_size"}).AddRow("exist_db", "exist_tb_1", "1"))
		mock.ExpectQuery(`EXECUTE DIRECT ON (dn002) 'SELECT nsp.nspname, c.relname, (PG_RELATION_SIZE(c.oid) / (1024 * 1024 * 1024))::NUMERIC AS pg_relation_size
FROM pg_class c
         JOIN pg_namespace nsp ON c.relnamespace = nsp.oid
WHERE nsp.nspname = ''exist_db''
AND   c.relname IN (''exist_tb_sub_1'',''exist_tb_sub_2'');'`).
			WillReturnRows(mock.NewRows([]string{"nspname", "relname", "pg_relation_size"}).AddRow("exist_db", "exist_tb_1", "1"))
	}

	// mock 规则审核器
	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, params, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

// ==== Rule test code end ====