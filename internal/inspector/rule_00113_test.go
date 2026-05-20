package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00113(t *testing.T) {
	ruleName := SQLE00113

	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句使用 NOT 关键字，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE c1_with_idx NOT LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 语句使用 != 操作符，违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE c1_with_idx != 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: INSERT 语句中的 SELECT 子句使用 NOT IN，违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, c1 FROM exist_db.exist_tb_2 WHERE c1 NOT IN ('test1', 'test2');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: UNION 语句中的 SELECT 子句使用 NOT EXISTS，违反规则",
			sql:           "SELECT id FROM exist_db.exist_tb_1 WHERE NOT EXISTS (SELECT 1 FROM exist_db.exist_tb_2 WHERE exist_tb_1.id = exist_tb_2.id) UNION SELECT id FROM exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: UPDATE 语句使用 NOT BETWEEN，违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' WHERE c1_with_idx NOT BETWEEN 'a' AND 'z';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: DELETE 语句使用 NOT LIKE，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE c1_with_idx NOT LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: WITH 语句中的 SELECT 子句使用 NOT IN，违反规则",
			sql:           "WITH temp AS (SELECT * FROM exist_db.exist_tb_1 WHERE c1_with_idx NOT IN ('test1', 'test2')) SELECT * FROM temp;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: SELECT 语句使用 IS NOT NULL，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 WHERE c1_with_idx IS NOT NULL;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 9: INSERT 语句中的 SELECT 子句使用 IS NOT NULL，不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, c1 FROM exist_db.exist_tb_2 WHERE c1 IS NOT NULL;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 10: UNION 语句中的 SELECT 子句使用 IS NOT NULL，不违反规则",
			sql:           "SELECT id FROM exist_db.exist_tb_1 WHERE c1_with_idx IS NOT NULL UNION SELECT id FROM exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleGeneral(t, ruleName, _DefaultParams)
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
