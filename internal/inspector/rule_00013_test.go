package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00013(t *testing.T) {
	ruleName := "SQLE00013"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
	}

	tests := []testCase{
		{
			desc:          "创建表使用 REAL 类型",
			sql:           "CREATE TABLE test_table_1 (id SERIAL, value REAL);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查是否包含 REAL 类型",
		},
		{
			desc:          "创建表使用 DOUBLE PRECISION 类型",
			sql:           "CREATE TABLE test_table_2 (id SERIAL, value DOUBLE PRECISION);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查是否包含 DOUBLE PRECISION 类型",
		},
		{
			desc:          "创建表使用 NUMERIC 类型",
			sql:           "CREATE TABLE test_table_3 (id SERIAL, value NUMERIC);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "检查是否仅包含 NUMERIC 类型",
		},
		{
			desc:          "创建表不使用浮点类型",
			sql:           "CREATE TABLE test_table_4 (id SERIAL, description TEXT);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "不包含 REAL 或 DOUBLE PRECISION 类型",
		},
		{
			desc:          "创建表同时使用 REAL 和 NUMERIC 类型",
			sql:           "CREATE TABLE test_table_5 (id SERIAL, value1 REAL, value2 NUMERIC);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查是否包含 REAL 类型",
		},
		{
			desc:          "修改表添加 REAL 类型的列",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN new_col REAL;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查添加的列类型",
		},
		{
			desc:          "修改表添加 DOUBLE PRECISION 类型的列",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN new_col DOUBLE PRECISION;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查添加的列类型",
		},
		{
			desc:          "修改表添加 NUMERIC 类型的列",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN new_col NUMERIC;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "检查添加的列类型",
		},
		{
			desc:          "修改表添加 INTEGER 类型的列",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN new_col INTEGER;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "检查添加的列类型",
		},
		{
			desc:          "修改表同时添加 REAL 和 INTEGER 类型的列",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN new_col1 REAL, ADD COLUMN new_col2 INTEGER;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查添加的列类型",
		},
		{
			desc:          "修改表将列类型改为 DOUBLE PRECISION",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN c1 TYPE DOUBLE PRECISION;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查修改的列类型",
		},
		{
			desc:          "修改表添加 DOUBLE PRECISION 类型的列",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN new_col DOUBLE PRECISION;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查添加的列类型",
		},
		{
			desc:          "修改表添加 REAL 类型的列",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN new_col REAL;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查添加的列类型",
		},
		{
			desc:          "创建表使用 DOUBLE PRECISION 类型",
			sql:           "CREATE TABLE test_table_6 (id SERIAL, value DOUBLE PRECISION);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查是否包含 DOUBLE PRECISION 类型",
		},
		{
			desc:          "创建表使用 REAL 类型",
			sql:           "CREATE TABLE test_table_7 (id SERIAL, value REAL);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "检查是否包含 REAL 类型",
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
