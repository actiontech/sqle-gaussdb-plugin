package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func setupForRuleSQLE00086(t *testing.T, ruleName string, sql string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, _, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// mock 规则审核器
	mockPgCtx := MockPGContext2(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, _DefaultParams, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

func TestRuleSQLE00086(t *testing.T) {
	ruleName := SQLE00086
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 语句中使用后缀匹配，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE v1 LIKE '%test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: SELECT 语句中使用前缀匹配，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE v1 LIKE 'test%';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: INSERT INTO 语句中使用子字符串匹配，违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id, v1, v2) SELECT id, v1, v2 FROM exist_db.exist_tb_2 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: UPDATE 语句中使用子字符串匹配，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: DELETE 语句中使用子字符串匹配，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: UNION 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE v1 LIKE '%test%' UNION SELECT v1 FROM exist_db.exist_tb_2 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: INTERSECT 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE v1 LIKE '%test%' INTERSECT SELECT v1 FROM exist_db.exist_tb_2 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 9: EXCEPT 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE v1 LIKE '%test%' EXCEPT SELECT v1 FROM exist_db.exist_tb_2 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 10: UNION ALL 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE v1 LIKE '%test%' UNION ALL SELECT v1 FROM exist_db.exist_tb_2 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 11: SELECT 语句中不使用 LIKE，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 13: SELECT 语句中使用字符集匹配，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE v1 LIKE '%[a-z]%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 14: 带别名的 SELECT 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT e1.v1 AS alias_v1 FROM exist_db.exist_tb_1 AS e1 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 15: 带别名的 INSERT INTO 语句中使用子字符串匹配，违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id, v1, v2) SELECT id, v1, v2 FROM exist_db.exist_tb_2 AS e2 WHERE e2.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 16: 带别名的 UPDATE 语句中使用子字符串匹配，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 AS e1 SET v1 = 'new_value' WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 17: 带别名的 DELETE 语句中使用子字符串匹配，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 AS e1 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 18: 子查询中使用子字符串匹配，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE v1 IN (SELECT v1 FROM exist_db.exist_tb_2 WHERE v1 LIKE '%test%');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 19: 相关子查询中使用子字符串匹配，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 AS e1 WHERE EXISTS (SELECT 1 FROM exist_db.exist_tb_2 AS e2 WHERE e1.v1 = e2.v1 AND e2.v1 LIKE '%test%');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 20: INNER JOIN 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT e1.v1, e2.v2 FROM exist_db.exist_tb_1 AS e1 INNER JOIN exist_db.exist_tb_2 AS e2 ON e1.v1 = e2.v1 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 21: LEFT JOIN 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT e1.v1, e2.v2 FROM exist_db.exist_tb_1 AS e1 LEFT JOIN exist_db.exist_tb_2 AS e2 ON e1.v1 = e2.v1 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 22: RIGHT JOIN 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT e1.v1, e2.v2 FROM exist_db.exist_tb_1 AS e1 RIGHT JOIN exist_db.exist_tb_2 AS e2 ON e1.v1 = e2.v1 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 23: FULL JOIN 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT e1.v1, e2.v2 FROM exist_db.exist_tb_1 AS e1 FULL JOIN exist_db.exist_tb_2 AS e2 ON e1.v1 = e2.v1 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 24: CROSS JOIN 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT e1.v1, e2.v2 FROM exist_db.exist_tb_1 AS e1 CROSS JOIN exist_db.exist_tb_2 AS e2 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 25: SELF JOIN 语句中使用子字符串匹配，违反规则",
			sql:           "SELECT a.v1, b.v2 FROM exist_db.exist_tb_1 AS a, exist_db.exist_tb_1 AS b WHERE a.v1 = b.v1 AND a.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00086(t, ruleName, test.sql)
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
