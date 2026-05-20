package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00009(t *testing.T) {
	ruleName := SQLE00009
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE LOWER(v2) = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: INSERT...SELECT 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id, v1, v2) SELECT id, v1, v2 FROM exist_db.exist_tb_2 WHERE UPPER(v1) = 'TEST';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: UPDATE 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE LENGTH(v1) > 5;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: DELETE 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE TRIM(v1) = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: UNION ALL 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE LOWER(v1) = 'test' UNION ALL SELECT v1 FROM exist_db.exist_tb_2 WHERE UPPER(v1) = 'TEST';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: WITH 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_1 WHERE LOWER(v1) = 'test') SELECT * FROM cte;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: SELECT 语句中 WHERE 子句不使用函数操作，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 8: INSERT...SELECT 语句中 WHERE 子句不使用函数操作，不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id, v1, v2) SELECT id, v1, v2 FROM exist_db.exist_tb_2 WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 9: UPDATE 语句中 WHERE 子句不使用函数操作，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 10: DELETE 语句中 WHERE 子句不使用函数操作，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 11: UNION ALL 语句中 WHERE 子句不使用函数操作，不违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE v1 = 'test' UNION ALL SELECT v1 FROM exist_db.exist_tb_2 WHERE v1 = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 12: WITH 语句中 WHERE 子句不使用函数操作，不违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_1 WHERE v1 = 'test') SELECT * FROM cte;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 13: SQL 语法错误，无违规，报错",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE LOWER(v1) = 'test' UNION;",
			expectResults: nil,
			expectError:   true,
		},
		{
			desc:          "用例 14: INTERSECT 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE LOWER(v1) = 'test' INTERSECT SELECT v1 FROM exist_db.exist_tb_2 WHERE UPPER(v1) = 'TEST';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 15: EXCEPT 语句中 WHERE 子句使用函数操作，违反规则",
			sql:           "SELECT v1 FROM exist_db.exist_tb_1 WHERE LOWER(v1) = 'test' EXCEPT SELECT v1 FROM exist_db.exist_tb_2 WHERE UPPER(v1) = 'TEST';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 16: SELECT 语句中 WHERE 子句使用 LIKE 模糊搜索，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE LOWER(v1) LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 17: SELECT 语句中 WHERE 子句使用 LIKE 单字符通配符匹配，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE LOWER(v1) LIKE '_test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 18: SELECT 语句中 WHERE 子句使用 LIKE 字符集匹配，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE LOWER(v1) LIKE '[a-z]test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 19: SELECT 语句中 WHERE 子句带别名，违反规则",
			sql:           "SELECT e1.v1 AS alias_v1 FROM exist_db.exist_tb_1 AS e1 WHERE LOWER(e1.v1) = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 20: SELECT 语句中 WHERE 子句带别名，不违反规则",
			sql:           "SELECT e1.v1 AS alias_v1 FROM exist_db.exist_tb_1 AS e1 WHERE e1.v1 = 'test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 21: 相关子查询中 WHERE 子句使用函数操作，违反规则",
			sql:           "SELECT column1 FROM exist_db.exist_tb_1 AS t1 WHERE EXISTS (SELECT 1 FROM exist_db.exist_tb_2 AS t2 WHERE LOWER(t2.column2) = t1.column2);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 22: 相关子查询中 WHERE 子句不使用函数操作，不违反规则",
			sql:           "SELECT column1 FROM exist_db.exist_tb_1 AS t1 WHERE EXISTS (SELECT 1 FROM exist_db.exist_tb_2 AS t2 WHERE t2.column2 = t1.column2);",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleGeneral2(t, ruleName, _DefaultParams)
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
