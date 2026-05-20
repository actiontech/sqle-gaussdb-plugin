package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00240(t *testing.T) {
	ruleName := SQLE00240 // SQL 审核规范
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
			desc:          "case 2: Create table without partitions keyword",
			sql:           "CREATE TABLE new_table_2 (id bigint, name text)",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: Create table with keyword 'partition' in column comment",
			sql:           "CREATE TABLE new_table_3 (id bigint, name text, note text DEFAULT 'partition note')",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: Create table using data from existing table structure",
			sql:           "CREATE TABLE new_table_4 AS TABLE exist_tb_1 WITH NO DATA",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: Create table with foreign key reference",
			sql:           "CREATE TABLE new_table_5 (id bigint PRIMARY KEY, ref_id bigint REFERENCES exist_tb_2(id))",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: Create table with partition by range and partitions",
			sql:           "CREATE TABLE customers (id int, name text, sex int, city text, age int, log_date timestamp, mark text, primary key (id)) PARTITION BY RANGE (log_date) BEGIN (timestamp without time zone '2024-01-01') STEP (interval '1 month') PARTITIONS (12) DISTRIBUTE BY SHARD(id);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 7: Create table with partition by range without partitions",
			sql:           "CREATE TABLE customers2 (id int, name text, sex int, city text, age int, log_date timestamp, mark text) PARTITION BY RANGE (log_date) DISTRIBUTE BY SHARD(id);",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_0 PARTITION OF customers2 FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 9: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_1 PARTITION OF customers2 FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 10: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_2 PARTITION OF customers2 FOR VALUES FROM ('2024-03-01') TO ('2024-04-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_3 PARTITION OF customers2 FOR VALUES FROM ('2024-04-01') TO ('2024-05-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_4 PARTITION OF customers2 FOR VALUES FROM ('2024-05-01') TO ('2024-06-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_5 PARTITION OF customers2 FOR VALUES FROM ('2024-06-01') TO ('2024-07-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_6 PARTITION OF customers2 FOR VALUES FROM ('2024-07-01') TO ('2024-08-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 15: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_7 PARTITION OF customers2 FOR VALUES FROM ('2024-08-01') TO ('2024-09-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 16: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_8 PARTITION OF customers2 FOR VALUES FROM ('2024-09-01') TO ('2024-10-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 17: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_9 PARTITION OF customers2 FOR VALUES FROM ('2024-10-01') TO ('2024-11-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 18: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_10 PARTITION OF customers2 FOR VALUES FROM ('2024-11-01') TO ('2024-12-01');",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 19: Create partition of customers2",
			sql:           "CREATE TABLE customers2_part_11 PARTITION OF customers2 FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');",
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
