package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func setupForRuleSQLE00221(t *testing.T, ruleName string, sql string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 在 mock 中加入需要从数据库中查询的数据
	mock.ExpectQuery("explain (FORMAT JSON) " + sql).
		WillReturnRows(mock.NewRows([]string{"QUERY PLAN"}).AddRow(`[{
			"Plan": {
				"Node Type": "Seq Scan",
				"Relation Name": "exist_tb_3",
				"Alias": "exist_tb_3",
				"Startup Cost": 0.00,
				"Total Cost": 35.50,
				"Plan Rows": 2550,
				"Plan Width": 100
			}
		}]`))

	// mock 规则审核器
	mockPgCtx := MockPGContext2(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, _DefaultParams, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

func TestRuleSQLE00221(t *testing.T) {
	ruleName := SQLE00221
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句没有 WHERE 子句，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 语句带 WHERE 子句但不包含分片键，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: SELECT 语句带 WHERE 子句且包含分片键，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: SELECT 语句带 WHERE 子句且对分片键使用了函数，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE abs(v3) = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: UNION 语句中 SELECT 子句没有 WHERE 子句，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 UNION SELECT * FROM exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: UNION 语句中所有 SELECT 子句都带有包含分片键的 WHERE 子句，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 1 UNION SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 7: UPDATE 语句没有 WHERE 子句，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v1 = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: UPDATE 语句带 WHERE 子句但不包含分片键，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v1 = 'test' WHERE v2 = 'value';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 9: UPDATE 语句带 WHERE 子句且包含分片键，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v1 = 'test' WHERE v3 = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 10: DELETE 语句没有 WHERE 子句，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 11: DELETE 语句带 WHERE 子句但不包含分片键，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_3 WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 12: DELETE 语句带 WHERE 子句且包含分片键，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_3 WHERE v3 = 123;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 13: DELETE 语句带 WHERE 子句且对分片键使用了函数，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_3 WHERE abs(v3) = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		// 补充用例一
		{
			desc:          "用例 14: SELECT 语句带 LIKE 模糊搜索且包含分片键，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 1 AND v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 15: UNION ALL 语句中所有 SELECT 子句都带有包含分片键的 WHERE 子句，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 1 UNION ALL SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 16: INTERSECT 语句中所有 SELECT 子句都带有包含分片键的 WHERE 子句，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 1 INTERSECT SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 17: EXCEPT 语句中所有 SELECT 子句都带有包含分片键的 WHERE 子句，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 1 EXCEPT SELECT * FROM exist_db.exist_tb_3 WHERE v3 = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 18: SELECT 语句带别名且包含分片键，不违反规则",
			sql:           "SELECT e1.v1 AS alias_v1 FROM exist_db.exist_tb_3 AS e1 WHERE e1.v3 = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00221(t, ruleName, test.sql)
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
