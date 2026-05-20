package inspector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRule6301 covers rule pg_63_01: unique/primary index columns must be NOT NULL.
// The test uses setupForRuleGeneral2 which provides exist_tb_2 with:
//   - id:      bigint, NOT NULL, primary key, unique index exist_tb_2_id_key
//   - v1:      varchar, NOT NULL, no index
//   - v2:      varchar, NULLABLE, no index
//   - user_id: bigint, NOT NULL, non-unique index exist_tb_2_user_id_key
func TestRule6301(t *testing.T) {
	ruleName := RuleId6301
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	tests := []testCase{
		// --- CREATE TABLE: column-level constraints ---
		{
			desc:          "create table column unique without not null should fail",
			sql:           "CREATE TABLE test_6301_1 (id INT, name VARCHAR(100) UNIQUE);",
			expectResults: []*testResults{newTestResults().add(ruleName, "name")},
		},
		{
			desc:          "create table column unique with not null should pass",
			sql:           "CREATE TABLE test_6301_2 (id INT, name VARCHAR(100) NOT NULL UNIQUE);",
			expectResults: _RuleAuditPassResults,
		},
		{
			desc:          "create table column-level primary key should be treated as not null",
			sql:           "CREATE TABLE test_6301_3 (id INT PRIMARY KEY);",
			expectResults: _RuleAuditPassResults,
		},
		// --- CREATE TABLE: table-level constraints ---
		{
			desc:          "create table table-level primary key should be treated as not null",
			sql:           "CREATE TABLE test_6301_4 (id INT, PRIMARY KEY (id));",
			expectResults: _RuleAuditPassResults,
		},
		{
			desc:          "create table table-level unique on nullable column should fail",
			sql:           "CREATE TABLE test_6301_5 (id INT, c1 INT, UNIQUE (c1));",
			expectResults: []*testResults{newTestResults().add(ruleName, "c1")},
		},
		{
			desc:          "create table table-level unique on not null column should pass",
			sql:           "CREATE TABLE test_6301_6 (id INT, c1 INT NOT NULL, UNIQUE (c1));",
			expectResults: _RuleAuditPassResults,
		},
		// --- ALTER TABLE: ADD CONSTRAINT UNIQUE/PRIMARY ---
		{
			desc:          "alter table add unique on nullable column should fail",
			sql:           "ALTER TABLE exist_tb_2 ADD CONSTRAINT uq_6301_1 UNIQUE (v2);",
			expectResults: []*testResults{newTestResults().add(ruleName, "v2")},
		},
		{
			desc:          "alter table add unique on not null column should pass",
			sql:           "ALTER TABLE exist_tb_2 ADD CONSTRAINT uq_6301_2 UNIQUE (v1);",
			expectResults: _RuleAuditPassResults,
		},
		{
			desc:          "alter table add primary key on nullable column should fail",
			sql:           "ALTER TABLE exist_tb_2 ADD PRIMARY KEY (v2);",
			expectResults: []*testResults{newTestResults().add(ruleName, "v2")},
		},
		{
			desc:          "alter table add primary key on not null column should pass",
			sql:           "ALTER TABLE exist_tb_2 ADD PRIMARY KEY (v1);",
			expectResults: _RuleAuditPassResults,
		},
		// --- ALTER TABLE: DROP NOT NULL ---
		{
			desc:          "alter table drop not null on unique-indexed column should fail",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN id DROP NOT NULL;",
			expectResults: []*testResults{newTestResults().add(ruleName, "id")},
		},
		{
			desc:          "alter table drop not null on non-indexed column should pass",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN v1 DROP NOT NULL;",
			expectResults: _RuleAuditPassResults,
		},
		{
			desc:          "alter table drop not null on normal-indexed column should pass for rule6301",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN user_id DROP NOT NULL;",
			expectResults: _RuleAuditPassResults,
		},
		// --- CREATE INDEX: must be unique to trigger rule6301 ---
		{
			desc:          "create unique index on nullable column should fail",
			sql:           "CREATE UNIQUE INDEX idx_6301_1 ON exist_tb_2(v2);",
			expectResults: []*testResults{newTestResults().add(ruleName, "v2")},
		},
		{
			desc:          "create unique index on not null column should pass",
			sql:           "CREATE UNIQUE INDEX idx_6301_2 ON exist_tb_2(v1);",
			expectResults: _RuleAuditPassResults,
		},
		{
			desc:          "create normal index should be ignored by rule6301",
			sql:           "CREATE INDEX idx_6301_3 ON exist_tb_2(v2);",
			expectResults: _RuleAuditPassResults,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleGeneral2(t, ruleName, _DefaultParams)
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

// TestRule6302 covers rule pg_63_02: normal index columns should be NOT NULL.
// The test uses setupForRuleGeneral2 (same mock as TestRule6301).
func TestRule6302(t *testing.T) {
	ruleName := RuleId6302
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}

	tests := []testCase{
		// --- CREATE INDEX: only non-unique indexes are in scope ---
		{
			desc:          "create normal index on nullable column should fail",
			sql:           "CREATE INDEX idx_6302_1 ON exist_tb_2(v2);",
			expectResults: []*testResults{newTestResults().add(ruleName, "v2")},
		},
		{
			desc:          "create normal index on not null column should pass",
			sql:           "CREATE INDEX idx_6302_2 ON exist_tb_2(v1);",
			expectResults: _RuleAuditPassResults,
		},
		{
			desc:          "create unique index should be ignored by rule6302",
			sql:           "CREATE UNIQUE INDEX idx_6302_3 ON exist_tb_2(v2);",
			expectResults: _RuleAuditPassResults,
		},
		// --- CREATE TABLE: no ordinary indexes possible in PG CREATE TABLE → always pass ---
		{
			desc:          "create table unique constraint should be ignored by rule6302",
			sql:           "CREATE TABLE test_6302_1 (id INT, name VARCHAR(100) UNIQUE);",
			expectResults: _RuleAuditPassResults,
		},
		// --- ALTER TABLE: ADD UNIQUE/PRIMARY constraint is ignored by rule6302 ---
		{
			desc:          "alter table add unique should be ignored by rule6302",
			sql:           "ALTER TABLE exist_tb_2 ADD CONSTRAINT uq_6302_1 UNIQUE (v2);",
			expectResults: _RuleAuditPassResults,
		},
		// --- ALTER TABLE: DROP NOT NULL on normal-indexed column should fail ---
		{
			desc:          "alter table drop not null on normal-indexed column should fail",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN user_id DROP NOT NULL;",
			expectResults: []*testResults{newTestResults().add(ruleName, "user_id")},
		},
		{
			desc:          "alter table drop not null on unique-indexed column should pass for rule6302",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN id DROP NOT NULL;",
			expectResults: _RuleAuditPassResults,
		},
		{
			desc:          "alter table drop not null on non-indexed column should pass",
			sql:           "ALTER TABLE exist_tb_2 ALTER COLUMN v1 DROP NOT NULL;",
			expectResults: _RuleAuditPassResults,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleGeneral2(t, ruleName, _DefaultParams)
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

// TestRule63MixedDetection verifies that rule6301 and rule6302 are mutually exclusive:
// a unique index only triggers 6301, a normal index only triggers 6302.
func TestRule63MixedDetection(t *testing.T) {
	type testCase struct {
		desc             string
		uniqueSQL        string
		normalSQL        string
		expectUnique6301 *testResults
		expectUnique6302 *testResults
		expectNormal6301 *testResults
		expectNormal6302 *testResults
	}

	tests := []testCase{
		{
			desc:             "both rules should be triggered",
			uniqueSQL:        "CREATE UNIQUE INDEX uk_63_mix_1 ON exist_tb_2(v2);",
			normalSQL:        "CREATE INDEX idx_63_mix_1 ON exist_tb_2(v2);",
			expectUnique6301: newTestResults().add(RuleId6301, "v2"),
			expectUnique6302: newTestResults(),
			expectNormal6301: newTestResults(),
			expectNormal6302: newTestResults().add(RuleId6302, "v2"),
		},
		{
			desc:             "both rules should not be triggered",
			uniqueSQL:        "CREATE UNIQUE INDEX uk_63_mix_2 ON exist_tb_2(v1);",
			normalSQL:        "CREATE INDEX idx_63_mix_2 ON exist_tb_2(v1);",
			expectUnique6301: newTestResults(),
			expectUnique6302: newTestResults(),
			expectNormal6301: newTestResults(),
			expectNormal6302: newTestResults(),
		},
		{
			desc:             "only rule6301 should be triggered",
			uniqueSQL:        "CREATE UNIQUE INDEX uk_63_mix_3 ON exist_tb_2(v2);",
			normalSQL:        "CREATE INDEX idx_63_mix_3 ON exist_tb_2(v1);",
			expectUnique6301: newTestResults().add(RuleId6301, "v2"),
			expectUnique6302: newTestResults(),
			expectNormal6301: newTestResults(),
			expectNormal6302: newTestResults(),
		},
		{
			desc:             "only rule6302 should be triggered",
			uniqueSQL:        "CREATE UNIQUE INDEX uk_63_mix_4 ON exist_tb_2(v1);",
			normalSQL:        "CREATE INDEX idx_63_mix_4 ON exist_tb_2(v2);",
			expectUnique6301: newTestResults(),
			expectUnique6302: newTestResults(),
			expectNormal6301: newTestResults(),
			expectNormal6302: newTestResults().add(RuleId6302, "v2"),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d6301, ctx6301 := setupForRuleGeneral2(t, RuleId6301, _DefaultParams)
			uniqueResult6301, err := d6301.Audit(ctx6301, []string{test.uniqueSQL})
			assert.NoError(t, err, test.desc+" [rule6301 unique]")
			assertRuleResult(t, test.desc+" [rule6301 unique]", test.uniqueSQL, []*testResults{test.expectUnique6301}, uniqueResult6301)

			d6301, ctx6301 = setupForRuleGeneral2(t, RuleId6301, _DefaultParams)
			normalResult6301, err := d6301.Audit(ctx6301, []string{test.normalSQL})
			assert.NoError(t, err, test.desc+" [rule6301 normal]")
			assertRuleResult(t, test.desc+" [rule6301 normal]", test.normalSQL, []*testResults{test.expectNormal6301}, normalResult6301)

			d6302, ctx6302 := setupForRuleGeneral2(t, RuleId6302, _DefaultParams)
			uniqueResult6302, err := d6302.Audit(ctx6302, []string{test.uniqueSQL})
			assert.NoError(t, err, test.desc+" [rule6302 unique]")
			assertRuleResult(t, test.desc+" [rule6302 unique]", test.uniqueSQL, []*testResults{test.expectUnique6302}, uniqueResult6302)

			d6302, ctx6302 = setupForRuleGeneral2(t, RuleId6302, _DefaultParams)
			normalResult6302, err := d6302.Audit(ctx6302, []string{test.normalSQL})
			assert.NoError(t, err, test.desc+" [rule6302 normal]")
			assertRuleResult(t, test.desc+" [rule6302 normal]", test.normalSQL, []*testResults{test.expectNormal6302}, normalResult6302)
		})
	}
}

// TestRule63FullFlowNullableBackToNull keeps the "phase-by-phase" verification style,
// but runs each SQL independently with a fresh mocked context to avoid shared mock query exhaustion.
func TestRule63FullFlowNullableBackToNull(t *testing.T) {
	type phaseCase struct {
		sql    string
		expect *testResults
	}

	t.Run("rule6301 full flow", func(t *testing.T) {
		cases := []phaseCase{
			{sql: "CREATE UNIQUE INDEX uk_63_ff_1 ON exist_tb_2(v2);", expect: newTestResults().add(RuleId6301, "v2")},
			{sql: "CREATE INDEX idx_63_ff_1 ON exist_tb_2(v2);", expect: newTestResults()},
			{sql: "UPDATE exist_tb_2 SET v2 = CONCAT('fallback_', id::text) WHERE v2 IS NULL;", expect: newTestResults()},
		}
		for idx, c := range cases {
			d, ctx := setupForRuleGeneral2(t, RuleId6301, _DefaultParams)
			result, err := d.Audit(ctx, []string{c.sql})
			assert.NoError(t, err)
			assertRuleResult(t, fmt.Sprintf("rule6301 full flow phase %d", idx+1), c.sql, []*testResults{c.expect}, result)
		}
	})

	t.Run("rule6302 full flow", func(t *testing.T) {
		cases := []phaseCase{
			{sql: "CREATE UNIQUE INDEX uk_63_ff_1 ON exist_tb_2(v2);", expect: newTestResults()},
			{sql: "CREATE INDEX idx_63_ff_1 ON exist_tb_2(v2);", expect: newTestResults().add(RuleId6302, "v2")},
			{sql: "UPDATE exist_tb_2 SET v2 = CONCAT('fallback_', id::text) WHERE v2 IS NULL;", expect: newTestResults()},
		}
		for idx, c := range cases {
			d, ctx := setupForRuleGeneral2(t, RuleId6302, _DefaultParams)
			result, err := d.Audit(ctx, []string{c.sql})
			assert.NoError(t, err)
			assertRuleResult(t, fmt.Sprintf("rule6302 full flow phase %d", idx+1), c.sql, []*testResults{c.expect}, result)
		}
	})
}
