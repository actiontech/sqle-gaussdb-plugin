package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00227(t *testing.T) {
	ruleName := SQLE00227
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockPlan      string
	}

	/*
		ATT：生成的单测中 mockPlan 格式错误，在 json 格式中，没有 Distribute results 字段，
		当前测试能通过的原因是因为在代码中是通过字符串匹配 Distribute results 字段进行判断，所以测试能通过
	*/
	tests := []testCase{
		{
			desc:          "用例 1: 查询单表，不跨分片",
			sql:           "SELECT * FROM exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Seq Scan", "Relation Name": "exist_tb_1"}}]`,
		},
		{
			desc:          "用例 2: 查询跨分片",
			sql:           "SELECT * FROM exist_tb_3 WHERE c3_shd_key = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 3: 查询多表连接，跨分片",
			sql:           "SELECT * FROM exist_tb_1 a JOIN exist_tb_3 b ON a.id_shd_key = b.c3_shd_key;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 4: 查询多表连接，不跨分片",
			sql:           "SELECT * FROM exist_tb_1 a JOIN exist_tb_2 b ON a.id_shd_key = b.id;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Hash Join", "Relation Name": "exist_tb_1"}}]`,
		},
		{
			desc:          "用例 5: 查询子查询，跨分片",
			sql:           "SELECT * FROM exist_tb_1 WHERE id_shd_key IN (SELECT id FROM exist_tb_3 WHERE c3_shd_key = 1);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 6: 查询子查询，不跨分片",
			sql:           "SELECT * FROM exist_tb_1 WHERE id_shd_key IN (SELECT id FROM exist_tb_2 WHERE id = 1);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Seq Scan", "Relation Name": "exist_tb_2"}}]`,
		},
		{
			desc:          "用例 7: 插入单表，不跨分片",
			sql:           "INSERT INTO exist_tb_1 (c1_with_idx, c2_with_idx) VALUES ('value1', 'value2');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Insert"}}]`,
		},
		{
			desc:          "用例 8: 插入多表，不跨分片",
			sql:           "INSERT INTO exist_tb_1 (c1_with_idx, c2_with_idx) SELECT c1, c2 FROM exist_tb_2;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Seq Scan", "Relation Name": "exist_tb_2"}}]`,
		},
		{
			desc:          "用例 9: 插入多表，跨分片",
			sql:           "INSERT INTO exist_tb_3 (c1, c2, c3_shd_key) SELECT c1, c2, id FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 10: 更新单表，不跨分片",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'new_value' WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Update"}}]`,
		},
		{
			desc:          "用例 11: 更新多表，不跨分片",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'new_value' FROM exist_tb_2 WHERE exist_tb_1.id_shd_key = exist_tb_2.id;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Hash Join", "Relation Name": "exist_tb_1"}}]`,
		},
		{
			desc:          "用例 12: 更新多表，跨分片",
			sql:           "UPDATE exist_tb_3 SET c1 = 'new_value' FROM exist_tb_1 WHERE exist_tb_3.c3_shd_key = exist_tb_1.id_shd_key;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 13: 删除单表，不跨分片",
			sql:           "DELETE FROM exist_tb_1 WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Delete"}}]`,
		},
		{
			desc:          "用例 14: 删除多表，不跨分片",
			sql:           "DELETE FROM exist_tb_1 USING exist_tb_2 WHERE exist_tb_1.id_shd_key = exist_tb_2.id;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Hash Join", "Relation Name": "exist_tb_1"}}]`,
		},
		{
			desc:          "用例 15: 删除多表，跨分片",
			sql:           "DELETE FROM exist_tb_3 USING exist_tb_1 WHERE exist_tb_3.c3_shd_key = exist_tb_1.id_shd_key;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 16: 查询多表连接，跨分片",
			sql:           "SELECT * FROM exist_tb_1 a JOIN exist_tb_3 b ON a.c1_with_idx = b.c1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 17: 查询多表连接，不跨分片",
			sql:           "SELECT * FROM exist_tb_2 a JOIN exist_tb_3 b ON a.id = b.id;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，不包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Hash Join", "Relation Name": "exist_tb_2"}}]`,
		},
		{
			desc:          "用例 18: 插入多表，跨分片",
			sql:           "INSERT INTO exist_tb_3 (c1, c2, c3_shd_key) SELECT c1, c2, user_id FROM exist_tb_2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 19: 更新多表，跨分片",
			sql:           "UPDATE exist_tb_3 SET c1 = 'new_value' FROM exist_tb_2 WHERE exist_tb_3.c3_shd_key = exist_tb_2.user_id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
		{
			desc:          "用例 20: 删除多表，跨分片",
			sql:           "DELETE FROM exist_tb_3 USING exist_tb_2 WHERE exist_tb_3.c3_shd_key = exist_tb_2.user_id;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain，包含 Distribute results",
			mockPlan:      `[{"Plan": {"Node Type": "Distribute results"}}]`,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00227(t, ruleName, _DefaultParams, test.sql, test.mockPlan)
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

func setupForRuleSQLE00227(t *testing.T, ruleName string, params params.Params, sql, mockPlan string) (*driverImpl, context.Context) {
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	mock.ExpectQuery("EXPLAIN " + sql).
		WillReturnRows(mock.NewRows([]string{"QUERY PLAN"}).AddRow(mockPlan))

	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, params, "", currentSchema, mockPgCtx)

	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

// ==== Rule test code end ====
