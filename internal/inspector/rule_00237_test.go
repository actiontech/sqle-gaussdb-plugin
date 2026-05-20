package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00237(t *testing.T) {
	ruleName := SQLE00237
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 创建函数使用OUT参数",
			sql:           "CREATE FUNCTION test_func(a OUT int) RETURNS void AS $$ BEGIN a := 1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 创建函数使用IN OUT参数",
			sql:           "CREATE FUNCTION test_func(a IN OUT int) RETURNS int AS $$ BEGIN a := a + 1; RETURN a; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 创建函数不使用OUT或IN OUT参数",
			sql:           "CREATE FUNCTION test_func(a int) RETURNS int AS $$ BEGIN RETURN a + 1; END; $$ LANGUAGE plpgsql;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: 创建函数同时使用IN, OUT参数",
			sql:           "CREATE FUNCTION test_func(a IN int, b OUT int) RETURNS void AS $$ BEGIN b := a + 1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 创建函数使用多个OUT参数",
			sql:           "CREATE FUNCTION test_func(a OUT int, b OUT int) RETURNS void AS $$ BEGIN a := 1; b := 2; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: 创建函数使用OUT参数和返回类型",
			sql:           "CREATE FUNCTION test_func(a OUT int) RETURNS int AS $$ BEGIN a := 1; RETURN a; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 7: 创建函数不使用任何参数",
			sql:           "CREATE FUNCTION test_func() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 创建函数使用多个参数，无OUT或IN OUT",
			sql:           "CREATE FUNCTION test_func(a int, b int) RETURNS int AS $$ BEGIN RETURN a + b; END; $$ LANGUAGE plpgsql;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 9: 创建函数使用OUT参数在复杂表达式中",
			sql:           "CREATE FUNCTION test_func(a int, b OUT int) RETURNS void AS $$ BEGIN b := a * (SELECT max(id) FROM exist_tb_1); END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 10: 创建函数使用OUT参数与默认值",
			sql:           "CREATE FUNCTION test_func(a OUT int DEFAULT 5) RETURNS void AS $$ BEGIN a := a + 1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 创建函数使用多个OUT参数和IN OUT参数",
			sql:           "CREATE FUNCTION func_insert_t1(IN f_num int, OUT f_id int, OUT f_r1 int, OUT f_r2 int, IN OUT f_r3 int) AS $$ DECLARE i int := 0; BEGIN WHILE i < f_num LOOP INSERT INTO exist_tb_1 VALUES (i, floor(random()*100), floor(random()*100)); i := i + 1; END LOOP; SELECT * FROM exist_tb_1 INTO f_id, f_r1, f_r2, f_r3 WHERE id_shd_key = i - 1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 创建函数使用返回表结构代替OUT参数",
			sql:           "CREATE FUNCTION func_insert_t1_without_out_parameter(IN f_num int) RETURNS TABLE (v_id bigint, r_r1 character varying(255), r_r2 character varying(10485760)) AS $$ DECLARE i int := 0; BEGIN WHILE i < f_num LOOP INSERT INTO exist_tb_1 VALUES (i, floor(random()*100), floor(random()*100)); i := i + 1; END LOOP; RETURN QUERY SELECT * FROM exist_tb_1 WHERE id_shd_key = i - 1; END; $$ LANGUAGE plpgsql;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
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
