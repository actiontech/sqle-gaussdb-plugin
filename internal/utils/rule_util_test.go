package utils

import (
	parser "github.com/pganalyze/pg_query_go/v2"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

type extractSubQueries struct {
	sql          string
	expectCount  int
	expectResult []string
	actualResult []string
}

func TestExtractSubQueries(t *testing.T) {
	testCases := []extractSubQueries{
		{
			sql:          "SELECT id,name FROM test",
			expectResult: []string{strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}} location:7}} location:7}} target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"name\"}} location:10}} location:10}} from_clause:{range_var:{relname:\"test\" inh:true relpersistence:\"p\" location:20}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE", " ", "")},
			expectCount:  1,
		},
		{
			sql:         "SELECT (select id from test) as id,name FROM test",
			expectCount: 2,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{name:\"id\"  val:{sub_link:{sub_link_type:EXPR_SUBLINK  subselect:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}  location:15}}  location:15}}  from_clause:{range_var:{relname:\"test\"  inh:true  relpersistence:\"p\"  location:23}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}  location:7}}  location:7}}  target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"name\"}}  location:35}}  location:35}}  from_clause:{range_var:{relname:\"test\"  inh:true  relpersistence:\"p\"  location:45}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE", " ", ""),
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}  location:15}}  location:15}}  from_clause:{range_var:{relname:\"test\"  inh:true  relpersistence:\"p\"  location:23}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE", " ", ""),
			},
		},
		{
			sql:         "SELECT id,name from (select id,name from test) t",
			expectCount: 2,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}} location:7}} location:7}} target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"name\"}} location:10}} location:10}} from_clause:{range_subselect:{subquery:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}} location:28}} location:28}} target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"name\"}} location:31}} location:31}} from_clause:{range_var:{relname:\"test\" inh:true relpersistence:\"p\" location:41}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE}} alias:{aliasname:\"t\"}}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE", " ", ""),
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}} location:28}} location:28}} target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"name\"}} location:31}} location:31}} from_clause:{range_var:{relname:\"test\" inh:true relpersistence:\"p\" location:41}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE", " ", ""),
			},
		},
		{
			sql:         "SELECT id,name from test where id in(select id from test)",
			expectCount: 2,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}  location:7}}  location:7}}  target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"name\"}}  location:10}}  location:10}}  from_clause:{range_var:{relname:\"test\"  inh:true  relpersistence:\"p\"  location:20}}  where_clause:{sub_link:{sub_link_type:ANY_SUBLINK  testexpr:{column_ref:{fields:{string:{str:\"id\"}}  location:31}}  subselect:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}  location:44}}  location:44}}  from_clause:{range_var:{relname:\"test\"  inh:true  relpersistence:\"p\"  location:52}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}  location:34}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE", " ", ""),
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}  location:44}}  location:44}}  from_clause:{range_var:{relname:\"test\"  inh:true  relpersistence:\"p\"  location:52}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE", " ", ""),
			},
		},
	}
	for _, testCase := range testCases {
		tree, err := parser.Parse(testCase.sql)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		switch stmt := tree.GetStmts()[0].GetStmt().GetNode().(type) {
		case *parser.Node_SelectStmt:
			assert.NotEmpty(t, stmt.SelectStmt)
			// 执行被测试的函数
			subQueries := &[]*parser.SelectStmt{}
			subQueries = ExtractSubQueries(tree.GetStmts()[0].GetStmt())
			assert.NoError(t, err)
			// 验证结果是否正确
			assert.Len(t, *subQueries, testCase.expectCount)
			actualResult := *subQueries
			for _, result := range actualResult {
				testCase.actualResult = append(testCase.actualResult,
					strings.ReplaceAll(result.String(), " ", ""))
			}
			assert.EqualValues(t, testCase.expectResult, testCase.actualResult)
		}
	}
}

// 测试方法:RecursiveSubQuery
func TestRecursiveSubQuery(t *testing.T) {
	testCases := []extractSubQueries{
		{
			sql:         "SELECT id,name from (select id,name from test) t",
			expectCount: 1,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}location:28}}location:28}}target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"name\"}}location:31}}location:31}}from_clause:{range_var:{relname:\"test\"inh:truerelpersistence:\"p\"location:41}}limit_option:LIMIT_OPTION_DEFAULTop:SETOP_NONE", " ", ""),
			},
		},
		{
			sql:         "SELECT id,name from test where id in(select id from test)",
			expectCount: 1,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}location:44}}location:44}}from_clause:{range_var:{relname:\"test\"inh:truerelpersistence:\"p\"location:52}}limit_option:LIMIT_OPTION_DEFAULTop:SETOP_NONE", " ", ""),
			},
		},
		{
			sql:         "SELECT (select id from test) as id,name FROM test",
			expectCount: 1,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"id\"}}location:15}}location:15}}from_clause:{range_var:{relname:\"test\"inh:truerelpersistence:\"p\"location:23}}limit_option:LIMIT_OPTION_DEFAULTop:SETOP_NONE", " ", ""),
			},
		},
		{
			sql:         "select customer_id, order_month, total_sales from (select customer_id, extract(month from order_date) as order_month, sum(quantity * price) as total_sales, row_number() over (partition by customer_id order by sum(quantity * price) desc) as rn from sales group by customer_id, extract(month from order_date)) as subquery where rn = 1",
			expectCount: 1,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"customer_id\"}}location:58}}location:58}}target_list:{res_target:{name:\"order_month\"val:{func_call:{funcname:{string:{str:\"pg_catalog\"}}funcname:{string:{str:\"date_part\"}}args:{a_const:{val:{string:{str:\"month\"}}location:79}}args:{column_ref:{fields:{string:{str:\"order_date\"}}location:90}}location:71}}location:71}}target_list:{res_target:{name:\"total_sales\"val:{func_call:{funcname:{string:{str:\"sum\"}}args:{a_expr:{kind:AEXPR_OPname:{string:{str:\"*\"}}lexpr:{column_ref:{fields:{string:{str:\"quantity\"}}location:122}}rexpr:{column_ref:{fields:{string:{str:\"price\"}}location:133}}location:131}}location:118}}location:118}}target_list:{res_target:{name:\"rn\"val:{func_call:{funcname:{string:{str:\"row_number\"}}over:{partition_clause:{column_ref:{fields:{string:{str:\"customer_id\"}}location:188}}order_clause:{sort_by:{node:{func_call:{funcname:{string:{str:\"sum\"}}args:{a_expr:{kind:AEXPR_OPname:{string:{str:\"*\"}}lexpr:{column_ref:{fields:{string:{str:\"quantity\"}}location:213}}rexpr:{column_ref:{fields:{string:{str:\"price\"}}location:224}}location:222}}location:209}}sortby_dir:SORTBY_DESCsortby_nulls:SORTBY_NULLS_DEFAULTlocation:-1}}frame_options:1058location:174}location:156}}location:156}}from_clause:{range_var:{relname:\"sales\"inh:truerelpersistence:\"p\"location:248}}group_clause:{column_ref:{fields:{string:{str:\"customer_id\"}}location:263}}group_clause:{func_call:{funcname:{string:{str:\"pg_catalog\"}}funcname:{string:{str:\"date_part\"}}args:{a_const:{val:{string:{str:\"month\"}}location:284}}args:{column_ref:{fields:{string:{str:\"order_date\"}}location:295}}location:276}}limit_option:LIMIT_OPTION_DEFAULTop:SETOP_NONE", " ", ""),
			},
		},
		{
			sql:         "SELECT * FROM (VALUES (1, 'John', 25), (2, 'Alice', 30), (3, 'Bob', 35)) users(id, name, age)",
			expectCount: 1,
			expectResult: []string{
				strings.ReplaceAll("values_lists:{list:{items:{a_const:{val:{integer:{ival:1}}location:23}}items:{a_const:{val:{string:{str:\"John\"}}location:26}}items:{a_const:{val:{integer:{ival:25}}location:34}}}}values_lists:{list:{items:{a_const:{val:{integer:{ival:2}}location:40}}items:{a_const:{val:{string:{str:\"Alice\"}}location:43}}items:{a_const:{val:{integer:{ival:30}}location:52}}}}values_lists:{list:{items:{a_const:{val:{integer:{ival:3}}location:58}}items:{a_const:{val:{string:{str:\"Bob\"}}location:61}}items:{a_const:{val:{integer:{ival:35}}location:68}}}}limit_option:LIMIT_OPTION_DEFAULTop:SETOP_NONE", " ", ""),
			},
		},
		{
			sql:         "WITH sales_data AS (SELECT customer_id, sum(amount) AS total_sales FROM sales GROUP BY customer_id) SELECT customers.id, customers.name, sales_data.total_sales FROM customers JOIN sales_data ON customers.id = sales_data.customer_id",
			expectCount: 1,
			expectResult: []string{
				strings.ReplaceAll("target_list:{res_target:{val:{column_ref:{fields:{string:{str:\"customer_id\"}}location:27}}location:27}}target_list:{res_target:{name:\"total_sales\"val:{func_call:{funcname:{string:{str:\"sum\"}}args:{column_ref:{fields:{string:{str:\"amount\"}}location:44}}location:40}}location:40}}from_clause:{range_var:{relname:\"sales\"inh:truerelpersistence:\"p\"location:72}}group_clause:{column_ref:{fields:{string:{str:\"customer_id\"}}location:87}}limit_option:LIMIT_OPTION_DEFAULTop:SETOP_NONE", " ", ""),
			},
		},
	}
	for _, testCase := range testCases {
		tree, err := parser.Parse(testCase.sql)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(tree.GetStmts()))
		switch stmt := tree.GetStmts()[0].GetStmt().GetNode().(type) {
		case *parser.Node_SelectStmt:
			assert.NotEmpty(t, stmt.SelectStmt)
			// 执行被测试的函数
			subQueries := &[]*parser.SelectStmt{}
			subQueries = RecursiveSubQuery(tree.GetStmts()[0].GetStmt().GetSelectStmt())
			assert.NoError(t, err)
			// 验证结果是否正确
			assert.Len(t, *subQueries, testCase.expectCount)
			actualResult := *subQueries
			for _, result := range actualResult {
				testCase.actualResult = append(testCase.actualResult,
					strings.ReplaceAll(result.String(), " ", ""))
			}
			assert.EqualValues(t, testCase.expectResult, testCase.actualResult)
		}
	}
}
