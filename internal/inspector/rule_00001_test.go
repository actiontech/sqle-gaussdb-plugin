package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00001(t *testing.T) {
	ruleName := SQLE00001

	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 0: select 语句有 where not exists ，不违反规则",
			sql:           "select * from exist_db.exist_tb_1 where not exists (select * from exist_db.exist_tb_1 where id = 1);",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 1: DELETE 语句没有 WHERE 条件，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: DELETE 语句有 WHERE 条件，但条件为恒真，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE 1=1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: DELETE 语句有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE id = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: UPDATE 语句没有 WHERE 条件，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: UPDATE 语句有 WHERE 条件，但条件为恒真，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE 1=1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: UPDATE 语句有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE id = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 7: SELECT 语句没有 WHERE 条件，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: SELECT 语句有 WHERE 条件，但条件为恒真，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE true=true;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 9: SELECT 语句有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 10: UNION 语句中一个子句没有 WHERE 条件，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 UNION SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 11: UNION 语句中所有子句都有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 UNION SELECT * FROM exist_db.exist_tb_2 WHERE id = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 12: SELECT 语句有 WHERE 条件，但条件为恒真，且有多个或组成的条件，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE 1=1 OR c1_with_idx = 'value';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 13: SELECT 语句有 WHERE 条件，且条件不为恒真，且有多个条件，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 AND c1_with_idx = 'value';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 14: UNION ALL 语句中一个子句没有 WHERE 条件，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 UNION ALL SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 15: UNION ALL 语句中所有子句都有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 UNION ALL SELECT * FROM exist_db.exist_tb_2 WHERE id = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 16: INTERSECT 语句中一个子句没有 WHERE 条件，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 INTERSECT SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 17: INTERSECT 语句中所有子句都有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 INTERSECT SELECT * FROM exist_db.exist_tb_2 WHERE id = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 18: EXCEPT 语句中一个子句没有 WHERE 条件，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 EXCEPT SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 19: EXCEPT 语句中所有子句都有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE id = 1 EXCEPT SELECT * FROM exist_db.exist_tb_2 WHERE id = 2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 20: SELECT 语句带别名，有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "SELECT e1.v1 AS alias_v1 FROM exist_db.exist_tb_1 AS e1 WHERE e1.v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 21: 相关子查询，有 WHERE 条件，且条件不为恒真，不违反规则",
			sql:           "SELECT column1 FROM exist_db.exist_tb_1 AS t1 WHERE EXISTS (SELECT 1 FROM exist_db.exist_tb_2 AS t2 WHERE t1.column2 = t2.column2);",
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
