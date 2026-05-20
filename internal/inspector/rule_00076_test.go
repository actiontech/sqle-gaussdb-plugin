package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func setupForRuleSQLE00076(t *testing.T, ruleName string, sql, mockSQL string, row int) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 在 mock 中加入需要从数据库中查询的数据
	mock.ExpectQuery(mockSQL).WillReturnRows(mock.NewRows([]string{"count"}).AddRow(row))

	// mock 规则审核器
	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, _DefaultParams, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	return d, ctx
}

func TestRuleSQLE00076(t *testing.T) {
	ruleName := SQLE00076
	tests := []struct {
		desc          string
		sql           string
		mockSQL       string
		Row           int
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: UPDATE 操作影响行数不超过阈值，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET id_shd_key = 'updated'",
			mockSQL:       `SELECT count(*) AS count FROM exist_db.exist_tb_1`,
			Row:           1000,
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 2: UPDATE 操作影响行数超过阈值，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET id_shd_key = 'updated' WHERE id_shd_key < 200000;",
			mockSQL:       `SELECT count(*) AS count FROM exist_db.exist_tb_1 WHERE id_shd_key < 200000`,
			Row:           199999,
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: DELETE 操作影响行数不超过阈值，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE id_shd_key < 50000;",
			mockSQL:       `SELECT count(*) AS count FROM exist_db.exist_tb_1 WHERE id_shd_key < 50000;`,
			Row:           20,
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: DELETE 操作影响行数超过阈值，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE id_shd_key < 150000;",
			mockSQL:       `SELECT count(*) AS count FROM exist_db.exist_tb_1 WHERE id_shd_key < 150000`,
			Row:           150000,
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: UPDATE 操作影响行数刚好等于阈值，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET id_shd_key = 'updated' WHERE id_shd_key < 100000",
			mockSQL:       `SELECT count(*) AS count FROM exist_db.exist_tb_1 WHERE id_shd_key < 100000`,
			Row:           99,
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 6: DELETE 操作影响行数刚好等于阈值，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE id_shd_key < 100000;",
			mockSQL:       `SELECT count(*) AS count FROM exist_db.exist_tb_1 WHERE id_shd_key < 100000`,
			Row:           1,
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00076(t, ruleName, test.sql, test.mockSQL, test.Row)
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
