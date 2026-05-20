package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00230(t *testing.T) {
	ruleName := SQLE00230
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
			desc:          "case 1: 插入中文到分片键字段(不指定列)",
			sql:           "INSERT INTO exist_tb_3 VALUES (1, 'value1', 'value2', '1111测试');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 1: 插入中文到分片键字段",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2, c3_shd_key) VALUES (1, 'value1', 'value2', '1111测试');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 2: 插入非中文到分片键字段",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2, c3_shd_key) VALUES (2, 'value1', 'value2', 123);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 3: 插入中文到非分片键字段",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2, c3_shd_key) VALUES (3, '测试', 'value2', 456);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 4: 插入中文到分片键字段，多列插入",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2, c3_shd_key) VALUES (4, 'value1', 'value2', '测试'), (5, 'value3', 'value4', 789);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 5: 插入非中文到分片键字段，多列插入",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2, c3_shd_key) VALUES (6, 'value5', 'value6', 101), (7, 'value7', 'value8', 202);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 6: 插入中文到分片键字段",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2, c3_shd_key) VALUES (1, 'value1', 'value1', '测试');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 7: 插入非中文到分片键字段",
			sql:           "INSERT INTO exist_tb_1 (id_shd_key, c1_with_idx, c2_with_idx) VALUES (2, 'value1', 'value2');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_1 表的分片键为 id_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 8: 插入中文到非分片键字段",
			sql:           "INSERT INTO exist_tb_3 (id, c1, c2, c3_shd_key) VALUES (3, '测试', 'value2', 456);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_3 表的分片键为 c3_shd_key",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 10: 插入非中文到分片键字段，多列插入",
			sql:           "INSERT INTO exist_tb_1 (id_shd_key, c1_with_idx, c2_with_idx) VALUES (6, 'value5', 'value6'), (7, 'value7', 'value8');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 exist_tb_1 表的分片键为 id_shd_key",
			mockRows:      "",
			mockSQL:       "",
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
