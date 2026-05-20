package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00008(t *testing.T) {
	ruleName := SQLE00008
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	tests := []testCase{
		{
			desc:          "用例 1: 创建表时未指定主键",
			sql:           "CREATE TABLE new_table (id integer, name text);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: 创建表时正确指定主键",
			sql:           "CREATE TABLE new_table (id integer PRIMARY KEY, name text);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 3: 创建表时通过约束指定主键",
			sql:           "CREATE TABLE new_table (id integer, name text, CONSTRAINT pk PRIMARY KEY (id));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 4: 创建表时主键在复合键中",
			sql:           "CREATE TABLE new_table (id integer, name text, PRIMARY KEY (id, name));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 5: 创建表时主键和其他约束混合",
			sql:           "CREATE TABLE new_table (id integer PRIMARY KEY, name text UNIQUE);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 6: 删除表中的主键",
			sql:           "ALTER TABLE exist_tb_1 DROP CONSTRAINT exist_tb_1_pkey;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: 删除表中的非主键约束",
			sql:           "ALTER TABLE exist_tb_2 DROP CONSTRAINT exist_tb_2_user_id_key;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 8: 删除表中的列，不涉及主键",
			sql:           "ALTER TABLE exist_tb_3 DROP COLUMN c2;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 9: 删除表中的主键列",
			sql:           "ALTER TABLE exist_tb_3 DROP COLUMN id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 10: 添加新的主键约束",
			sql:           "ALTER TABLE exist_tb_9 ADD PRIMARY KEY (id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 11: 修改表结构但不涉及主键",
			sql:           "ALTER TABLE exist_tb_2 ADD COLUMN new_col text;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 12: 创建表时未指定主键，分布键指定",
			sql:           "CREATE TABLE customers(id int, name varchar(32) NOT NULL DEFAULT '', sex int DEFAULT 0, age int DEFAULT 0) DISTRIBUTE BY SHARD(id);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 13: 创建表时未指定主键，且是分区子表",
			sql:           "CREATE TABLE t1_202208 PARTITION OF t1 FOR VALUES FROM ('2022-08-01 00:00:00') TO ('2022-09-01 00:00:00')",
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
