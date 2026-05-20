package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00247(t *testing.T) {
	ruleName := "SQLE00247"
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
			desc:          "case 1: 创建表无分片关键字",
			sql:           "CREATE TABLE new_table_1 (id bigint PRIMARY KEY, name varchar);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 创建表含有分片关键字",
			sql:           "CREATE TABLE new_table_2 (id bigint PRIMARY KEY, name varchar) DISTRIBUTE BY SHARD(id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 创建表使用复制表关键字",
			sql:           "CREATE TABLE new_table_3 (id bigint PRIMARY KEY, name varchar) DISTRIBUTE BY REPLICATION;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 创建表无分片但有其他选项",
			sql:           "CREATE TABLE new_table_5 (id bigint PRIMARY KEY, name varchar) WITH (OIDS = FALSE);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 创建表无分片关键字但有其他选项",
			sql:           "CREATE TABLE new_table_7 (id bigint PRIMARY KEY, name varchar) WITH (OIDS = FALSE);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 创建表无分片关键字但有索引",
			sql:           "CREATE TABLE new_table_10 (id bigint PRIMARY KEY, name varchar); CREATE INDEX idx_name ON new_table_10(name);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 创建表含有分片关键字和索引",
			sql:           "CREATE TABLE new_table_11 (id bigint PRIMARY KEY, name varchar) DISTRIBUTE BY SHARD(id); CREATE INDEX idx_name ON new_table_11(name);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: 创建表使用复制表关键字和索引",
			sql:           "CREATE TABLE new_table_12 (id bigint PRIMARY KEY, name varchar) DISTRIBUTE BY REPLICATION; CREATE INDEX idx_name ON new_table_12(name);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: 创建表无分片关键字但有约束",
			sql:           "CREATE TABLE new_table_13 (id bigint PRIMARY KEY, name varchar, CONSTRAINT chk_name CHECK (name <> ''));",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 15: 创建表含有分片关键字和约束",
			sql:           "CREATE TABLE new_table_14 (id bigint PRIMARY KEY, name varchar, CONSTRAINT chk_name CHECK (name <> '')) DISTRIBUTE BY SHARD(id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 16: 创建表使用复制表关键字和约束",
			sql:           "CREATE TABLE new_table_15 (id bigint PRIMARY KEY, name varchar, CONSTRAINT chk_name CHECK (name <> '')) DISTRIBUTE BY REPLICATION;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
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
