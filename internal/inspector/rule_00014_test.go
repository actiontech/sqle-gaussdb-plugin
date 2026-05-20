package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00014(t *testing.T) {
	ruleName := "SQLE00014"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "case 1: 创建简单函数，违反规则",
			sql:           "CREATE FUNCTION simple_function() RETURNS integer AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 2: 创建带参数函数，违反规则",
			sql:           "CREATE FUNCTION add_numbers(a integer, b integer) RETURNS integer AS $$ BEGIN RETURN a + b; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 3: 创建嵌套函数，违反规则",
			sql:           "CREATE FUNCTION nested_function() RETURNS integer AS $$ BEGIN PERFORM add_numbers(1, 2); RETURN 3; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 4: 创建使用表数据的函数，违反规则",
			sql:           "CREATE FUNCTION count_rows() RETURNS bigint AS $$ BEGIN RETURN (SELECT COUNT(*) FROM exist_tb_1); END; $$ LANGUAGE sql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 5: 创建修改数据的函数，违反规则",
			sql:           "CREATE FUNCTION update_data() RETURNS void AS $$ BEGIN UPDATE exist_tb_1 SET c1_with_idx = 'updated' WHERE id_shd_key = 1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 6: 创建插入数据的函数，违反规则",
			sql:           "CREATE FUNCTION func_insert_customers() RETURNS void AS $$ DECLARE i int := 0; BEGIN WHILE i < 10000 LOOP INSERT INTO exist_tb_1 (id_shd_key, c1_with_idx) VALUES (i, '小季'||i::text); i := i + 1; END LOOP; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 7: 创建带有复杂逻辑的函数，违反规则",
			sql:           "CREATE FUNCTION complex_logic() RETURNS void AS $$ BEGIN IF EXISTS (SELECT 1 FROM exist_tb_2 WHERE user_id = 1) THEN UPDATE exist_tb_2 SET c1 = 'updated' WHERE user_id = 1; ELSE INSERT INTO exist_tb_2 (id, c1, user_id) VALUES (nextval('exist_db.exist_tb_2_id_seq'), 'new', 1); END IF; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 8: 创建删除数据的函数，违反规则",
			sql:           "CREATE FUNCTION delete_data() RETURNS void AS $$ BEGIN DELETE FROM exist_tb_3 WHERE c3_shd_key IS NULL; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 9: 创建带有索引操作的函数，违反规则",
			sql:           "CREATE FUNCTION index_operation() RETURNS void AS $$ BEGIN CREATE INDEX idx_temp ON exist_tb_9 (c1_with_idx); END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 10: 创建带有视图操作的函数，违反规则",
			sql:           "CREATE FUNCTION view_operation() RETURNS void AS $$ BEGIN REFRESH MATERIALIZED VIEW exist_view_1; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 11: 创建存储过程,未违反规则",
			sql:           "CREATE PROCEDURE test_procedure() LANGUAGE sql AS $$ BEGIN SELECT * FROM exist_tb_1; END; $$;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
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
