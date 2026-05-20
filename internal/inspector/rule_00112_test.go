package inspector

import (
	"context"
	"log"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00112(t *testing.T) {
	ruleName := "SQLE00112"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockRows      interface{}
		mockSQL       string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 字段与常量类型不匹配",
			sql:           "SELECT * FROM exist_tb_1 WHERE c1_with_idx = 12345",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 字段与字段类型不匹配",
			sql:           "SELECT * FROM exist_tb_2 WHERE c1 = user_id",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 常量与常量类型不匹配",
			sql:           "SELECT * FROM exist_tb_1 WHERE 'text' = 123",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: 字段与常量类型匹配",
			sql:           "SELECT * FROM exist_tb_1 WHERE c1_with_idx = 'valid_text'",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 字段与字段类型匹配",
			sql:           "SELECT * FROM exist_tb_2 WHERE c1 = c2",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: 常量与常量类型匹配",
			sql:           "SELECT * FROM exist_tb_1 WHERE 'text' = 'another_text'",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 15: 日期类型字段与字符串类型不匹配",
			sql:           "SELECT * FROM exist_tb_3 WHERE c3_shd_key = '2024-02-01'",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 16: 日期类型字段与字符串类型匹配",
			sql:           "SELECT * FROM exist_tb_4 WHERE date_column = to_date('2024-02-01', 'YYYY-MM-DD')",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 17: 日期类型字段与CURRENT_DATE匹配",
			sql:           "SELECT * FROM exist_tb_4 WHERE date_column = CURRENT_DATE",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 18: 日期类型字段与CURRENT_DATE不匹配",
			sql:           "SELECT * FROM exist_tb_4 WHERE id_shd_key = CURRENT_DATE",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},

		{
			desc:          "case 10: 删除时字段与常量类型不匹配",
			sql:           "DELETE FROM exist_tb_1 WHERE id_shd_key = 'invalid_id'",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 删除时字段与字段类型不匹配",
			sql:           "DELETE FROM exist_tb_1 WHERE c1_with_idx = id_shd_key",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 删除时字段与常量类型匹配",
			sql:           "DELETE FROM exist_tb_1 WHERE id_shd_key = 100",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 19: 删除时日期类型字段与字符串类型不匹配",
			sql:           "DELETE FROM exist_tb_3 WHERE c3_shd_key = '2024-02-01'",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 20: 删除时日期类型字段与字符串类型匹配",
			sql:           "DELETE FROM exist_tb_4 WHERE date_column = to_date('2024-02-01', 'YYYY-MM-DD')",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: 插入数据时使用不匹配类型的 WHERE 子句",
			sql:           "INSERT INTO exist_tb_2 (id, c1, user_id) SELECT id, c1, user_id FROM exist_tb_1 WHERE c1_with_idx = 100",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: 插入数据时使用匹配类型的 WHERE 子句",
			sql:           "INSERT INTO exist_tb_2 (id, c1, user_id) SELECT id, c1, user_id FROM exist_tb_1 WHERE c1_with_idx = 'valid_text'",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 21: 插入数据时使用不匹配类型的 WHERE 子句",
			sql:           "INSERT INTO exist_tb_2 (id, c1, user_id) SELECT id, c1, user_id FROM exist_tb_3 WHERE c3_shd_key = '2024-02-01'",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 22: 插入数据时使用匹配类型的 WHERE 子句",
			sql:           "INSERT INTO exist_tb_2 (id, c1, user_id) SELECT id, c1, user_id FROM exist_tb_4 WHERE date_column = to_date('2024-02-01', 'YYYY-MM-DD')",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context
			log.Printf("======= %v ====== \n", test.desc)
			if test.mockDesc == "" {
				d, ctx = setupForRuleGeneral(t, ruleName, _DefaultParams)
			} else {
				d, ctx = setupForRuleSQLE00112WithMock(t, ruleName, _DefaultParams, test.mockRows, test.mockSQL)
			}

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

func setupForRuleSQLE00112WithMock(t *testing.T, ruleName string, params params.Params, mockRows interface{}, mockSQL string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 因为用例需要，在 mock 中加入需要从数据库中查询的数据
	mock.ExpectQuery(mockSQL).WillReturnRows(mockRows.(*sqlmock.Rows))

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
