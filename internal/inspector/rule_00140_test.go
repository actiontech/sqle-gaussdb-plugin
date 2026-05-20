package inspector

import (
	"context"
	"strings"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====

func TestRuleSQLE00140(t *testing.T) {
	ruleName := "SQLE00140"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockRows      string
		mockSQL       string
	}

	tests := []testCase{
		{
			desc:          "case 1: 创建表未指定 schema",
			sql:           "CREATE TABLE new_table (id INT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 创建表指定 schema",
			sql:           "CREATE TABLE exist_db.new_table (id INT);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 创建视图未指定 schema",
			sql:           "CREATE VIEW new_view AS SELECT * FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: 创建视图指定 schema",
			sql:           "CREATE VIEW exist_db.new_view AS SELECT * FROM exist_db.exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 修改表结构未指定 schema",
			sql:           "ALTER TABLE exist_tb_1 ADD COLUMN new_col INT;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: 修改表结构指定 schema",
			sql:           "ALTER TABLE exist_db.exist_tb_1 ADD COLUMN v1 INT;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 7: 插入数据未指定 schema",
			sql:           "INSERT INTO exist_tb_1 (c1_with_idx) VALUES ('data');",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 插入数据指定 schema",
			sql:           "INSERT INTO exist_db.exist_tb_1 (c1_with_idx) VALUES ('data');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 9: 选择未指定 schema",
			sql:           "SELECT * FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 10: 选择指定 schema",
			sql:           "SELECT * FROM exist_db.exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 更新数据未指定 schema",
			sql:           "UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE c1_with_idx = 't';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 更新数据指定 schema",
			sql:           "UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' WHERE c1_with_idx = 't';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: 删除数据未指定 schema",
			sql:           "DELETE FROM exist_tb_1 WHERE c1_with_idx = 't';",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: 删除数据指定 schema",
			sql:           "DELETE FROM exist_db.exist_tb_1 WHERE c1_with_idx = 't';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 15: 调用存储过程未指定 schema",
			sql:           "CALL update_data();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 16: 调用存储过程指定 schema",
			sql:           "CALL exist_db.update_data();",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 17: 删除表未指定 schema",
			sql:           "DROP TABLE exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 18: 删除表指定 schema",
			sql:           "DROP TABLE exist_db.exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 19: 创建函数未指定 schema",
			sql:           "CREATE FUNCTION my_function() RETURNS INT AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 20: 创建函数指定 schema",
			sql:           "CREATE FUNCTION exist_db.my_function() RETURNS INT AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 21: 删除函数未指定 schema",
			sql:           "DROP FUNCTION my_function();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 22: 删除函数指定 schema",
			sql:           "DROP FUNCTION exist_db.my_function();",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 23: 创建序列未指定 schema",
			sql:           "CREATE SEQUENCE my_seq;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 24: 创建序列指定 schema",
			sql:           "CREATE SEQUENCE exist_db.my_seq;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 25: 删除序列未指定 schema",
			sql:           "DROP SEQUENCE my_seq;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 26: 删除序列指定 schema",
			sql:           "DROP SEQUENCE exist_db.my_seq;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 27: 创建类型未指定 schema",
			sql:           "CREATE TYPE my_type AS (id INT, name TEXT);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 28: 创建类型指定 schema",
			sql:           "CREATE TYPE exist_db.my_type AS (id INT, name TEXT);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 29: 删除类型未指定 schema",
			sql:           "DROP TYPE my_type;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 30: 删除类型指定 schema",
			sql:           "DROP TYPE exist_db.my_type;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 31: 创建物化视图未指定 schema",
			sql:           "CREATE MATERIALIZED VIEW mv1 AS SELECT * FROM exist_tb_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 32: 创建物化视图指定 schema",
			sql:           "CREATE MATERIALIZED VIEW exist_db.mv1 AS SELECT * FROM exist_db.exist_tb_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 33: 刷新物化视图未指定 schema",
			sql:           "REFRESH MATERIALIZED VIEW mv1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 34: 刷新物化视图指定 schema",
			sql:           "REFRESH MATERIALIZED VIEW exist_db.mv1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 35: 删除物化视图未指定 schema",
			sql:           "DROP MATERIALIZED VIEW mv1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 36: 删除物化视图指定 schema",
			sql:           "DROP MATERIALIZED VIEW exist_db.mv1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 37:创建 index未指定schema",
			sql:           "CREATE INDEX idx_c1 ON exist_tb_1 (c1_with_idx_1);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 38:重启序列未指定schema",
			sql:           "ALTER SEQUENCE new_seq RESTART;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 39:重启序列指定schema",
			sql:           "ALTER SEQUENCE exit_db.new_seq RESTART;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 40:drop index未指定schema",
			sql:           "DROP INDEX exist_tb_1_uniq_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 40:drop index指定schema",
			sql:           "DROP INDEX exist_db.exist_tb_1_uniq_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 41:创建 index指定schema",
			sql:           "CREATE INDEX idx_c1 ON exist_db.exist_tb_1 (c1_with_idx_1);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 42:创建物化视图子查询未指定schema",
			sql:           "CREATE MATERIALIZED VIEW exist_db.new_mv AS SELECT * FROM exist_view_1;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 43:创建物化视图子查询指定schema",
			sql:           "CREATE MATERIALIZED VIEW exist_db.new_mv AS SELECT * FROM exist_db.exist_view_1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context
			if test.mockSQL == "" {
				d, ctx = setupForRuleGeneral(t, ruleName, _DefaultParams)
			} else {
				d, ctx = setupForRuleWithMockFor140(t, ruleName, _DefaultParams, test.mockDesc, test.mockSQL, test.mockRows)
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

func setupForRuleWithMockFor140(t *testing.T, ruleName string, params params.Params, mockDesc, mockSQL, mockRows string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// 因为用例需要，在 mock 中加入需要从数据库中查询的数据
	mock.ExpectQuery(mockSQL).
		WillReturnRows(mock.NewRows(strings.Split(mockRows, ",")))

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
