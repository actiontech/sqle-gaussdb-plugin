package inspector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00060(t *testing.T) {
	ruleName := "SQLE00060"
	type testCase struct {
		desc          string
		sql           []string
		expectResults []*testResults
		expectError   bool
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 创建表没有注释",
			sql:           []string{"CREATE TABLE new_table_1 (id integer, data text);"},
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 2: 创建表有注释",
			sql:           []string{"CREATE TABLE new_table_2 (id integer, data text);", "COMMENT ON TABLE new_table_2 IS 'This is a table for storing data.';"},
			expectResults: []*testResults{newTestResults(), newTestResults()},
			expectError:   false,
		},
		{
			desc:          "case 2: 创建表有非当前表注释",
			sql:           []string{"CREATE TABLE new_table_2 (id integer, data text);", "COMMENT ON TABLE new_table_ttt IS 'This is a table for storing data.';"},
			expectResults: []*testResults{newTestResults().add(ruleName), newTestResults()},
			expectError:   false,
		},
		{
			desc:          "case 4: 创建带schema的表有注释",
			sql:           []string{"CREATE TABLE exist_db.exist_tb_t1 (id bigint);", "COMMENT ON TABLE exist_db.exist_tb_t1 IS 'This is a table for storing data.';"},
			expectResults: []*testResults{newTestResults(), newTestResults()},
			expectError:   false,
		},
	}

	// 执行测试
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			d, ctx := setupForRuleGeneral(t, ruleName, _DefaultParams)
			results, err := d.Audit(ctx, tc.sql)
			if tc.expectError {
				assert.Error(t, err, tc.desc)
			} else {
				assert.NoError(t, err, tc.desc)
				assertRuleResult(t, tc.desc, strings.Join(tc.sql, "\n"), tc.expectResults, results)
			}
		})
	}
}

// ==== Rule test code end ====
