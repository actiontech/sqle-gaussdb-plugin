package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00223(t *testing.T) {
	ruleName := SQLE00223
	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: 更新非分片键，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE id = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 2: 更新分片键，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v3 = 10 WHERE id = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: 更新多个字段，其中包含分片键，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v1 = 'new_value', v3 = 10 WHERE id = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: 更新多个字段，不包含分片键，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v1 = 'new_value', v2 = 'new_value' WHERE id = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 5: 更新分片键，表没有分片键，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_2 SET user_id = 10 WHERE id = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 8: 更新非分片键，带别名，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 AS e1 SET e1.c1_with_idx = 'new_value' WHERE e1.id_shd_key = 1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 9: 更新分片键，LIKE 模糊搜索，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v3 = 10 WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 10: 更新非分片键，LIKE 模糊搜索，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE v1 LIKE '%test%';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 11: 更新分片键，单字符通配符匹配，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v3 = 10 WHERE v1 LIKE '_test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 12: 更新非分片键，单字符通配符匹配，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE v1 LIKE '_test';",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 13: 更新分片键，字符集匹配，违反规则",
			sql:           "UPDATE exist_db.exist_tb_3 SET v3 = 10 WHERE v1 LIKE '[a-z]%';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 14: 更新非分片键，字符集匹配，不违反规则",
			sql:           "UPDATE exist_db.exist_tb_1 SET v1 = 'new_value' WHERE v1 LIKE '[a-z]%';",
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
