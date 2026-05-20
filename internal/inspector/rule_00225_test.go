package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func setupForRuleSQLE00225(t *testing.T, ruleName string, sql string) (*driverImpl, context.Context) {
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

func TestRuleSQLE00225(t *testing.T) {
	ruleName := SQLE00225
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句，使用非分片键关联分片表，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 t3 JOIN exist_db.exist_tb_1 t1 ON t3.c1 = t1.id_shd_key;",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 1.1: SELECT 语句，使用分片键关联分片表，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 t3 JOIN exist_db.exist_tb_1 t1 ON t3.c3_shd_key = t1.id_shd_key;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 语句，使用非分片键关联分片表，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 t3, exist_db.exist_tb_1 t1 WHERE t3.c1 = t1.id_shd_key;",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2.1: SELECT 语句，使用分片键关联分片表，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 t3, exist_db.exist_tb_1 t1 WHERE t3.c3_shd_key = t1.id_shd_key;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: WITH 语句，使用非分片键关联分片表，涉及CTE表，不违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_1) SELECT * FROM exist_db.exist_tb_3 t3 JOIN cte ON t3.c3_shd_key = cte.c1_with_idx;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: WITH 语句，子句使用非分片键关联分片表，违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_3 t3, exist_db.exist_tb_1 t1 WHERE t3.c1 = t1.id_shd_key) SELECT * FROM exist_db.exist_tb_3 t3 JOIN cte ON t3.c1 = cte.c1_with_idx;",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: INSERT ... SELECT 语句，使用非分片键关联分片表，违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_3 (id, c1, c2, c3_shd_key) SELECT t1.id, t1.c1_with_idx, t1.c2_with_idx, 1 FROM exist_db.exist_tb_1 t1 JOIN exist_db.exist_tb_3 t3 ON t1.c1_with_idx = t3.c1;",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: UNION 语句，使用非分片键关联分片表，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 t3 JOIN exist_db.exist_tb_1 t1 ON t3.c1 = t1.c1_with_idx UNION SELECT * FROM exist_db.exist_tb_3 t3 JOIN exist_db.exist_tb_2 t2 ON t3.c1 = t2.c1;",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: SELECT 语句，使用分片键关联分片表，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 t3 JOIN exist_db.exist_tb_3 t3_2 ON t3.c3_shd_key = t3_2.c3_shd_key;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 8: DELETE 语句，使用非分片键关联分片表，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_3 WHERE c1 IN (SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE id_shd_key = 1);",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8.1: DELETE 语句，使用分片键关联分片表，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_3 WHERE c3_shd_key IN (SELECT id_shd_key FROM exist_db.exist_tb_1 WHERE id_shd_key = 1);",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 9: UPDATE 语句，使用非分片键关联分片表，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET c2 = 10000 WHERE c1 IN (SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE id_shd_key = 1);",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 9.1: UPDATE 语句，使用分片键关联分片表，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET c2 = 10000 WHERE c3_shd_key IN (SELECT id_shd_key FROM exist_db.exist_tb_1 WHERE id_shd_key = 1);",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00225(t, ruleName, test.sql)
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

// ==== Rule test code end ====
