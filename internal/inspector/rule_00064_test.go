package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00064(t *testing.T) {
	ruleName := "SQLE00064"
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
			desc:          "case 1: 创建索引时使用VARCHAR字段长度超过2000",
			sql:           "CREATE INDEX test_idx_1 ON exist_tb_1 (c2_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 创建索引时使用VARCHAR字段长度不超过2000",
			sql:           "CREATE INDEX test_idx_2 ON exist_tb_3 (c3);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 创建表时主键为VARCHAR字段长度超过2000",
			sql:           "CREATE TABLE exist_tb_new (id bigint, name character varying(3000), PRIMARY KEY (name));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: 创建表时主键为VARCHAR字段长度不超过2000",
			sql:           "CREATE TABLE exist_tb_new (id bigint, name character varying(1500), PRIMARY KEY (name));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 修改表添加主键时，主键为VARCHAR字段长度超过2000",
			sql:           "ALTER TABLE exist_tb_2 ADD COLUMN new_col character varying(2500), ADD PRIMARY KEY (new_col);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: 修改表添加主键时，主键为VARCHAR字段长度不超过2000",
			sql:           "ALTER TABLE exist_tb_2 ADD COLUMN new_col character varying(1000), ADD PRIMARY KEY (new_col);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 7: 修改表添加主键约束时，涉及的VARCHAR字段长度超过2000",
			sql:           "ALTER TABLE exist_tb_1 ADD PRIMARY KEY (c2_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 修改表添加主键约束时，涉及的VARCHAR字段长度不超过2000",
			sql:           "ALTER TABLE exist_tb_1 ADD PRIMARY KEY (c1_with_idx);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 9: 创建索引时使用VARCHAR字段长度超过2000",
			sql:           "CREATE INDEX test_idx_3 ON exist_tb_2 (c2);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 10: 创建索引时使用VARCHAR字段长度不超过2000",
			sql:           "CREATE INDEX test_idx_4 ON exist_tb_2 (c1);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 创建表时主键为VARCHAR字段长度超过2000",
			sql:           "CREATE TABLE exist_tb_new_2 (id bigint, name character varying(2500), PRIMARY KEY (name));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 创建表时主键为VARCHAR字段长度不超过2000",
			sql:           "CREATE TABLE exist_tb_new_2 (id bigint, name character varying(1000), PRIMARY KEY (name));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: 修改表添加主键时，主键为VARCHAR字段长度超过2000",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN new_col character varying(3000), ADD PRIMARY KEY (new_col);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: 修改表添加主键时，主键为VARCHAR字段长度不超过2000",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN new_col character varying(1500), ADD PRIMARY KEY (new_col);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 15: 修改表添加主键约束时，涉及的VARCHAR字段长度超过2000",
			sql:           "ALTER TABLE exist_tb_9 ADD PRIMARY KEY (c2_with_idx);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 16: 修改表添加主键约束时，涉及的VARCHAR字段长度不超过2000",
			sql:           "ALTER TABLE exist_tb_9 ADD PRIMARY KEY (c1_with_idx);",
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
