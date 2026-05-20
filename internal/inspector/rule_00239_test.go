package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00239(t *testing.T) {
	ruleName := "SQLE00239"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockSQL       string
	}

	tests := []testCase{
		{
			desc:          "case 1: SELECT 语句使用数学运算在分区键上",
			sql:           "SELECT * FROM exist_tb_3 WHERE c3_shd_key + 1 = 10;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 的分区键为 c3_shd_key",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name = 'exist_tb_3'",
		},
		{
			desc:          "case 2: SELECT 语句使用函数在分区键上",
			sql:           "SELECT * FROM exist_tb_3 WHERE abs(c3_shd_key) = 5;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 的分区键为 c3_shd_key",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name = 'exist_tb_3'",
		},
		{
			desc:          "case 3: SELECT 语句在非分区键上使用数学运算",
			sql:           "SELECT * FROM exist_tb_1 WHERE c1_with_idx + 'test' = 'c1_with_idxtest';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_1 的分区键不包括 c1_with_idx",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name = 'exist_tb_1'",
		},
		{
			desc:          "case 8: UNION 语句中一个SELECT使用数学运算在分区键上",
			sql:           "SELECT * FROM exist_tb_3 WHERE c3_shd_key * 2 = 6 UNION SELECT * FROM exist_tb_1 WHERE c1_with_idx = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 的分区键为 c3_shd_key",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name = 'exist_tb_3'",
		},
		{
			desc:          "case 9: UNION 语句中所有SELECT均在非分区键上使用运算",
			sql:           "SELECT * FROM exist_tb_1 WHERE c1_with_idx + 'test' = 'c1_with_idxtest' UNION SELECT * FROM exist_tb_2 WHERE c1 + 'value' = 'c1value';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_1 和 exist_tb_2 的分区键不包括 c1_with_idx 和 c1",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name IN ('exist_tb_1', 'exist_tb_2')",
		},
		{
			desc:          "case 4: UPDATE 语句使用数学运算在分区键上",
			sql:           "UPDATE exist_tb_3 SET c1 = 'updated' WHERE c3_shd_key - 2 = 8;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 的分区键为 c3_shd_key",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name = 'exist_tb_3'",
		},
		{
			desc:          "case 5: UPDATE 语句在非分区键上使用函数",
			sql:           "UPDATE exist_tb_1 SET c2_with_idx = 'updated' WHERE upper(c1_with_idx) = 'C1_WITH_IDX';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_1 的分区键不包括 c1_with_idx",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name = 'exist_tb_1'",
		},
		{
			desc:          "case 6: DELETE 语句使用函数在分区键上",
			sql:           "DELETE FROM exist_tb_3 WHERE sqrt(c3_shd_key) = 3;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 的分区键为 c3_shd_key",
			mockSQL:       "SELECT partition_key FROM information_schema.tables WHERE table_name = 'exist_tb_3'",
		},
		{
			desc:          "case 7: DELETE 语句在非分区键上使用数学运算",
			sql:           "DELETE FROM exist_tb_2 WHERE id + 10 = 20;",
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
