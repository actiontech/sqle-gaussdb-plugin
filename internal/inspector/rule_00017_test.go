package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00017(t *testing.T) {
	ruleName := "SQLE00017"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	tests := []testCase{
		{
			desc:          "用例 1: 创建表含 BYTEA 类型字段",
			sql:           "CREATE TABLE test_table_1 (id INT, data BYTEA);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: 创建表含 TEXT 类型字段",
			sql:           "CREATE TABLE test_table_2 (id INT, description TEXT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 3: 创建表不含 BYTEA 或 TEXT 类型字段",
			sql:           "CREATE TABLE test_table_3 (id INT, name VARCHAR(255));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 4: 创建表同时含 BYTEA 和 TEXT 类型字段",
			sql:           "CREATE TABLE test_table_4 (id INT, file BYTEA, notes TEXT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: 修改表添加 BYTEA 类型字段",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN new_data BYTEA;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: 修改表添加 TEXT 类型字段",
			sql:           "ALTER TABLE exist_tb_2 ADD COLUMN new_description TEXT;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: 修改表添加非 BYTEA 和 TEXT 类型字段",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN new_name VARCHAR(255);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 8: 修改表同时添加 BYTEA 和 TEXT 类型字段",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN file BYTEA, ADD COLUMN notes TEXT;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 9: 修改表更改字段类型为 BYTEA",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c1 TYPE BYTEA;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 10: 修改表更改字段类型为 TEXT",
			sql:           "ALTER TABLE exist_tb_3 ALTER COLUMN c2 TYPE TEXT;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 11: 修改表更改字段类型为非 BYTEA 和 TEXT",
			sql:           "ALTER TABLE exist_tb_1 ALTER COLUMN c1_with_idx TYPE VARCHAR(500);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 12: 修改表同时更改字段类型为 BYTEA 和 TEXT",
			sql:           "ALTER TABLE exist_tb_1 ALTER COLUMN c1_with_idx TYPE BYTEA, ALTER COLUMN c2_with_idx TYPE TEXT;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 13: 修改表添加多个非 BYTEA 和 TEXT 类型字段",
			sql:           "ALTER TABLE exist_tb_2 ADD COLUMN new_col1 VARCHAR(200), ADD COLUMN new_col2 INTEGER;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 14: 创建表含 BYTEA 和非 BYTEA/TEXT 类型字段",
			sql:           "CREATE TABLE test_table_5 (id INT, data BYTEA, info VARCHAR(100));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 15: 创建表含 TEXT 和非 BYTEA/TEXT 类型字段",
			sql:           "CREATE TABLE test_table_6 (id INT, description TEXT, info INTEGER);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 16: 创建表不含 BYTEA 或 TEXT 但有多个其他类型字段",
			sql:           "CREATE TABLE test_table_7 (id INT, name VARCHAR(255), age INTEGER);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 17: 修改表添加 BYTEA 和非 BYTEA/TEXT 类型字段",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN file BYTEA, ADD COLUMN age INTEGER;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 18: 修改表添加 TEXT 和非 BYTEA/TEXT 类型字段",
			sql:           "ALTER TABLE exist_tb_9 ADD COLUMN notes TEXT, ADD COLUMN score INTEGER;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 19: 修改表更改字段类型为非 BYTEA/TEXT 同时添加 BYTEA 字段",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c1 TYPE VARCHAR(300), ADD COLUMN file BYTEA;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 20: 修改表更改字段类型为非 BYTEA/TEXT 同时添加 TEXT 字段",
			sql:           "ALTER TABLE exist_tb_3 ALTER COLUMN c2 TYPE INTEGER, ADD COLUMN description TEXT;",
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
