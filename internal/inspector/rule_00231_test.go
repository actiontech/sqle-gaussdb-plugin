package inspector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00231(t *testing.T) {
	ruleName := "SQLE00231"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockRows      []string
	}

	tests := []testCase{
		{
			desc:          "创建单层视图",
			sql:           "CREATE VIEW view1 AS SELECT * FROM exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 pg_views 无结果",
			mockRows:      nil,
		},
		{
			desc:          "创建嵌套视图，内层为表",
			sql:           "CREATE VIEW view2 AS SELECT * FROM exist_view_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 pg_views 返回 exist_view_1",
		},
		{
			desc:          "创建视图，内层为表和视图混合",
			sql:           "CREATE VIEW view5 AS SELECT a.*, b.* FROM exist_tb_2 a, exist_view_1 b;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 pg_views 返回 view1",
		},
		{
			desc:          "创建视图，使用子查询但不是视图",
			sql:           "CREATE VIEW view7 AS SELECT * FROM (SELECT * FROM exist_tb_3) AS subquery;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟查询 pg_views 无结果",
		},
		{
			desc:          "创建视图，嵌套已存在的视图和新视图",
			sql:           "CREATE VIEW view8 AS SELECT * FROM exist_view_1, (SELECT * FROM exist_tb_2) AS subquery;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟查询 pg_views 返回 view1",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context
			d, ctx = setupForRuleGeneral(t, ruleName, _DefaultParams)

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
