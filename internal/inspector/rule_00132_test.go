package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00132(t *testing.T) {
	ruleName := "SQLE00132"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 主查询含子查询",
			sql:           "SELECT * FROM exist_tb_1 WHERE id_shd_key IN (SELECT id FROM exist_tb_2);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 2: 子查询在JOIN条件",
			sql:           "SELECT * FROM exist_tb_1 JOIN exist_tb_2 ON exist_tb_1.id_shd_key = (SELECT MAX(id) FROM exist_tb_3);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 3: 子查询在SELECT字段",
			sql:           "SELECT (SELECT count(*) FROM exist_tb_2), c1_with_idx FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 4: 不含子查询",
			sql:           "SELECT c1_with_idx, c2_with_idx FROM exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 13: 子查询在EXISTS条件",
			sql:           "SELECT * FROM exist_tb_1 WHERE EXISTS (SELECT 1 FROM exist_tb_2 WHERE exist_tb_2.id = exist_tb_1.id_shd_key);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 14: 不含子查询",
			sql:           "SELECT * FROM exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 5: INSERT...SELECT含子查询",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2) SELECT id, c1, c2 FROM exist_tb_1 WHERE id_shd_key IN (SELECT id FROM exist_tb_2);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 6: INSERT...SELECT不含子查询",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2) SELECT id, c1, c2 FROM exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 15: INSERT...SELECT含子查询",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2) SELECT id, c1, c2 FROM exist_tb_1 WHERE EXISTS (SELECT 1 FROM exist_tb_2 WHERE exist_tb_2.id = exist_tb_1.id_shd_key);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 16: INSERT...SELECT不含子查询",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2) SELECT id, c1, c2 FROM exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 7: UPDATE含子查询",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE id_shd_key IN (SELECT id FROM exist_tb_2);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 8: UPDATE不含子查询",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE c2_with_idx IS NOT NULL;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 17: UPDATE含子查询",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE EXISTS (SELECT 1 FROM exist_tb_2 WHERE exist_tb_2.id = exist_tb_1.id_shd_key);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 18: UPDATE不含子查询",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 9: DELETE含子查询",
			sql:           "DELETE FROM exist_tb_1 WHERE id_shd_key IN (SELECT id FROM exist_tb_2);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 10: DELETE不含子查询",
			sql:           "DELETE FROM exist_tb_1 WHERE c2_with_idx IS NULL;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 19: DELETE含子查询",
			sql:           "DELETE FROM exist_tb_1 WHERE EXISTS (SELECT 1 FROM exist_tb_2 WHERE exist_tb_2.id = exist_tb_1.id_shd_key);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 20: DELETE不含子查询",
			sql:           "DELETE FROM exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 11: UNION ALL含子查询",
			sql:           "SELECT id FROM exist_tb_1 WHERE c1_with_idx = (SELECT c1 FROM exist_tb_3) UNION ALL SELECT id FROM exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 12: UNION ALL不含子查询",
			sql:           "SELECT id FROM exist_tb_1 UNION ALL SELECT id FROM exist_tb_2;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 21: UNION ALL含子查询",
			sql:           "SELECT id FROM exist_tb_1 WHERE EXISTS (SELECT 1 FROM exist_tb_2 WHERE exist_tb_2.id = exist_tb_1.id_shd_key) UNION ALL SELECT id FROM exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 22: UNION ALL不含子查询",
			sql:           "SELECT id FROM exist_tb_1 WHERE id_shd_key = 1 UNION ALL SELECT id FROM exist_tb_2;",
			expectResults: _RuleAuditPassResults,
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
