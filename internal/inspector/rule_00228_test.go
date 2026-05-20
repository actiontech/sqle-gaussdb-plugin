package inspector

import (
	"context"
	"log"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00228(t *testing.T) {
	ruleName := SQLE00228
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
			desc:          "case 2: Update , 'Index Cond'和'Filter' 均没有分片键",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'new_data' WHERE id_shd_key = 1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "Simulate explain showing use of shard key in plan",
			mockPlan: `[
		{
			"Plan": {
			"Node Type": "Remote Fast Query Execution",
			"Parallel Aware": false,
			"Node/s": "dn001,dn002,dn003,dn004",
			"Remote Plan": "[\n {\n \"Plan\": {\n   \"Node Type\": \"ModifyTable\",\n    \"Operation\":\"Update\",\n    \"Parallel Aware\": false,\n    \"Relation Name\":\"t5\",\n     \"Alias\":\"t5\",\n    \"Startup Cost\":0.00,\n    \"Total Cost\": 0.01,\n    \"Plan Rows\": 0,\n    \"Plan Width\": 0,\n   \"Plans\": [\n     {\n       \"Node Type\":\"Seq Scan\",\n       \"Part Relationship\":\"Outer\",\n       \"Parallel Aware\": false,\n       \"Relation Name\":\"t5\",\n       \"Alias\":\"t5\",\n       \"Startup Cost\": 0.00,\n       \"Total Cost\": 0.01,\n       \"Plan Rows\": 1,\n       \"Plan Width\": 128,\n       \"Filter\":\"((c2)::text = 'b'::text)\"\n     }\n   ]\n }\n }\n ]"
				}
			}
			]`,
		},
		{
			desc:          "case 2: Update , 'Filter' 中有分片键",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'new_data' WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "Simulate explain showing use of shard key in plan",
			mockPlan: `[
				{
					"Plan": {
					"Node Type": "Remote Fast Query Execution",
					"Parallel Aware": false,
					"Node/s": "dn001,dn002,dn003,dn004",
					"Remote Plan": "[\n {\n \"Plan\": {\n   \"Node Type\": \"ModifyTable\",\n    \"Operation\":\"Update\",\n    \"Parallel Aware\": false,\n    \"Relation Name\":\"exist_tb_1\",\n     \"Alias\":\"exist_tb_1\",\n    \"Startup Cost\":0.00,\n    \"Total Cost\": 0.01,\n    \"Plan Rows\": 0,\n    \"Plan Width\": 0,\n   \"Plans\": [\n     {\n       \"Node Type\":\"Seq Scan\",\n       \"Part Relationship\":\"Outer\",\n       \"Parallel Aware\": false,\n       \"Relation Name\":\"exist_tb_1\",\n       \"Filter\":\"(id_shd_key)::text = 'b'::text)\",\n       \"Alias\":\"exist_tb_1\",\n       \"Startup Cost\": 0.00,\n       \"Total Cost\": 0.01,\n       \"Plan Rows\": 1,\n       \"Plan Width\": 128\"\n     }\n   ]\n }\n }\n ]"
						}
					}
					]`,
		},
		{
			desc:          "case 3: Update , 'Index Cond'均没有分片键",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'new_data' WHERE id_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "Simulate explain showing use of shard key in plan",
			mockPlan: `[
				{
					"Plan": {
					"Node Type": "Remote Fast Query Execution",
					"Parallel Aware": false,
					"Node/s": "dn001,dn002,dn003,dn004",
					"Remote Plan": "[\n {\n \"Plan\": {\n   \"Node Type\": \"ModifyTable\",\n    \"Operation\":\"Update\",\n    \"Parallel Aware\": false,\n    \"Relation Name\":\"exist_tb_1\",\n     \"Alias\":\"exist_tb_1\",\n    \"Startup Cost\":0.00,\n    \"Total Cost\": 0.01,\n    \"Plan Rows\": 0,\n    \"Plan Width\": 0,\n   \"Plans\": [\n     {\n       \"Node Type\":\"Seq Scan\",\n       \"Part Relationship\":\"Outer\",\n       \"Parallel Aware\": false,\n       \"Relation Name\":\"exist_tb_1\",\n       \"Index Cond\":\"id_shd_key\",\n       \"Alias\":\"exist_tb_1\",\n       \"Startup Cost\": 0.00,\n       \"Total Cost\": 0.01,\n       \"Plan Rows\": 1,\n       \"Plan Width\": 128,\n       \"Filter\":\"((c2)::text = 'b'::text)\"\n     }\n   ]\n }\n }\n ]"
						}
					}
					]`,
		},
		{
			desc:          "case 4: Select , 'Index Cond'中有分片键",
			sql:           "SELECT * FROM exist_tb_3 WHERE c3_shd_key = 1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "Simulate explain showing use of shard key in selection plan",
			mockPlan: `[
				{
				"Plan": {
				"Node Type": "Remote Fast Query Execution",
				"Parallel Aware": false,
				"Node/s": "dn001,dn002,dn003,dn004",
				"Remote Plan": "[\n {\n \"Plan\": {\n   \"Node Type\": \"ModifyTable\",\n    \"Operation\":\"Select\",\n    \"Parallel Aware\": false,\n    \"Relation Name\":\"exist_tb_3\",\n     \"Alias\":\"exist_tb_3\",\n    \"Startup Cost\":0.00,\n    \"Total Cost\": 0.01,\n    \"Plan Rows\": 0,\n    \"Plan Width\": 0,\n   \"Plans\": [\n     {\n       \"Node Type\":\"Seq Scan\",\n       \"Part Relationship\":\"Outer\",\n       \"Parallel Aware\": false,\n       \"Relation Name\":\"exist_tb_3\",\n       \"Index Cond\":\"c3_shd_key\",\n       \"Alias\":\"exist_tb_3\",\n       \"Startup Cost\": 0.00,\n       \"Total Cost\": 0.01,\n       \"Plan Rows\": 1,\n       \"Plan Width\": 128,\n       \"Filter\":\"((c2)::text = 'b'::text)\"\n     }\n   ]\n }\n }\n ]"
					}
				}
				]`,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			log.Printf("======= %v ======\n", test.desc)
			d, ctx := setupForRuleSQLE00228(t, ruleName, _DefaultParams, test.sql, test.mockPlan)
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

func setupForRuleSQLE00228(t *testing.T, ruleName string, params params.Params, sql, mockPlan string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 因为用例需要，在 mock 中加入需要从数据库中查询的数据
	mock.ExpectQuery("EXPLAIN (FORMAT JSON) " + sql).
		WillReturnRows(mock.NewRows([]string{"QUERY PLAN"}).AddRow(mockPlan))

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
