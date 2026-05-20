package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00015(t *testing.T) {
	ruleName := SQLE00015

	defaultCollate := "en_US.UTF-8"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 创建表时使用与数据库不一致的排序规则",
			sql:           "CREATE TABLE new_table_1 (id INT, name VARCHAR(100) COLLATE \"en_US\")",
			expectResults: []*testResults{newTestResults().add(ruleName, defaultCollate)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 创建表时使用与数据库一致的排序规则",
			sql:           "CREATE TABLE new_table_2 (id INT, name VARCHAR(100) COLLATE \"en_US.UTF-8\")",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 创建表时未指定排序规则",
			sql:           "CREATE TABLE new_table_3 (id INT, name VARCHAR(100))",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: 修改表时使用与数据库不一致的排序规则",
			sql:           "ALTER TABLE exist_tb_1 ALTER COLUMN c1_with_idx TYPE VARCHAR(100) COLLATE \"en_US\"",
			expectResults: []*testResults{newTestResults().add(ruleName, defaultCollate)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 修改表时使用与数据库一致的排序规则",
			sql:           "ALTER TABLE exist_tb_1 ALTER COLUMN c1_with_idx TYPE VARCHAR(100) COLLATE \"en_US.UTF-8\"",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: 修改表时未指定排序规则",
			sql:           "ALTER TABLE exist_tb_1 ALTER COLUMN c1_with_idx TYPE VARCHAR(100)",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 7: 创建索引时使用与数据库不一致的排序规则",
			sql:           "CREATE INDEX idx_name ON exist_tb_2 (c1 COLLATE \"en_US\")",
			expectResults: []*testResults{newTestResults().add(ruleName, defaultCollate)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 创建索引时使用与数据库一致的排序规则",
			sql:           "CREATE INDEX idx_name ON exist_tb_2 (c1 COLLATE \"en_US.UTF-8\")",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 9: 创建索引时未指定排序规则",
			sql:           "CREATE INDEX idx_name ON exist_tb_2 (c1)",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 10: 创建表时使用与数据库不一致的排序规则",
			sql:           "CREATE TABLE new_tb_4 (id INT, name VARCHAR(100) COLLATE \"en_US\")",
			expectResults: []*testResults{newTestResults().add(ruleName, defaultCollate)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 创建表时使用与数据库一致的排序规则",
			sql:           "CREATE TABLE exist_tb_5 (id INT, name VARCHAR(100) COLLATE \"en_US.UTF-8\")",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 创建表时未指定排序规则",
			sql:           "CREATE TABLE exist_tb_6 (id INT, name VARCHAR(100))",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: 修改表时使用与数据库不一致的排序规则",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c2 TYPE VARCHAR(100) COLLATE \"en_US\"",
			expectResults: []*testResults{newTestResults().add(ruleName, defaultCollate)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: 修改表时使用与数据库一致的排序规则",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c2 TYPE VARCHAR(100) COLLATE \"en_US.UTF-8\"",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 15: 修改表时未指定排序规则",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c2 TYPE VARCHAR(100)",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 16: 创建索引时使用与数据库不一致的排序规则",
			sql:           "CREATE INDEX idx_name_2 ON exist_tb_2 (c1 COLLATE \"en_US\")",
			expectResults: []*testResults{newTestResults().add(ruleName, defaultCollate)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 17: 创建索引时使用与数据库一致的排序规则",
			sql:           "CREATE INDEX idx_name_2 ON exist_tb_2 (c1 COLLATE \"en_US.UTF-8\")",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 18: 创建索引时未指定排序规则",
			sql:           "CREATE INDEX idx_name_2 ON exist_tb_2 (c1)",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
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
