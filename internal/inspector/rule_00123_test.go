package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00123(t *testing.T) {
	ruleName := SQLE00123
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: 使用 TRUNCATE 操作，违反规则",
			sql:           "TRUNCATE TABLE exist_db.exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: 使用 TRUNCATE 操作，违反规则",
			sql:           "TRUNCATE TABLE exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: 使用 TRUNCATE 操作，违反规则",
			sql:           "TRUNCATE TABLE exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: 使用 TRUNCATE 操作，违反规则",
			sql:           "TRUNCATE TABLE exist_db.exist_tb_9;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: 不使用 TRUNCATE 操作，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 6: 不使用 TRUNCATE 操作，不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_2 (id, c1, c2, user_id) VALUES (1, 'test', 'test', 1);",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 7: 不使用 TRUNCATE 操作，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET c1 = 'updated' WHERE id = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 8: 不使用 TRUNCATE 操作，不违反规则",
			sql:           "SELECT * FROM exist_db.exist_tb_9;",
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
