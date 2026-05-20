package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00084(t *testing.T) {
	ruleName := "SQLE00084"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockPlan      string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "用例 1: 创建临时表，使用 TEMPORARY 关键字",
			sql:           "CREATE TEMPORARY TABLE temp_table (id INT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "无需模拟",
		},
		{
			desc:          "用例 2: 创建临时表，使用 TEMP 关键字",
			sql:           "CREATE TEMP TABLE temp_table (id INT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "无需模拟",
		},
		{
			desc:          "用例 3: 创建普通表，不使用 TEMPORARY 或 TEMP 关键字",
			sql:           "CREATE TABLE regular_table (id INT);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "无需模拟",
		},
		{
			desc:          "用例 4: 插入数据，不触发外部排序或磁盘操作",
			sql:           "INSERT INTO exist_tb_1 (c1_with_idx, c2_with_idx) VALUES ('test', 'test');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回无 external merge 或 disk 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Insert", "Plan Rows": 1}}]`,
		},
		{
			desc:          "用例 5: 插入数据，触发外部排序",
			sql:           "INSERT INTO exist_tb_1 (c1_with_idx, c2_with_idx) SELECT c1, c2 FROM exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 external merge 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external merge"}}]`,
		},
		{
			desc:          "用例 6: 插入数据，触发磁盘操作",
			sql:           "INSERT INTO exist_tb_1 (c1_with_idx, c2_with_idx) SELECT c1, c2 FROM exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 disk 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "disk"}}]`,
		},
		{
			desc:          "用例 7: 更新数据，不触发外部排序或磁盘操作",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回无 external merge 或 disk 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Update", "Plan Rows": 1}}]`,
		},
		{
			desc:          "用例 8: 更新数据，触发外部排序",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE c2_with_idx = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 external merge 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external merge"}}]`,
		},
		{
			desc:          "用例 9: 更新数据，触发磁盘操作",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE c2_with_idx = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 disk 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "disk"}}]`,
		},
		{
			desc:          "用例 10: 删除数据，不触发外部排序或磁盘操作",
			sql:           "DELETE FROM exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回无 external merge 或 disk 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Delete", "Plan Rows": 1}}]`,
		},
		{
			desc:          "用例 11: 删除数据，触发外部排序",
			sql:           "DELETE FROM exist_tb_1 WHERE c2_with_idx = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 external merge 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external merge"}}]`,
		},
		{
			desc:          "用例 12: 删除数据，触发磁盘操作",
			sql:           "DELETE FROM exist_tb_1 WHERE c2_with_idx = 'test';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 disk 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "disk"}}]`,
		},
		{
			desc:          "用例 13: 查询数据，触发外部排序",
			sql:           "SELECT * FROM exist_tb_1 ORDER BY c2_with_idx;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 external merge 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external merge"}}]`,
		},
		{
			desc:          "用例 14: 查询数据，触发磁盘操作",
			sql:           "SELECT * FROM exist_tb_1 ORDER BY c2_with_idx;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 disk 关键词",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "disk"}}]`,
		},
		{
			desc:          "用例 15: 查询数据，触发哈希连接",
			sql:           "SELECT * FROM exist_tb_1 JOIN exist_tb_2 ON exist_tb_1.id_shd_key = exist_tb_2.user_id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，返回 Parallel Hash 关键词，Buckets 或 Batches 大于 0",
			mockPlan:      `[{"Plan": {"Node Type": "Hash Join", "Hash Cond": "(exist_tb_1.id_shd_key = exist_tb_2.user_id)", "Plans": [{"Node Type": "Seq Scan", "Relation Name": "exist_tb_1"}, {"Node Type": "Hash", "Plans": [{"Node Type": "Hash", "Relation Name": "exist_tb_2", "Hash Buckets": 1024, "Hash Batches": 4}]}]}}]`,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context

			if test.mockDesc == "无需模拟" {
				d, ctx = setupForRuleGeneral(t, ruleName, _DefaultParams)
			} else {
				d, ctx = setupForRuleSQLE00084(t, ruleName, _DefaultParams, test.sql, test.mockPlan)
			}

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

func setupForRuleSQLE00084(t *testing.T, ruleName string, params params.Params, sql, mockPlan string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 因为用例需要，在 mock 中加入需要从数据库中查询的数据
	mock.ExpectQuery("EXPLAIN (FORMAT JSON) " + sql).WillReturnRows(mock.NewRows([]string{"Plan"}).AddRow(mockPlan))

	// mock 规则审核器
	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, params, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

// ==== Rule test code end ====
