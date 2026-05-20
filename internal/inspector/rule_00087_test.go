package inspector

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00087(t *testing.T) {
	ruleName := SQLE00087
	inClauseThreshold := 500

	tests := []struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
	}{
		{
			desc:          "用例 1: SELECT 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("SELECT * FROM exist_db.exist_tb_1 WHERE id IN (%s);", generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 2: SELECT 语句，IN 子句元素数量未超过阈值，不违反规则",
			sql:           fmt.Sprintf("SELECT * FROM exist_db.exist_tb_1 WHERE id IN (%s);", generateInClause(inClauseThreshold-1)),
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
		},
		{
			desc:          "用例 3: WITH 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("WITH cte AS (SELECT * FROM exist_db.exist_tb_1 WHERE id IN (%s)) SELECT * FROM cte;", generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 4: INSERT ... SELECT 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("INSERT INTO exist_db.exist_tb_1 (id_shd_key, c1_with_idx) SELECT id, c1 FROM exist_db.exist_tb_2 WHERE id IN (%s);", generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 5: UPDATE 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("UPDATE exist_db.exist_tb_1 SET c1_with_idx = 'updated' WHERE id_shd_key IN (%s);", generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 6: DELETE 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("DELETE FROM exist_db.exist_tb_1 WHERE id_shd_key IN (%s);", generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 7: UNION 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("SELECT id FROM exist_db.exist_tb_1 WHERE id_shd_key IN (%s) UNION SELECT id FROM exist_db.exist_tb_2 WHERE id IN (%s);", generateInClause(inClauseThreshold+1), generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 8: UNION ALL 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("SELECT id FROM exist_db.exist_tb_1 WHERE id IN (%s) UNION ALL SELECT id FROM exist_db.exist_tb_2 WHERE id IN (%s);", generateInClause(inClauseThreshold+1), generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 9: INTERSECT 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("SELECT id FROM exist_db.exist_tb_1 WHERE id IN (%s) INTERSECT SELECT id FROM exist_db.exist_tb_2 WHERE id IN (%s);", generateInClause(inClauseThreshold+1), generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 10: EXCEPT 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("SELECT id FROM exist_db.exist_tb_1 WHERE id IN (%s) EXCEPT SELECT id FROM exist_db.exist_tb_2 WHERE id IN (%s);", generateInClause(inClauseThreshold+1), generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 11: 带别名的 SELECT 语句，IN 子句元素数量超过阈值，违反规则",
			sql:           fmt.Sprintf("SELECT e1.id AS alias_id FROM exist_db.exist_tb_1 AS e1 WHERE e1.id IN (%s);", generateInClause(inClauseThreshold+1)),
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
		},
		{
			desc:          "用例 12: 带别名的 SELECT 语句，IN 子句元素数量未超过阈值，不违反规则",
			sql:           fmt.Sprintf("SELECT e1.id AS alias_id FROM exist_db.exist_tb_1 AS e1 WHERE e1.id IN (%s);", generateInClause(inClauseThreshold-1)),
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

func generateInClause(num int) string {
	var values []string
	for i := 1; i <= num; i++ {
		values = append(values, fmt.Sprintf("%d", i))
	}
	return strings.Join(values, ", ")
}

// ==== Rule test code end ====
