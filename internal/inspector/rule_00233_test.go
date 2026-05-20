package inspector

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00233(t *testing.T) {
	ruleName := SQLE00233
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockRows      interface{}
		mockSQL       string
	}

	tests := []testCase{
		// WITH statements
		{
			desc:          "case 1: Use RECURSIVE in a CTE with a sharded table",
			sql:           "WITH RECURSIVE cte AS (SELECT * FROM exist_tb_1) SELECT * FROM cte;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_1",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_1",
		},
		{
			desc:          "case 2: Use RECURSIVE in a CTE without a sharded table",
			sql:           "WITH RECURSIVE cte AS (SELECT * FROM exist_tb_2) SELECT * FROM cte;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "mock non-sharded table exist_tb_2",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_2",
		},
		{
			desc:          "case 9: Use RECURSIVE in a CTE with another sharded table",
			sql:           "WITH RECURSIVE cte AS (SELECT * FROM exist_tb_3) SELECT * FROM cte;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_3",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_3",
		},
		{
			desc:          "case 10: Use RECURSIVE in a CTE without a sharded table",
			sql:           "WITH RECURSIVE cte AS (SELECT * FROM exist_tb_9) SELECT * FROM cte;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "mock non-sharded table exist_tb_9",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_9",
		},

		// CREATE VIEW statements
		{
			desc:          "case 3: Create VIEW using RECURSIVE CTE with sharded table",
			sql:           "CREATE VIEW test_view AS WITH RECURSIVE cte AS (SELECT * FROM exist_tb_1) SELECT * FROM cte;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_1",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_1",
		},
		{
			desc:          "case 4: Create VIEW using RECURSIVE CTE without sharded table",
			sql:           "CREATE VIEW test_view AS WITH RECURSIVE cte AS (SELECT * FROM exist_tb_2) SELECT * FROM cte;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "mock non-sharded table exist_tb_2",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_2",
		},
		{
			desc:          "case 11: Create VIEW using RECURSIVE CTE with another sharded table",
			sql:           "CREATE VIEW test_view_2 AS WITH RECURSIVE cte AS (SELECT * FROM exist_tb_3) SELECT * FROM cte;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_3",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_3",
		},
		{
			desc:          "case 12: Create VIEW using RECURSIVE CTE without a sharded table",
			sql:           "CREATE VIEW test_view_2 AS WITH RECURSIVE cte AS (SELECT * FROM exist_tb_9) SELECT * FROM cte;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "mock non-sharded table exist_tb_9",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_9",
		},

		// CREATE FUNCTION statements
		{
			desc:          "case 5: Create FUNCTION using RECURSIVE CTE with sharded table",
			sql:           "CREATE FUNCTION test_func() RETURNS TABLE(id bigint) AS $$ BEGIN RETURN QUERY WITH RECURSIVE cte AS (SELECT * FROM exist_tb_1) SELECT * FROM cte; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_1",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_1",
		},
		{
			desc:          "case 5.1: Create FUNCTION using RECURSIVE CTE with sharded table, more space",
			sql:           "CREATE FUNCTION test_func() RETURNS TABLE(id bigint) AS $$ BEGIN RETURN QUERY WITH	RECURSIVE cte AS (SELECT * FROM exist_tb_1) SELECT * FROM cte; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_1",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_1",
		},
		{
			desc:          "case 5.2: Create FUNCTION using RECURSIVE CTE with sharded table, lower",
			sql:           "CREATE FUNCTION test_func() RETURNS TABLE(id bigint) AS $$ BEGIN RETURN QUERY with recursive cte AS (SELECT * FROM exist_tb_1) SELECT * FROM cte; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_1",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_1",
		},
		{
			desc:          "case 13: Create FUNCTION using RECURSIVE CTE with another sharded table",
			sql:           "CREATE FUNCTION test_func_2() RETURNS TABLE(id bigint) AS $$ BEGIN RETURN QUERY WITH RECURSIVE cte AS (SELECT * FROM exist_tb_3) SELECT * FROM cte; END; $$ LANGUAGE plpgsql;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_3",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_3",
		},

		// CREATE PROCEDURE statements
		{
			desc:          "case 7: Create PROCEDURE using RECURSIVE CTE with sharded table",
			sql:           "CREATE PROCEDURE test_proc() LANGUAGE plpgsql AS $$ BEGIN WITH RECURSIVE cte AS (SELECT * FROM exist_tb_1) SELECT * FROM cte; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_1",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_1",
		},
		{
			desc:          "case 15: Create PROCEDURE using RECURSIVE CTE with another sharded table",
			sql:           "CREATE PROCEDURE test_proc_2() LANGUAGE plpgsql AS $$ BEGIN WITH RECURSIVE cte AS (SELECT * FROM exist_tb_3) SELECT * FROM cte; END; $$;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "mock sharded table exist_tb_3",
			mockRows:      nil,
			mockSQL:       "SELECT * FROM exist_tb_3",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			log.Printf("======= %v ======\n", test.desc)
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
