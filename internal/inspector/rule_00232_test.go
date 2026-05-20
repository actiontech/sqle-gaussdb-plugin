package inspector

import (
	"context"
	"strconv"
	"testing"

	mock "github.com/DATA-DOG/go-sqlmock"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00232(t *testing.T) {
	ruleName := SQLE00232
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockRows      *mock.Rows
		mockSQL       string
	}

	mockRowsFn := func(n int) *mock.Rows {
		mockRows := mock.NewRows([]string{"relname"})
		for i := 0; i < n; i++ {
			mockRows.AddRow(strconv.Itoa(i))
		}
		return mockRows
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "创建表无分区",
			sql:           "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "创建表按天分区超过阈值",
			sql:           "CREATE TABLE t_time_range (f1 bigint, f2 timestamp, f3 bigint) PARTITION BY RANGE (f2) BEGIN (timestamp without time zone '2024-01-01 0:0:0') STEP (interval '1 day') PARTITIONS (366) DISTRIBUTE BY SHARD(f1);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "创建表按月分区未超过阈值",
			sql:           "CREATE TABLE t_time_range2 (f1 bigint, f2 timestamp, f3 bigint) PARTITION BY RANGE (f2) BEGIN (timestamp without time zone '2024-01-01 0:0:0') STEP (interval '1 month') PARTITIONS (12) DISTRIBUTE BY SHARD(f1);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},

		{
			desc:          "case 4: 创建子表，父表分区数量为99，不违反规则",
			sql:           "CREATE TABLE test_partition_table_p100 PARTITION OF test_partition_table FOR VALUES FROM (99000) TO (100000);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟父表test_partition_table已有99个分区",
			mockSQL:       `SELECT c.relname FROM pg_class c JOIN pg_inherits i ON c.oid = i.inhparent JOIN pg_namespace n ON c.relnamespace = n.oid WHERE c.relname = $1 AND n.nspname = $2`,
			mockRows:      mockRowsFn(99),
		},
		{
			desc:          "case 5: 创建子表，父表分区数量为100，违反规则",
			sql:           "CREATE TABLE test_partition_table_p101 PARTITION OF test_partition_table FOR VALUES FROM (100000) TO (101000);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟父表test_partition_table已有100个分区",
			mockSQL:       `SELECT c.relname FROM pg_class c JOIN pg_inherits i ON c.oid = i.inhparent JOIN pg_namespace n ON c.relnamespace = n.oid WHERE c.relname = $1 AND n.nspname = $2`,
			mockRows:      mockRowsFn(100),
		},
		{
			desc:          "case 6: 创建子表，父表分区数量为101，违反规则",
			sql:           "CREATE TABLE test_partition_table_p102 PARTITION OF test_partition_table FOR VALUES FROM (101000) TO (102000);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟父表test_partition_table已有101个分区",
			mockSQL:       `SELECT c.relname FROM pg_class c JOIN pg_inherits i ON c.oid = i.inhparent JOIN pg_namespace n ON c.relnamespace = n.oid WHERE c.relname = $1 AND n.nspname = $2`,
			mockRows:      mockRowsFn(101),
		},
		{
			desc:          "case 7: 创建分区表时未指定分区数量",
			sql:           "CREATE TABLE test_non_partition_table (id bigint, data text);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 创建子表，但父表不存在",
			sql:           "CREATE TABLE test_partition_table_p103 PARTITION OF nonexistent_table FOR VALUES FROM (102000) TO (103000);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟不存在的父表",
			mockSQL:       `SELECT c.relname FROM pg_class c JOIN pg_inherits i ON c.oid = i.inhparent JOIN pg_namespace n ON c.relnamespace = n.oid WHERE c.relname = $1 AND n.nspname = $2`,
			mockRows:      mockRowsFn(0),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context

			if test.mockSQL != "" {
				d, ctx = setupForRuleSQLE00232WithMock(t, ruleName, _DefaultParams, test.mockSQL, test.mockRows)
			} else {
				d, ctx = setupForRuleGeneral(t, ruleName, _DefaultParams)
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

func setupForRuleSQLE00232WithMock(t *testing.T, ruleName string, params params.Params, mockSQL string, mockRows *mock.Rows) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 在 mock 中加入需要从数据库中查询的数据
	mock.ExpectQuery(mockSQL).WillReturnRows(mockRows)

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
