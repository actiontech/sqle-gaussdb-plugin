package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00245(t *testing.T) {
	ruleName := SQLE00245

	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: 简单 UPDATE 语句，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'new_value' WHERE id_shd_key = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 3: 简单 DELETE 语句，不违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 4: DELETE 语句包含多表 USING，违反规则",
			sql:           "DELETE FROM exist_db.exist_tb_1 USING exist_db.exist_tb_2, exist_db.exist_tb_3 WHERE exist_db.exist_tb_1.id_shd_key = exist_db.exist_tb_2.id AND exist_db.exist_tb_2.c1 = exist_db.exist_tb_3.c1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: WITH 语句包含 UPDATE 且 FROM 子句包含多个表，违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_2) UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'new_value' FROM exist_db.exist_tb_2, exist_db.exist_tb_3 WHERE exist_db.exist_tb_1.id_shd_key = exist_db.exist_tb_2.id AND exist_db.exist_tb_2.c1 = exist_db.exist_tb_3.c1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: WITH 语句包含 UPDATE 且 FROM 子句包含单个表，不违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_2) UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'new_value' FROM exist_db.exist_tb_2 WHERE exist_db.exist_tb_1.id_shd_key = exist_db.exist_tb_2.id;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 7: WITH 语句包含 DELETE 且 USING 子句包含多个表，违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_2) DELETE FROM exist_db.exist_tb_1 USING exist_db.exist_tb_2, exist_db.exist_tb_3 WHERE exist_db.exist_tb_1.id_shd_key = exist_db.exist_tb_2.id AND exist_db.exist_tb_2.c1 = exist_db.exist_tb_3.c1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: WITH 语句包含 DELETE 且 USING 子句包含单个表，不违反规则",
			sql:           "WITH cte AS (SELECT * FROM exist_db.exist_tb_2) DELETE FROM exist_db.exist_tb_1 USING exist_db.exist_tb_2 WHERE exist_db.exist_tb_1.id_shd_key = exist_db.exist_tb_2.id;",
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
