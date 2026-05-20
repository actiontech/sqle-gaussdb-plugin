package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00110(t *testing.T) {
	ruleName := SQLE00110
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句，where 子句字段有索引，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE c1_with_idx = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 语句，where 子句字段无索引，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_2 WHERE c1 = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: SELECT 语句，group by 子句字段有索引，不违反规则",
			sql:           "SELECT c1_with_idx, COUNT(*) FROM exist_db.exist_tb_1 GROUP BY c1_with_idx;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: SELECT 语句，order by 子句字段无索引，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 ORDER BY c1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: DELETE 语句，where 子句字段有索引，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE c1_with_idx = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 6: DELETE 语句，where 子句字段无索引，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_3 WHERE c1 = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: INSERT 语句，不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'test');",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 8: UPDATE 语句，where 子句字段有索引，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' WHERE c1_with_idx = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 9: UPDATE 语句，where 子句字段无索引，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET c1 = 'updated' WHERE c2 = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 10: UNION 语句，所有子查询字段有索引，不违反规则",
			sql:           "SELECT c1_with_idx FROM exist_db.exist_tb_1 UNION SELECT c1_with_idx FROM exist_db.exist_tb_9;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 13: WITH 语句内子查询无索引，违反规则",
			sql:           "WITH tmp(v5) AS (SELECT v5 FROM exist_db.exist_tb_9 WHERE v5 = 'test') SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 14: WITH 语句内子查询有索引，不违反规则",
			sql:           "WITH tmp(id) AS (SELECT id FROM exist_db.exist_tb_1 WHERE id = 1) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 15: WITH 语句内的子查询使用 GROUP BY 有索引，不违反规则",
			sql:           "WITH tmp(v1, cnt) AS (SELECT v1, COUNT(*) FROM exist_db.exist_tb_1 GROUP BY c1_with_idx) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 16: WITH 语句内的子查询使用 GROUP BY 无索引，违反规则",
			sql:           "WITH tmp(v5, cnt) AS (SELECT v5, COUNT(*) FROM exist_db.exist_tb_9 GROUP BY v5) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 17: WITH 语句内的子查询使用 ORDER BY 有索引，不违反规则",
			sql:           "WITH tmp AS (SELECT * FROM exist_db.exist_tb_1 ORDER BY id) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 18: WITH 语句内的子查询使用 ORDER BY 无索引，违反规则",
			sql:           "WITH tmp AS (SELECT * FROM exist_db.exist_tb_9 ORDER BY v5) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 19: WITH 语句内的子查询 UNION 有索引，不违反规则",
			sql:           "WITH tmp AS (SELECT v1 FROM exist_db.exist_tb_1 where id = 1 UNION SELECT v1 FROM exist_db.exist_tb_1) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 20: WITH 语句内的子查询 UNION 没有使用查询条件，不违反规则",
			sql:           "WITH tmp AS (SELECT v2 FROM exist_db.exist_tb_1 UNION SELECT v2 FROM exist_db.exist_tb_1) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00110(t, ruleName, test.sql)
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

func setupForRuleSQLE00110(t *testing.T, ruleName, sql string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, _, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// mock 规则审核器
	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, _DefaultParams, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

// ==== Rule test code end ====
