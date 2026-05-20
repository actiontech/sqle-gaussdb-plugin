package inspector

import (
	"context"
	"testing"

	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

// ==== Rule test code start ====
func TestRuleSQLE00082(t *testing.T) {
	ruleName := SQLE00082
	type testCase struct {
		desc          string
		sql           string
		expectResults []*testResults
		expectError   bool
		mockDesc      string
		mockPlan      string
	}

	//===== test cases =====
	tests := []testCase{
		{
			desc:          "用例 1: 查询语句触发外部排序",
			sql:           "SELECT * FROM exist_tb_1 ORDER BY c2_with_idx;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Method: external sort\"",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external sort"}}]`,
		},
		{
			desc:          "用例 2: 查询语句触发磁盘排序",
			sql:           "SELECT * FROM exist_tb_2 ORDER BY c2;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Method: disk\"",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Space Type": "Disk"}}]`,
		},
		{
			desc:          "用例 3: 查询语句不触发文件排序",
			sql:           "SELECT * FROM exist_tb_3 ORDER BY c1;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果不包含 \"Sort Method\" 字段",
			mockPlan:      `[{"Plan": {"Node Type": "Sort"}}]`,
		},
		{
			desc:          "用例 4: 查询语句不包含排序",
			sql:           "SELECT * FROM exist_tb_9;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果不包含 \"Sort Method\" 字段",
			mockPlan:      `[{"Plan": {"Node Type": "Seq Scan"}}]`,
		},
		{
			desc:          "用例 5: 查询语句触发内存排序",
			sql:           "SELECT * FROM exist_tb_9 ORDER BY c2_with_idx;",
			expectResults: _RuleAuditPassResults,
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Method: quicksort\"",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "quicksort"}}]`,
		},
		{
			desc:          "用例 6: 查询语句触发外部排序",
			sql:           "SELECT DISTINCT(c2_with_idx) FROM exist_tb_1 ORDER BY c2_with_idx DESC LIMIT 10;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Method: external sort\"",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external sort"}}]`,
		},
		{
			desc:          "用例 7: 查询语句触发磁盘排序",
			sql:           "SELECT c2 FROM exist_tb_2 GROUP BY c2 ORDER BY c2 DESC;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Method: disk\"",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Space Type": "Disk"}}]`,
		},
		{
			desc:          "用例 9: 联合查询触发外部排序",
			sql:           "(SELECT c2_with_idx FROM exist_tb_1 GROUP BY c2_with_idx) UNION ALL (SELECT c2_with_idx FROM exist_tb_1 GROUP BY c2_with_idx);",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Method: external sort\"",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external sort"}}]`,
		},
		{
			desc:          "用例 10: CTE 查询触发外部排序",
			sql:           "WITH tmp AS (SELECT c2_with_idx FROM exist_tb_1 ORDER BY c2_with_idx DESC) SELECT * FROM tmp;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Method: external sort\"",
			mockPlan:      `[{"Plan": {"Node Type": "Sort", "Sort Method": "external sort"}}]`,
		},

		{
			desc:          "用例 11: SELECT c2 FROM exist_tb_2 GROUP BY c2 ORDER BY c2 DESC;",
			sql:           "SELECT c2 FROM exist_tb_2 GROUP BY c2 ORDER BY c2 DESC;",
			expectResults: []*testResults{newTestResults().add(ruleName)},
			expectError:   false,
			mockDesc:      "模拟待测 sql explain (analyze, json) 返回结果包含 \"Sort Space Type\": \"Disk\"",
			mockPlan: ` [                                                          
   {                                                        
     "Plan": {                                              
       "Node Type": "Limit",                                
       "Parallel Aware": false,                             
       "Startup Cost": 282005.16,                           
       "Total Cost": 282005.19,                             
       "Plan Rows": 10,                                     
       "Plan Width": 516,                                   
       "Actual Startup Time": 32236.171,                    
       "Actual Total Time": 32236.172,                      
       "Actual Rows": 1,                                    
       "Actual Loops": 1,                                   
       "Plans": [                                           
         {                                                  
           "Node Type": "Sort",                             
           "Parent Relationship": "Outer",                  
           "Parallel Aware": false,                         
           "Startup Cost": 282005.16,                       
           "Total Cost": 282005.41,                         
           "Plan Rows": 100,                                
           "Plan Width": 516,                               
           "Actual Startup Time": 32236.168,                
           "Actual Total Time": 32236.168,                  
           "Actual Rows": 1,                                
           "Actual Loops": 1,                               
           "Sort Key": ["mark1 DESC"],                      
           "Sort Method": "quicksort",                      
           "Sort Space Used": 29,                           
           "Sort Space Type": "Disk",                     
           "Plans": [                                       
             {                                              
               "Node Type": "Aggregate",                    
               "Strategy": "Hashed",                        
               "Partial Mode": "Simple",                    
               "Parent Relationship": "Outer",              
               "Parallel Aware": false,                     
               "Startup Cost": 282002.00,                   
               "Total Cost": 282003.00,                     
               "Plan Rows": 100,                            
               "Plan Width": 516,                           
               "Actual Startup Time": 32236.107,            
               "Actual Total Time": 32236.111,              
               "Actual Rows": 1,                            
               "Actual Loops": 1,                           
               "Group Key": ["mark1"],                      
               "Plans": [                                   
                 {                                          
                   "Node Type": "Remote Subquery Scan",     
                   "Parent Relationship": "Outer",          
                   "Parallel Aware": true,                  
                   "Replicated": "no",                      
                   "Node List": ["dn001", "dn002"],         
                   "Startup Cost": 100.00,                  
                   "Total Cost": 280752.00,                 
                   "Plan Rows": 500000,                     
                   "Plan Width": 516,                       
                   "Actual Startup Time": 58.701,           
                   "Actual Total Time": 25089.246,          
                   "Actual Rows": 1000000,                  
                   "Actual Loops": 1,                       
                   "Plans": [                               
                     {                                      
                       "Node Type": "Gather",               
                       "Parent Relationship": "Outer",      
                       "Parallel Aware": false,             
                       "Startup Cost": 0.00,                
                       "Total Cost": 20152.00,              
                       "Plan Rows": 1000000,                
                       "Plan Width": 516,                   
                       "Data Node": "ALL",                  
                       "Actual Min Startup Time": 48.706,   
                       "Actual Max Startup Time": 67.401,   
                       "Actual Min Total Time": 5213.300,   
                       "Actual Max Total Time": 5297.225,   
                       "Actual Min Rows": 499436,           
                       "Actual Max Rows": 500564,           
                       "Actual Min Loops": 1,               
                       "Actual Max Loops": 1,               
                       "Workers Planned": 2,                
                       "Workers Launched": 2,               
                       "Single Copy": false,                
                       "Plans": [                           
                         {                                  
                           "Node Type": "Seq Scan",         
                           "Parent Relationship": "Outer",  
                           "Parallel Aware": true,          
                           "Relation Name": "customers",    
                           "Alias": "customers",            
                           "Startup Cost": 0.00,            
                           "Total Cost": 20152.00,          
                           "Plan Rows": 500000,             
                           "Plan Width": 516,               
                           "Data Node": "ALL",              
                           "Actual Min Startup Time": 0.111,
                           "Actual Max Startup Time": 0.182,
                           "Actual Min Total Time": 405.080,
                           "Actual Max Total Time": 442.700,
                           "Actual Min Rows": 249718,       
                           "Actual Max Rows": 250282,       
                           "Actual Min Loops": 2,           
                           "Actual Max Loops": 2            
                         }                                  
                       ]                                    
                     }                                      
                   ]                                        
                 }                                          
               ]                                            
             }                                              
           ]                                                
         }                                                  
       ]                                                    
     },                                                     
     "Planning Time": 0.547,                                
     "Triggers": [                                          
     ],                                                     
     "Execution Time": 32240.868                            
   }                                                        
 ]`,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d, ctx := setupForRuleSQLE00082(t, ruleName, test.sql, test.mockPlan)
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

func setupForRuleSQLE00082(t *testing.T, ruleName string, sql, mockPlan string) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, mock, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	//在 mock 中加入数据库返回的rows
	if mockPlan != "" {
		mock.ExpectQuery("EXPLAIN (ANALYZE, FORMAT JSON) " + sql).
			WillReturnRows(mock.NewRows([]string{"QUERY PLAN"}).AddRow(mockPlan))
	}

	// mock 规则审核器
	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, params.Params{}, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

// ==== Rule test code end ====
