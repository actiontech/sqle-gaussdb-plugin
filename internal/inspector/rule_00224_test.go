package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00224(t *testing.T) {
	ruleName := "SQLE00224"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "创建表，列名使用数字开头",
			sql:           "CREATE TABLE test_table (1column INTEGER, 2column TEXT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   true,
		},
		{
			desc:          "创建用户以pg开头",
			sql:           "CREATE USER pguser WITH PASSWORD 'password';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常创建用户名",
			sql:           "CREATE USER valid_user WITH PASSWORD 'password';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "创建角色以pg开头",
			sql:           "CREATE ROLE pgrole;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常创建角色名",
			sql:           "CREATE ROLE valid_role;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "创建自定义类型以pg开头",
			sql:           "CREATE TYPE pgtype AS (id INT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常创建自定义类型名",
			sql:           "CREATE TYPE valid_type AS (id INT);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "创建模式以pg开头",
			sql:           "CREATE SCHEMA pgschema;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常创建模式名",
			sql:           "CREATE SCHEMA valid_schema;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "创建表包含表级约束不应触发panic",
			sql:           "CREATE TABLE account_check_overview (id bigserial NOT NULL, business_type varchar(8) NULL, CONSTRAINT account_check_overview_pkey PRIMARY KEY (id));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "修改用户名为以pg开头",
			sql:           "ALTER USER exist_user RENAME TO pguser;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常修改用户名",
			sql:           "ALTER USER exist_user RENAME TO valid_user;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "修改角色名为以pg开头",
			sql:           "ALTER ROLE exist_role RENAME TO pgrole;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常修改角色名",
			sql:           "ALTER ROLE exist_role RENAME TO valid_role;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "修改自定义类型名为以pg开头",
			sql:           "ALTER TYPE exist_type RENAME TO pgtype;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常修改自定义类型名",
			sql:           "ALTER TYPE exist_type RENAME TO valid_type;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "修改模式名为以pg开头",
			sql:           "ALTER SCHEMA exist_schema RENAME TO pgschema;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "正常修改模式名",
			sql:           "ALTER SCHEMA exist_schema RENAME TO valid_schema;",
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
