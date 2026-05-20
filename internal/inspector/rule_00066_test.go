package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00066(t *testing.T) {
	ruleName := SQLE00066

	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: ALTER TABLE DROP COLUMN 违反规则",
			sql:           "ALTER TABLE exist_db.exist_tb_1 DROP COLUMN c2_with_idx;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: ALTER TABLE DROP CONSTRAINT 违反规则",
			sql:           "ALTER TABLE exist_db.exist_tb_2 DROP CONSTRAINT pk_test_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: DROP TABLE 违反规则",
			sql:           "DROP TABLE exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: DROP INDEX 不违反规则",
			sql:           "DROP INDEX exist_db.exist_tb_9_idx_1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 6: DROP SCHEMA 违反规则",
			sql:           "DROP SCHEMA exist_db;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: DROP DATABASE 违反规则",
			sql:           "DROP DATABASE exist_db;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: SQL 语法错误，无违规，报错",
			sql:           "DROP FROM exist_db.exist_tb_1;",
			expectResults: nil,
			expectError:   true,
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
