// ==== Rule test code start ====
package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleSQLE00029(t *testing.T) {
	ruleName := "SQLE00029"
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
			desc:          "case 1: 创建基本存储过程，违反规则",
			sql:           "CREATE PROCEDURE test_procedure() LANGUAGE sql AS $$ BEGIN SELECT * FROM exist_tb_1; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 2: 创建带参数的存储过程，违反规则",
			sql:           "CREATE PROCEDURE test_procedure(param1 INTEGER) LANGUAGE plpgsql AS $$ BEGIN SELECT param1; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 3: 创建带多个操作的存储过程，违反规则",
			sql:           "CREATE PROCEDURE complex_procedure() LANGUAGE plpgsql AS $$ BEGIN INSERT INTO exist_tb_2 VALUES (1, 'data', 'more', 100); SELECT * FROM exist_tb_1; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 4: 创建嵌套存储过程，违反规则",
			sql:           "CREATE PROCEDURE nested_procedure() LANGUAGE plpgsql AS $$ BEGIN CALL another_procedure(); END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 5: 创建使用动态SQL的存储过程，违反规则",
			sql:           "CREATE PROCEDURE dynamic_sql_procedure() LANGUAGE plpgsql AS $$ BEGIN EXECUTE 'SELECT * FROM exist_tb_3'; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 6: 创建简单存储过程，违反规则",
			sql:           "CREATE PROCEDURE simple_procedure() LANGUAGE sql AS $$ BEGIN SELECT * FROM exist_tb_9; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 7: 创建带条件判断的存储过程，违反规则",
			sql:           "CREATE PROCEDURE conditional_procedure() LANGUAGE plpgsql AS $$ BEGIN IF EXISTS (SELECT 1 FROM exist_tb_1) THEN UPDATE exist_tb_1 SET c1_with_idx = 'updated'; END IF; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 8: 创建带循环的存储过程，违反规则",
			sql:           "CREATE PROCEDURE loop_procedure() LANGUAGE plpgsql AS $$ DECLARE i INTEGER := 0; BEGIN WHILE i < 10 LOOP INSERT INTO exist_tb_2 VALUES (i, 'data' || i, 'more', 100 + i); i := i + 1; END LOOP; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 9: 创建带异常处理的存储过程，违反规则",
			sql:           "CREATE PROCEDURE exception_procedure() LANGUAGE plpgsql AS $$ BEGIN BEGIN INSERT INTO exist_tb_2 VALUES (1, 'data', 'more', 100); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'Error inserting data'; END; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
		{
			desc:          "case 10: 创建带事务控制的存储过程，违反规则",
			sql:           "CREATE PROCEDURE transaction_procedure() LANGUAGE plpgsql AS $$ BEGIN START TRANSACTION; INSERT INTO exist_tb_2 VALUES (1, 'data', 'more', 100); COMMIT; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
			mockRows:      "",
			mockSQL:       "",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleGeneral(t, ruleName, _DefaultParams)
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
