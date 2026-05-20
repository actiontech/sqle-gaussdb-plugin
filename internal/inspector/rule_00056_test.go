package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00056(t *testing.T) {
	ruleName := SQLE00056
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
			desc:          "case 1: 创建数据库时指定UTF8MB4字符集",
			sql:           "CREATE DATABASE test_db WITH ENCODING = 'UTF8MB4';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 2: 创建数据库时未指定字符集",
			sql:           "CREATE DATABASE test_db;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 3: 创建数据库时指定非UTF8MB4字符集",
			sql:           "CREATE DATABASE test_db WITH ENCODING = 'LATIN1';",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "UTF8MB4")},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 4: 创建数据库时指定UTF8MB4字符集，但有额外选项",
			sql:           "CREATE DATABASE test_db WITH ENCODING = 'UTF8MB4' LC_COLLATE = 'en_US.utf8' LC_CTYPE = 'en_US.utf8';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 5: 创建数据库时使用小写encoding关键字",
			sql:           "CREATE DATABASE test_db with encoding = 'UTF8MB4';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 6: 创建数据库时指定UTF8MB4字符集，使用引号",
			sql:           "CREATE DATABASE test_db WITH ENCODING = \"UTF8MB4\";",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 7: 创建数据库时指定UTF8MB4字符集，带有模板",
			sql:           "CREATE DATABASE test_db WITH TEMPLATE template0 ENCODING = 'UTF8MB4';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 8: 创建数据库时指定非UTF8MB4字符集，带有模板",
			sql:           "CREATE DATABASE test_db WITH TEMPLATE template0 ENCODING = 'SQL_ASCII';",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "UTF8MB4")},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 9: 创建数据库时指定UTF8MB4字符集，带有额外选项和模板",
			sql:           "CREATE DATABASE test_db WITH TEMPLATE template1 ENCODING = 'UTF8MB4' LC_COLLATE = 'en_US.utf8' LC_CTYPE = 'en_US.utf8';",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 10: 创建数据库时未指定字符集，带有模板",
			sql:           "CREATE DATABASE test_db WITH TEMPLATE template1;",
			expectResults: []*testResults{newTestResults()},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 11: 创建数据库时指定UTF8字符集",
			sql:           "CREATE DATABASE test_db WITH ENCODING = 'UTF8';",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "UTF8MB4")},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 12: 创建数据库时指定SQL_ASCII字符集",
			sql:           "CREATE DATABASE test_db WITH ENCODING = 'SQL_ASCII';",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "UTF8MB4")},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 13: 创建数据库时指定KOI8R字符集",
			sql:           "CREATE DATABASE test_db WITH ENCODING = 'KOI8R';",
			expectResults: []*testResults{newTestResults().addRuleAndMessageArgs(ruleName, "UTF8MB4")},
			expectError:   false,
			mockDesc:      "",
		},
		{
			desc:          "case 14: 创建数据库时指定UTF8MB4字符集，且使用大写ENCODING关键字",
			sql:           "CREATE DATABASE test_db WITH ENCODING = 'UTF8MB4';",
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
