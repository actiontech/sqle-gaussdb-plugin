package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func setupForRuleSQLE00099(t *testing.T, ruleName string, sql string) (*driverImpl, context.Context) {
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

func TestRuleSQLE00099(t *testing.T) {
	ruleName := SQLE00099
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句中存在 FOR UPDATE 操作，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 FOR UPDATE;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 语句中不存在 FOR UPDATE 操作，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: UNION 语句中不存在 FOR UPDATE 操作，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 UNION SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 5: 复杂查询中存在 FOR UPDATE 操作，违反规则",
			sql:           "SELECT * FROM (SELECT * FROM exist_db.exist_tb_1) t FOR UPDATE;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: 复杂查询中不存在 FOR UPDATE 操作，不违反规则",
			sql:           "SELECT * FROM (SELECT * FROM exist_db.exist_tb_1) t;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 7: SQL 语法错误，无违规，报错",
			sql:           `SELECT * FROM exist_db.exist_tb_1 UNION;`,
			expectResults: nil,
			expectError:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00099(t, ruleName, test.sql)
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
