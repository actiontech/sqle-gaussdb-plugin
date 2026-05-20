package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00238(t *testing.T) {
	ruleName := SQLE00238
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockRows      string
		mockSQL       string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 创建表时分片键数据类型符合要求",
			sql:           "CREATE TABLE test_tb_1 (id int, c1 bigint, c2 smallint, c3 char, name varchar(255)) distribute by shard(id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 创建表时分片键数据类型不符合要求",
			sql:           "CREATE TABLE test_tb_2 (id float, name varchar(255)) distribute by shard(id);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 创建表时未指定分片方式，第一个字段类型符合",
			sql:           "CREATE TABLE test_tb_3 (id int, name varchar(255));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: 创建表时未指定分片方式，第一个字段类型不符合",
			sql:           "CREATE TABLE test_tb_4 (id float, name varchar(255));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 创建表时分片键为非第一个字段且类型符合",
			sql:           "CREATE TABLE test_tb_5 (name varchar(255), id int) distribute by shard(id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: 创建表时分片键为非第一个字段且类型不符合",
			sql:           "CREATE TABLE test_tb_6 (name varchar(255), id float) distribute by shard(id);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 7: 创建表时分片键为复合类型且符合",
			sql:           "CREATE TABLE test_tb_7 (id int, name char(10)) distribute by shard(id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 创建表时分片键为复合类型且不符合",
			sql:           "CREATE TABLE test_tb_8 (id timestamp, name char(10)) distribute by shard(id);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 9: 创建表时使用已存在的表结构且分片键符合",
			sql:           "CREATE TABLE test_tb_9 (id bigint, c1_with_idx integer) distribute by shard(c1_with_idx);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 创建表时分片键为text类型，不符合要求",
			sql:           "CREATE TABLE test_tb_11 (id int, addr text) distribute by shard(addr);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 创建表时分片键为varchar类型，符合要求",
			sql:           "CREATE TABLE test_tb_12 (id int, log_date varchar(20)) distribute by shard(log_date);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: 创建表时分片键为bigint类型，符合要求",
			sql:           "CREATE TABLE test_tb_13 (id bigint, name varchar(255)) distribute by shard(id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: 创建表时未指定分片方式，第一个字段为bigint类型，符合要求",
			sql:           "CREATE TABLE test_tb_14 (id bigint, name varchar(255));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 15: 创建表时未指定分片方式，第一个字段为text类型，不符合要求",
			sql:           "CREATE TABLE test_tb_15 (addr text, name varchar(255));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 16: 创建表时分片键为存在表结构中的字段且符合要求",
			sql:           "CREATE TABLE test_tb_16 (id bigint, c3_with_idx integer) distribute by shard(c3_with_idx);",
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
