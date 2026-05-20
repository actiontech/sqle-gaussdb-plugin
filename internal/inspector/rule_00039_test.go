package inspector

import (
	"context"
	"fmt"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00039(t *testing.T) {
	ruleName := SQLE00039
	selectivityParam := params.Params{
		&params.Param{
			Key:   DefaultSingleParamKeyName,
			Value: "0.7",
			Desc:  "数据区分度",
			Type:  params.ParamTypeString,
		},
	}

	tests := []struct {
		desc          string
		sql           string
		mockData      map[string]float64
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc: "用例 1: CREATE INDEX, 字段区分度高，不违反规则",
			sql:  "CREATE INDEX idx_test ON exist_db.exist_tb_1 (c2_with_idx);",
			mockData: map[string]float64{
				"c2_with_idx": 0.8,
			},
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc: "用例 2: CREATE INDEX, 字段区分度低，违反规则",
			sql:  "CREATE INDEX idx_test ON exist_db.exist_tb_1 (c2_with_idx);",
			mockData: map[string]float64{
				"c2_with_idx": 0.6,
			},
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "c2_with_idx", 0.6)},
			expectError:   false,
		},
		{
			desc: "用例 3: SELECT, WHERE 条件字段区分度高，不违反规则",
			sql:  "SELECT * FROM exist_db.exist_tb_1 WHERE c1_with_idx = 'test';",
			mockData: map[string]float64{
				"c1_with_idx": 0.8,
			},
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc: "用例 4: SELECT, WHERE 条件字段区分度低，违反规则",
			sql:  "SELECT * FROM exist_db.exist_tb_1 WHERE c1_with_idx = 'test';",
			mockData: map[string]float64{
				"c1_with_idx": 0.6,
			},
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "c1_with_idx", 0.6)},
			expectError:   false,
		},
		{
			desc: "用例 5: UNION, 字段区分度高，不违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 UNION SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.8,
			},
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc: "用例 6: UNION, 字段区分度低，违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 UNION SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.6,
			},
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "c1_with_idx", 0.6)},
			expectError:   false,
		},
		{
			desc: "用例 7: UNION ALL, 字段区分度高，不违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 UNION ALL SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.8,
			},
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc: "用例 8: UNION ALL, 字段区分度低，违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 UNION ALL SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.6,
			},
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "c1_with_idx", 0.6)},
			expectError:   false,
		},
		{
			desc: "用例 9: INTERSECT, 字段区分度高，不违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 INTERSECT SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.8,
			},
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc: "用例 10: INTERSECT, 字段区分度低，违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 INTERSECT SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.6,
			},
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "c1_with_idx", 0.6)},
			expectError:   false,
		},
		{
			desc: "用例 11: EXCEPT, 字段区分度高，不违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 EXCEPT SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.8,
			},
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc: "用例 12: EXCEPT, 字段区分度低，违反规则",
			sql:  "SELECT c1_with_idx FROM exist_db.exist_tb_1 WHERE c1_with_idx = 1 EXCEPT SELECT c1_with_idx FROM exist_db.exist_tb_2 WHERE c1_with_idx = 1;",
			mockData: map[string]float64{
				"c1_with_idx": 0.6,
			},
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "c1_with_idx", 0.6)},
			expectError:   false,
		},
		{
			desc: "用例 13: SELECT 带别名, WHERE 条件字段区分度高，不违反规则",
			sql:  "SELECT e1.c1_with_idx AS alias_c1 FROM exist_db.exist_tb_1 AS e1 WHERE e1.c1_with_idx = 'test';",
			mockData: map[string]float64{
				"c1_with_idx": 0.8,
			},
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc: "用例 14: SELECT 带别名, WHERE 条件字段区分度低，违反规则",
			sql:  "SELECT e1.c1_with_idx AS alias_c1 FROM exist_db.exist_tb_1 AS e1 WHERE e1.c1_with_idx = 'test';",
			mockData: map[string]float64{
				"c1_with_idx": 0.6,
			},
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "c1_with_idx", 0.6)},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00039(t, ruleName, selectivityParam, test.sql, test.mockData)
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

func setupForRuleSQLE00039(t *testing.T, ruleName string, params params.Params, sql string, mockData map[string]float64) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	rows := mock.NewRows([]string{"total"}).AddRow(100)

	queryString := fmt.Sprintf(`SELECT COUNT(1) total FROM (SELECT * FROM "exist_db"."exist_tb_1" LIMIT 50000)`)
	mock.ExpectQuery(queryString).
		WillReturnRows(rows)

	queryString = fmt.Sprintf(`SELECT COUNT(1) total FROM (SELECT * FROM "exist_db"."exist_tb_2" LIMIT 50000`)
	mock.ExpectQuery(queryString).
		WillReturnRows(rows)

	for column, selectivity := range mockData {
		query := `SELECT COUNT(*) AS record_count 
		FROM (SELECT "%s" FROM "exist_db"."exist_tb_1" LIMIT 50000) AS limited 
		GROUP BY "%s" ORDER BY record_count DESC LIMIT 1`
		queryString := fmt.Sprintf(query, column, column)
		mock.ExpectQuery(queryString).
			WillReturnRows(mock.NewRows([]string{"record_count"}).AddRow((1 - selectivity) * 100))

		query = `SELECT COUNT(*) AS record_count 
			FROM (SELECT "%s" FROM "exist_db"."exist_tb_2" LIMIT 50000) AS limited 
			GROUP BY "%s" ORDER BY record_count DESC LIMIT 1`
		queryString = fmt.Sprintf(query, column, column)
		mock.ExpectQuery(queryString).
			WillReturnRows(mock.NewRows([]string{"record_count"}).AddRow((1 - selectivity) * 100))
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
