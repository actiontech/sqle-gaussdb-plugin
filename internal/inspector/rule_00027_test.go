package inspector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====

func TestSQLRuleEnforcement(t *testing.T) {
	ruleName := "SQLE00027" // 假设规则名

	tests := []struct {
		desc          string
		sql           []string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc: "创建表时添加列注释",
			sql: []string{
				`CREATE TABLE t1(
					id integer primary key,
					name varchar(64),
      				type varchar(4)
				);
				`,
				`COMMENT ON COLUMN t1.id IS 'id';`,
				`COMMENT ON COLUMN t1.name IS 'name';`,
				`COMMENT ON COLUMN t1.type IS 'type';`,
			},
			expectResults: []*testResults{newTestResults(), newTestResults(), newTestResults(), newTestResults()},
			expectError:   false,
		},
		{
			desc: "创建表时未添加列注释",
			sql: []string{
				`CREATE TABLE t1(
					id integer primary key,
					name varchar(64),
      				type varchar(4)
				);`,
				`COMMENT ON COLUMN t1.id IS 'id';`,
				`COMMENT ON COLUMN t1.name IS 'name';`,
			},
			expectResults: []*testResults{newTestResults().add(ruleName, "type"), newTestResults(), newTestResults()},
			expectError:   false,
		},
		{
			desc: "创建表时未添加两个列的注释",
			sql: []string{
				`CREATE TABLE t1(
					id integer primary key,
					name varchar(64),
      				type varchar(4)
				);`,
				`COMMENT ON COLUMN t1.id IS 'id';`,
			},
			expectResults: []*testResults{newTestResults().add(ruleName, strings.Join([]string{"name", "type"}, ",")), newTestResults()},
			expectError:   false,
		},
		{
			desc: "增加列时添加列注释",
			sql: []string{
				`ALTER TABLE exist_tb_1 ADD COLUMN v4 int;`,
				`COMMENT ON COLUMN exist_tb_1.v4 IS 'test';`,
			},
			expectResults: []*testResults{newTestResults(), newTestResults()},
			expectError:   false,
		},
		{
			desc: "增加列时未添加列注释",
			sql: []string{
				`ALTER TABLE exist_tb_1 ADD COLUMN v4 int;`,
			},
			expectResults: []*testResults{newTestResults().add(ruleName, "v4")},
			expectError:   false,
		},
	}

	// 执行测试
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			d, ctx := setupForRuleGeneral(t, ruleName, _DefaultParams)
			results, err := d.Audit(ctx, tc.sql)
			if tc.expectError {
				assert.Error(t, err, tc.desc)
			} else {
				assert.NoError(t, err, tc.desc)
				assertRuleResult(t, tc.desc, strings.Join(tc.sql, "\n"), tc.expectResults, results)
			}
		})
	}
}

// ==== Rule test code end ====
