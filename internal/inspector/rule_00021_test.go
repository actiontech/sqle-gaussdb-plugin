package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00021(t *testing.T) {
	ruleName := "SQLE00021"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "用例 1: 创建表时字段缺少 NOT NULL 约束",
			sql:           "CREATE TABLE test_tb_1 (id bigint, name varchar(255));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: 创建表时所有字段包含 NOT NULL 约束",
			sql:           "CREATE TABLE test_tb_2 (id bigint NOT NULL, name varchar(255) NOT NULL);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 3: 创建表时部分字段包含 NOT NULL 约束",
			sql:           "CREATE TABLE test_tb_3 (id bigint NOT NULL, name varchar(255));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: 创建表时字段包含 NOT NULL 约束和其他约束",
			sql:           "CREATE TABLE test_tb_4 (id bigint NOT NULL PRIMARY KEY, name varchar(255) NOT NULL UNIQUE);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 5: 添加新字段时缺少 NOT NULL 约束",
			sql:           "ALTER TABLE exist_tb_1 ADD column new_col varchar(255);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: 添加新字段时包含 NOT NULL 约束",
			sql:           "ALTER TABLE exist_tb_1 ADD column new_col varchar(255) NOT NULL;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 7: 修改字段时移除 NOT NULL 约束",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c1 DROP NOT NULL;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: 修改字段时不涉及 NOT NULL 约束",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c1 TYPE varchar(500);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 9: 修改字段时添加 NOT NULL 约束",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c2 SET NOT NULL;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 10: 创建表时首个字段自动 NOT NULL 但其他字段无 NOT NULL 约束",
			sql:           "CREATE TABLE customers (name varchar(32) NOT NULL, sex varchar(10), age int);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 11: 修改表时添加字段无 NOT NULL 约束",
			sql:           "ALTER TABLE exist_tb_1 ADD column addr varchar(20);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 12: 创建表时所有字段都有 NOT NULL 约束",
			sql:           "CREATE TABLE customers (name varchar(32) NOT NULL, sex varchar(10) NOT NULL, age int NOT NULL);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 13: 修改表时设置字段为 NOT NULL",
			sql:           "ALTER TABLE exist_tb_1 ALTER COLUMN c1 SET NOT NULL;",
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
