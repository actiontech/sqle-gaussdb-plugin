package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00055(t *testing.T) {
	ruleName := SQLE00055
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
			desc:          "case 1: 创建与现有索引字段完全相同的索引",
			sql:           "CREATE INDEX test_idx_1 ON exist_tb_1 (c1_with_idx, c2_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName).addWarningMessage("表[exist_db.exist_tb_1]上列[c1_with_idx,c2_with_idx]对应索引已存在")},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 2: 创建与现有索引字段顺序不同的索引",
			sql:           "CREATE INDEX test_idx_2 ON exist_tb_1 (c2_with_idx, c1_with_idx);",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 3: 创建与现有复合索引的前缀相同的索引",
			sql:           "CREATE INDEX test_idx_3 ON exist_tb_1 (c1_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName).addWarningMessage("表[exist_db.exist_tb_1]上列[c1_with_idx]对应索引已存在")},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 4: 创建与现有索引字段不完全相同的索引",
			sql:           "CREATE INDEX test_idx_4 ON exist_tb_1 (c1_with_idx, c2_with_idx, id_shd_key);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 5: 创建与现有索引完全无关的索引",
			sql:           "CREATE INDEX test_idx_5 ON exist_tb_2 (c1);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 6: 创建索引时包含函数（不应与现有函数索引冲突）",
			sql:           "CREATE INDEX test_idx_6 ON exist_tb_9 (lower(c2_with_idx));",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 7: 创建索引与现有函数索引字段相同但不包含函数",
			sql:           "CREATE INDEX test_idx_7 ON exist_tb_9 (c5_with_func_idx);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 9: 创建索引与现有唯一索引字段相同",
			sql:           "CREATE INDEX test_idx_9 ON exist_tb_2 (user_id);",
			expectResults: []*testResults{newTestResults().add(ruleName).addWarningMessage("表[exist_db.exist_tb_2]上列[user_id]对应索引已存在")},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 10: 创建索引与现有索引的子集相同",
			sql:           "CREATE INDEX test_idx_10 ON exist_tb_9 (c3_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName).addWarningMessage("表[exist_db.exist_tb_9]上列[c3_with_idx]对应索引已存在")},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 11: 创建索引与现有索引的超集相同",
			sql:           "CREATE INDEX test_idx_11 ON exist_tb_9 (c1_with_idx, c2_with_idx, c3_with_idx, c4_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName).addWarningMessage("表[exist_db.exist_tb_9]上列[c1_with_idx,c2_with_idx,c3_with_idx,c4_with_idx]对应索引已存在")},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 12: 创建索引在现有索引的基础上添加额外字段",
			sql:           "CREATE INDEX test_idx_12 ON exist_tb_9 (c1_with_idx, c2_with_idx, c3_with_idx, c4_with_idx, c5_with_func_idx);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 13: 创建与现有复合索引的前缀相同的索引",
			sql:           "CREATE INDEX test_idx_13 ON exist_tb_9 (c1_with_idx, c2_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 14: 创建与现有索引字段完全相同的索引",
			sql:           "CREATE INDEX test_idx_14 ON exist_tb_9 (c1_with_idx, c2_with_idx, c3_with_idx, c4_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName).addWarningMessage("表[exist_db.exist_tb_9]上列[c1_with_idx,c2_with_idx,c3_with_idx,c4_with_idx]对应索引已存在")},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 15: 创建与现有索引字段前缀相同的索引",
			sql:           "CREATE INDEX test_idx_15 ON exist_tb_9 (c1_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 16: 创建与现有索引字段不完全相同的索引",
			sql:           "CREATE INDEX test_idx_16 ON exist_tb_9 (c1_with_idx, c2_with_idx, c3_with_idx, c4_with_idx, c5_with_func_idx);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 17: 创建与现有索引完全无关的索引",
			sql:           "CREATE INDEX test_idx_17 ON exist_tb_9 (c5_with_func_idx);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context
			if test.mockDesc == "" {
				d, ctx = setupForRuleGeneral(t, ruleName, _DefaultParams)
			} else {
				d, ctx = setupForRuleSQLE00055(t, ruleName, _DefaultParams, test.mockSQL, test.mockRows)
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

func setupForRuleSQLE00055(t *testing.T, ruleName string, params params.Params, mockSQL string, mockRows string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 如果有 mock 数据，加入 mock 数据
	if mockSQL != "" && mockRows != "" {
		mock.ExpectQuery(mockSQL).WillReturnRows(mock.NewRows([]string{mockRows}))
	}

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
