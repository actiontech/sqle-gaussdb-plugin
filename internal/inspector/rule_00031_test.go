package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00031(t *testing.T) {
	ruleName := "SQLE00031"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 基本的视图创建",
			sql:           "CREATE VIEW test_view AS SELECT * FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 2: 带条件的视图创建",
			sql:           "CREATE VIEW test_view AS SELECT id FROM exist_tb_1 WHERE c1_with_idx = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 3: 嵌套视图创建",
			sql:           "CREATE VIEW test_view AS SELECT * FROM (SELECT * FROM exist_tb_1) AS subquery;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 4: 使用聚合函数的视图创建",
			sql:           "CREATE VIEW test_view AS SELECT COUNT(*) FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 5: 创建视图指定列名",
			sql:           "CREATE VIEW test_view (id, c1) AS SELECT id, c1_with_idx FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 6: 视图创建使用联接",
			sql:           "CREATE VIEW test_view AS SELECT t1.id, t2.c1 FROM exist_tb_1 t1 JOIN exist_tb_2 t2 ON t1.id = t2.user_id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 7: 视图创建使用子查询",
			sql:           "CREATE VIEW test_view AS SELECT id FROM (SELECT id FROM exist_tb_1) AS sub;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 8: 使用分区键的视图创建",
			sql:           "CREATE VIEW test_view AS SELECT * FROM exist_tb_3 WHERE c3_shd_key = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 9: 使用复杂条件的视图创建",
			sql:           "CREATE VIEW test_view AS SELECT * FROM exist_tb_1 WHERE c1_with_idx LIKE 'A%' AND c2_with_idx IS NOT NULL;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 10: 创建视图使用多表联接",
			sql:           "CREATE VIEW test_view AS SELECT t1.id, t2.c1 FROM exist_tb_1 t1, exist_tb_2 t2 WHERE t1.id = t2.user_id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 11: 基本的视图创建（正例）",
			sql:           "SELECT * FROM exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 12: 复杂查询（正例）",
			sql:           "SELECT COUNT(*) FROM exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 13: 子查询（正例）",
			sql:           "SELECT id FROM exist_tb_1 WHERE id IN (SELECT id FROM exist_tb_2);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 14: 联合查询（正例）",
			sql:           "SELECT t1.id, t2.c1 FROM exist_tb_1 t1 JOIN exist_tb_2 t2 ON t1.id = t2.user_id;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "case 15: 嵌套查询（正例）",
			sql:           "SELECT * FROM (SELECT * FROM exist_tb_1) AS subquery;",
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
