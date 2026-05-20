package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00030(t *testing.T) {
	ruleName := "SQLE00030"
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	tests := []testCase{
		{
			desc:          "case 1: 创建基本触发器",
			sql:           "CREATE TRIGGER test_trigger AFTER INSERT ON exist_tb_1 FOR EACH ROW EXECUTE FUNCTION test_func();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 2: 创建带条件的触发器",
			sql:           "CREATE TRIGGER test_trigger BEFORE UPDATE ON exist_tb_1 FOR EACH ROW EXECUTE FUNCTION update_notify();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 3: 创建多个动作的触发器",
			sql:           "CREATE TRIGGER test_trigger AFTER INSERT OR DELETE OR UPDATE ON exist_tb_1 FOR EACH ROW EXECUTE FUNCTION log_activity();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 4: 在不存在的表上尝试创建触发器",
			sql:           "CREATE TRIGGER error_trigger AFTER INSERT ON non_exist_tb FOR EACH ROW EXECUTE FUNCTION do_nothing();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 5: 创建涉及多个事件的触发器",
			sql:           "CREATE TRIGGER complex_trigger BEFORE INSERT OR UPDATE ON exist_tb_2 FOR EACH ROW EXECUTE FUNCTION compute_values();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 6: 创建INSTEAD OF触发器，用于视图",
			sql:           "CREATE TRIGGER view_trigger INSTEAD OF UPDATE ON exist_view_1 FOR EACH ROW EXECUTE FUNCTION refresh_view();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 7: 在触发器内部使用事务控制语句",
			sql:           "CREATE TRIGGER trans_trigger AFTER INSERT ON exist_tb_2 FOR EACH ROW EXECUTE PROCEDURE transactional_activity();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 8: 创建触发器，动作是调用现有数据库函数",
			sql:           "CREATE TRIGGER func_trigger AFTER UPDATE ON exist_tb_3 FOR EACH ROW WHEN (NEW.c1 <> OLD.c1) EXECUTE FUNCTION update_log();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 9: 尝试在分布式列上创建触发器",
			sql:           "CREATE TRIGGER dist_col_trigger AFTER UPDATE OF c3_shd_key ON exist_tb_3 FOR EACH ROW EXECUTE FUNCTION log_changes();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 10: 创建触发器，触发条件为与业务规则相关的条件",
			sql:           "CREATE TRIGGER business_rule_trigger AFTER INSERT ON exist_tb_9 FOR EACH ROW WHEN (NEW.c1_with_idx > 100) EXECUTE FUNCTION notify_large_insert();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 11: 创建触发器，使用自定义函数和复杂的条件表达式",
			sql:           "CREATE TRIGGER advanced_trigger AFTER UPDATE ON exist_tb_9 FOR EACH ROW WHEN (NEW.c2_with_idx IS NOT NULL AND NEW.c2_with_idx <> OLD.c2_with_idx) EXECUTE FUNCTION update_cache();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 12: 创建触发器，触发条件为简单的条件表达式",
			sql:           "CREATE TRIGGER simple_trigger AFTER INSERT ON exist_tb_3 FOR EACH ROW WHEN (NEW.c1 IS NOT NULL) EXECUTE FUNCTION simple_log();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 13: 创建触发器，动作是调用存储过程",
			sql:           "CREATE TRIGGER proc_trigger AFTER DELETE ON exist_tb_2 FOR EACH ROW EXECUTE PROCEDURE delete_log();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 14: 创建触发器，触发条件为复杂的条件表达式",
			sql:           "CREATE TRIGGER complex_cond_trigger BEFORE UPDATE ON exist_tb_9 FOR EACH ROW WHEN (NEW.c1_with_idx IS NOT NULL AND NEW.c1_with_idx > 50) EXECUTE FUNCTION complex_log();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 15: 创建触发器，动作是调用外部函数",
			sql:           "CREATE TRIGGER external_func_trigger AFTER INSERT ON exist_tb_1 FOR EACH ROW EXECUTE FUNCTION external_log();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 16: 创建触发器，触发条件为检查唯一约束",
			sql:           "CREATE TRIGGER unique_check_trigger BEFORE INSERT ON exist_tb_9 FOR EACH ROW WHEN (NEW.c2_with_idx IS DISTINCT FROM OLD.c2_with_idx) EXECUTE FUNCTION unique_check();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "case 17: 创建触发器，触发条件为检查分布式列",
			sql:           "CREATE TRIGGER dist_col_check_trigger AFTER INSERT ON exist_tb_3 FOR EACH ROW WHEN (NEW.c3_shd_key IS NOT NULL) EXECUTE FUNCTION dist_col_log();",
			expectResults: []*testResults{newTestResults().add(ruleName)},
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
