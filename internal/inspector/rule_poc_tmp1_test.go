package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

func TestRuleSQLEPOC01(t *testing.T) {
	ruleName := "SQLEPOC01"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockPlan      string
	}

	var maxScanRowCount = 10000

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "用例 1: Select查询,使用索引,扫描行数小于于maxScanRowCount",
			sql:           "Select id from exist_tb_2 where c1 = 1",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
			mockPlan: `[
  {
    "Plan": {
      "Node Type": "Remote Fast Query Execution",
      "Parallel Aware": false,
      "Startup Cost": 0.00,
      "Total Cost": 0.00,
      "Plan Rows": 0,
      "Plan Width": 0,
      "Node/s": "dn001"
      "Remote plan": [
        {
          "Plan": {
            "Node Type": "Index Scan",
            "Parallel Aware": false,
            "Scan Direction": "Forward",
            "Index Name": "big_table_pkey",
            "Relation Name": "big_table",
            "Alias": "big_table",
            "Startup Cost": 0.43,
            "Total Cost": 8.45,
            "Plan Rows": 1,
            "Plan Width": 45,
            "Index Cond": "(id = 1)"
          }
        }
      ]
    }
  }
]`,
		},
		{
			desc:          "用例 2: Select查询,使用索引,扫描行数大于maxScanRowCount",
			sql:           "Select id from exist_tb_2 where c1 = 1",
			expectResults: []*testResults{newTestResults().add(ruleName, maxScanRowCount)},
			expectError:   false,
			mockPlan: `[
  {
    "Plan": {
      "Node Type": "Remote Fast Query Execution",
      "Parallel Aware": false,
      "Startup Cost": 0.00,
      "Total Cost": 0.00,
      "Plan Rows": 0,
      "Plan Width": 0,
      "Node/s": "dn001"
      "Remote plan": [
        {
          "Plan": {
            "Node Type": "Index Scan",
            "Parallel Aware": false,
            "Scan Direction": "Forward",
            "Index Name": "big_table_pkey",
            "Relation Name": "big_table",
            "Alias": "big_table",
            "Startup Cost": 0.43,
            "Total Cost": 8.45,
            "Plan Rows": 20000,
            "Plan Width": 45,
            "Index Cond": "(id = 1)"
          }
        }
      ]
    }
  }
]`,
		},
		{
			desc:          "用例 2_1: Update,使用索引,扫描行数小于maxScanRowCount",
			sql:           "update exist_tb_2 set c1='zhangsan' where c1=2",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
			mockPlan: `[
        {
          "Plan": {
            "Node Type": "ModifyTable",
            "Operation": "Update",
            "Parallel Aware": false,
            "Relation Name":"exist_tb_2",
            "Alias":"exist_tb_2",
            "Startup Cost": 0.42,
            "Total Cost": 0.44,
            "Plan Rows": 0,
            "Plan Width": 0,
            "Plans": [
              {
                "Plan": {
                  "Node Type": "Index Scan",
                  "Parent Relationship":"outer",
                  "Parallel Aware": false,
                  "Scan Direction": "Forward",
                  "Index Name": "big_table_pkey",
                  "Relation Name": "exist_tb_2",
                  "Alias": "exist_tb_2",
                  "Startup Cost": 0.42,
                  "Total Cost": 8.43,
                  "Plan Rows": 1,
                  "Plan Width": 145,
                  "Index Cond": "(id = 1)"
                }
              }
            ]
          }
        }
]`,
		},
		{
			desc:          "用例 3: Select查询,未使用索引,扫描行数小于maxScanRowCount",
			sql:           "Select id from exist_tb_2 where c1 = 1",
			expectResults: []*testResults{newTestResults().add(ruleName, maxScanRowCount)},
			expectError:   false,
			mockPlan: `[
  {
    "Plan": {
      "Node Type": "Seq Scan",
      "Parallel Aware": false,
      "Startup Cost": 0.00,
      "Total Cost": 0.00,
      "Plan Rows": 25,
      "Plan Width": 0,
      "Node/s": "dn001"
    }
  }
]`,
		},
		{
			desc:          "用例 4: Select查询,使用索引,扫描行数大于maxScanRowCount",
			sql:           "Select id from exist_tb_2 where c1 = 1",
			expectResults: []*testResults{newTestResults().add(ruleName, maxScanRowCount)},
			expectError:   false,
			mockPlan: `[
  {
    "Plan": {
      "Node Type": "Seq Scan",
      "Parallel Aware": false,
      "Startup Cost": 0.00,
      "Total Cost": 0.00,
      "Plan Rows": 20000,
      "Plan Width": 0,
      "Node/s": "dn001"
    }
  }
]`,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context

			d, ctx = setupForRuleSQLE00002(t, ruleName, _DefaultParams, test.sql, test.mockPlan)

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

func setupForRuleSQLE00002(t *testing.T, ruleName string, params params.Params, sql, mockPlan string) (*driverImpl, context.Context) {
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
