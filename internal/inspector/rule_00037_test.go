package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00037(t *testing.T) {
	ruleName := "SQLE00037"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockSQL       string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "创建索引不超出限制",
			sql:           "CREATE INDEX new_idx_1 ON exist_tb_3 (c1);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "创建索引导致索引数量超出限制",
			sql:           "CREATE INDEX new_idx_2 ON exist_tb_1 (c1);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleWithMock(t, ruleName, _DefaultParams, test.mockSQL, test.mockDesc)
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

func setupForRuleWithMock(t *testing.T, ruleName string, params params.Params, mockSQL string, mockDesc string) (*driverImpl, context.Context) {
	e, _, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, params, "", currentSchema, mockPgCtx)
	for _, rh := range RuleHandlers {
		rule := rh.Rule
		if rule.Name == ruleName {
			rule.Params.SetParamValue(DefaultSingleParamKeyName, "2")
		}
	}

	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

// ==== Rule test code end ====
