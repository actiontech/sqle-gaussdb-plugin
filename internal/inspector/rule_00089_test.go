package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00089(t *testing.T) {
	ruleName := SQLE00089

	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: INSERT INTO ... SELECT 违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, c1 FROM exist_db.exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: INSERT INTO ... VALUES 不违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) VALUES (1, 'test');",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 3: INSERT INTO ... SELECT with JOIN 违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT t2.id, t2.c1 FROM exist_db.exist_tb_2 t2 JOIN exist_db.exist_tb_3 t3 ON t2.id = t3.id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: INSERT INTO ... SELECT with WHERE 违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, c1 FROM exist_db.exist_tb_2 WHERE c2 = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: INSERT INTO ... SELECT with subquery 违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, (SELECT c1 FROM exist_db.exist_tb_3 WHERE id = t2.id) FROM exist_db.exist_tb_2 t2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: INSERT INTO ... SELECT with UNION 违反规则",
			sql:           "INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, c1 FROM exist_db.exist_tb_2 UNION SELECT id, c1 FROM exist_db.exist_tb_3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
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
