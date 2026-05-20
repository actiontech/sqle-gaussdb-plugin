package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00053(t *testing.T) {
	ruleName := SQLE00053
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT * 违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 指定列名，不违反规则",
			sql:           "SELECT id, c1_with_idx FROM exist_db.exist_tb_1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 3: SELECT * with JOIN 违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 t1 JOIN exist_db.exist_tb_2 t2 ON t1.id = t2.id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: SELECT 指定列名 with JOIN 不违反规则",
			sql:           "SELECT t1.id, t2.c1 FROM exist_db.exist_tb_1 t1 JOIN exist_db.exist_tb_2 t2 ON t1.id = t2.id;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 5: SELECT * with UNION 违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 UNION SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: SELECT 指定列名 with UNION 不违反规则",
			sql:           "SELECT id, c1_with_idx FROM exist_db.exist_tb_1 UNION SELECT id, c1 FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 7: SELECT * with EXCEPT 违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 EXCEPT SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: SELECT 指定列名 with EXCEPT 不违反规则",
			sql:           "SELECT id, c1_with_idx FROM exist_db.exist_tb_1 EXCEPT SELECT id, c1 FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 9: SELECT * with INTERSECT 违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_1 INTERSECT SELECT * FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 10: SELECT 指定列名 with INTERSECT 不违反规则",
			sql:           "SELECT id, c1_with_idx FROM exist_db.exist_tb_1 INTERSECT SELECT id, c1 FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 11: SELECT * with 子查询 违反规则",
			sql:           "SELECT * FROM (SELECT * FROM exist_db.exist_tb_1) sub;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 12: SELECT 指定列名 with 子查询 不违反规则",
			sql:           "SELECT id, c1_with_idx FROM (SELECT id, c1_with_idx FROM exist_db.exist_tb_1) sub;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 13: DELETE 语句，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 14: INSERT 语句，不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'test');",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 15: UPDATE 语句，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' WHERE c1_with_idx = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 16: DROP TABLE 语句，不违反规则",
			sql:           "DROP TABLE exist_db.exist_tb_1;",
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
