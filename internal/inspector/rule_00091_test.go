package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func setupForRuleSQLE00091(t *testing.T, ruleName string, sql string) (*driverImpl, context.Context) {
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

func TestRuleSQLE00091(t *testing.T) {
	ruleName := SQLE00091
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1.1: DML 语句中的 SELECT 子句，带有连接条件，不违反规则",
			sql:           "SELECT a.id, b.c1 FROM exist_db.exist_tb_1 a JOIN exist_db.exist_tb_2 b ON a.id_shd_key = b.id;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 1.3: DML 语句中的 SELECT 子句，隐式Join，带有连接条件，不违反规则",
			sql:           "SELECT a.id, b.c1 FROM exist_db.exist_tb_1 a, exist_db.exist_tb_2 b WHERE a.id_shd_key = b.id;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 1.4: DML 语句中的 SELECT 子句，隐式Join，没有连接条件，违反规则",
			sql:           "SELECT a.id, b.c1 FROM exist_db.exist_tb_1 a, exist_db.exist_tb_2 b;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 1.5: INSERT 语句，不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'example');",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 1.6: INSERT 语句中的 SELECT 子句，不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, c1_with_idx FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 1.7: INSERT 语句中的 SELECT 子句，违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT a.id, b.c1 FROM exist_db.exist_tb_1 a, exist_db.exist_tb_2 b;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2.1: UNION 语句，所有 SELECT 子句都有连接条件，不违反规则",
			sql:           "SELECT a.id FROM exist_db.exist_tb_1 a JOIN exist_db.exist_tb_2 b ON a.id_shd_key = b.id UNION SELECT c.id FROM exist_db.exist_tb_3 c JOIN exist_db.exist_tb_2 d ON c.id = d.id;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 3.1: UPDATE 语句，带有连接条件，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' FROM exist_db.exist_tb_2 WHERE exist_tb_1.id_shd_key = exist_tb_2.id;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 3.2: UPDATE 语句，没有连接条件，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3.3: UPDATE 语句，连接条件恒真，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' FROM exist_db.exist_tb_2 WHERE 2 = 2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4.1: DELETE 语句，带有连接条件，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 USING exist_db.exist_tb_2 WHERE exist_tb_1.id_shd_key = exist_tb_2.id;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4.2: DELETE 语句，没有连接条件，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 USING exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4.3: DELETE 语句，连接条件恒真，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 USING exist_db.exist_tb_2 WHERE 1 = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: SELECT 语句，连接条件恒真，违反规则",
			sql:           "SELECT a.id, b.c1 FROM exist_db.exist_tb_1 a, exist_db.exist_tb_2 b WHERE 1 = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5.1: SELECT 语句，连接条件恒真，违反规则",
			sql:           "SELECT a.id, b.c1 FROM exist_db.exist_tb_1 a JOIN exist_db.exist_tb_2 b ON 1 = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00091(t, ruleName, test.sql)
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
