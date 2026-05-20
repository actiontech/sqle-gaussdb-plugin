package inspector

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00020(t *testing.T) {
	ruleName := "SQLE00020"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockRows      *sqlmock.Rows
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "用例 1: 创建表超过列数限制",
			sql:           "CREATE TABLE test_tb (id bigint, c1 integer, c2 integer, c3 integer, c4 integer, c5 integer, c6 integer);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: 创建表未超过列数限制",
			sql:           "CREATE TABLE test_tb (id bigint, c1 integer, c2 integer);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 3: 添加列后超过列数限制",
			sql:           "ALTER TABLE exist_tb_9 ADD COLUMN c6 integer;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: 添加列后未超过列数限制",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN c4 integer;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 6: 删除列后未超过列数限制",
			sql:           "ALTER TABLE exist_tb_1 DROP COLUMN c1_with_idx;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 7: 同时添加和删除列导致超限",
			sql:           "ALTER TABLE exist_tb_9 ADD COLUMN c6 integer, ADD COLUMN c7 integer, DROP COLUMN c1_with_idx;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: 同时添加和删除列未超限",
			sql:           "ALTER TABLE exist_tb_3 ADD COLUMN c4 integer, DROP COLUMN c1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
		},
		{
			desc:          "用例 10: 添加多列后超过列数限制",
			sql:           "ALTER TABLE exist_tb_9 ADD COLUMN c6 integer, ADD COLUMN c7 integer;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var d *driverImpl
			var ctx context.Context
			d, ctx = setupForRuleGeneral(t, ruleName, params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "5",
				Desc:  "表内列数上限",
				Type:  params.ParamTypeString,
			},
			})
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
