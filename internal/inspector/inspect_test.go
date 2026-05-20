package inspector

import (
	"context"
	"strings"
	"testing"

	parser "actiontech.cloud/sqle/pg_query_go/v5"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"

	"github.com/stretchr/testify/assert"
)

func TestEstimateSQLAffectRows(t *testing.T) {
	impl := NewMockDriver(nil, "", "", nil, nil)
	inputs := []struct {
		TestName      string
		Sql           string
		ExpectedCount int
		ExpectedSql   string
	}{
		// select stmt
		{
			TestName:    `[select]not need convert`,
			Sql:         `select count(*) from db1.tb1`,
			ExpectedSql: `select count(*) from db1.tb1`,
		},
		{
			TestName:    `[select]not need convert`,
			Sql:         `select count(*) from tb1`,
			ExpectedSql: `select count(*) from tb1`,
		},
		{
			TestName:    `[select]not need convert`,
			Sql:         `select count(*) from db1.tb1 where a = 1 and b = 2`,
			ExpectedSql: `select count(*) from db1.tb1 where a = 1 and b = 2`,
		},
		{
			TestName:    `[select]not need convert`,
			Sql:         `select count(*) from db1.tb1 where a = 1 limit 10`,
			ExpectedSql: `select count(*) from db1.tb1 where a = 1 limit 10`,
		},
		{
			TestName:    `[select]need convert`,
			Sql:         `select a from db1.tb1 where b = 1 limit 10`,
			ExpectedSql: `select count(*) from db1.tb1 where b = 1 limit 10`,
		},
		{
			TestName:    `[select]need convert`,
			Sql:         `select * from db1.tb1 where b = 1 limit 10`,
			ExpectedSql: `select count(*) from db1.tb1 where b = 1 limit 10`,
		},
		{
			TestName:    `[select]need convert`,
			Sql:         `select * from db1.tb1 where b = 1 order by a limit 10`,
			ExpectedSql: `select count(*) from db1.tb1 where b = 1 order by a limit 10`,
		},
		{
			TestName:    `[select]need convert with subquery`,
			Sql:         `select (select a from tb2 where b=1) from db1.tb1 where c = 1 limit 10`,
			ExpectedSql: `select count(*) from db1.tb1 where c = 1 limit 10`,
		},
		{
			TestName:    `[select]need convert with subquery`,
			Sql:         `select (select a from tb2 where b=1) from db1.tb1 where c in (select id from tb3) limit 10`,
			ExpectedSql: `select count(*) from db1.tb1 where c in (select id from tb3) limit 10`,
		},
		{
			TestName:    `[select]need convert with join`,
			Sql:         `select tb1.a, tb2.a from tb1 join tb2 using (b)`,
			ExpectedSql: `select count(*) from tb1 join tb2 using (b)`,
		},
		{
			TestName:    `[select]need convert with join`,
			Sql:         `select tb1.a, tb2.a from tb1 join tb2 on tb1.a = tb2.b`,
			ExpectedSql: `select count(*) from tb1 join tb2 on tb1.a = tb2.b`,
		},
		{
			TestName:    `[select]select into`,
			Sql:         `SELECT * INTO tb2 FROM tb1 WHERE a > 1`,
			ExpectedSql: `select count(*) from tb1 where a > 1`,
		},
		{
			TestName:    `[select]select for update`,
			Sql:         `select * from goods where id = 1 for update;`,
			ExpectedSql: `select count(*) from goods where id = 1`,
		},
		{
			TestName:    `[select]need convert with group by`,
			Sql:         `select a, sum(b) as total from tb6 group by a`,
			ExpectedSql: `select count(*) from tb6`,
		},

		// insert stmt
		{
			TestName:      `[insert]need convert`,
			Sql:           `insert into db1.tb1 values (1,2),(3,4)`,
			ExpectedCount: 2,
		},
		{
			TestName:    `[insert]need convert insert select`,
			Sql:         `insert into t1 (id) select id from t2`,
			ExpectedSql: `select count(*) from t2`,
		},

		// update stmt
		{
			TestName:    `[update]need convert`,
			Sql:         `update tb1 set a = 1`,
			ExpectedSql: `select count(*) from tb1`,
		},
		{
			TestName:    `[update]need convert`,
			Sql:         `update db1.tb1 set a = 1`,
			ExpectedSql: `select count(*) from db1.tb1`,
		},
		{
			TestName:    `[update]need convert`,
			Sql:         `update db1.tb1 set a = 1 where b = 2`,
			ExpectedSql: `select count(*) from db1.tb1 where b = 2`,
		},
		{
			TestName:    `[update]need convert`,
			Sql:         `update db1.tb1 set a = 1 where b = 2 and c = 3`,
			ExpectedSql: `select count(*) from db1.tb1 where b = 2 and c = 3`,
		},
		{
			TestName:    `[update]need convert`,
			Sql:         `update db1.tb1 set (a, b, c) = (a+1, b+1, c+1) where b = 2`,
			ExpectedSql: `select count(*) from db1.tb1 where b = 2`,
		},
		{
			TestName:    `[update]need convert`,
			Sql:         `update tb1 set id = (select id from tb1 where id = 2) where id = 2`,
			ExpectedSql: `select count(*) from tb1 where id = 2`,
		},
		{
			TestName:    `[update]need convert with returning`,
			Sql:         `update db1.tb1 set (a, b, c) = (a+1, b+1, c+1) where b = 2 returning a,b`,
			ExpectedSql: `select count(*) from db1.tb1 where b = 2`,
		},
		//{  // todo luowei
		//	TestName:    `[update]need convert with join`,
		//	RawStmt:         `update db1.tb1 set a = a+1 from tb2 where tb1.a = tb2.b`,
		//	ExpectedSql: ``,
		//},
		{
			TestName:    `[update]need convert with subquery`,
			Sql:         `update db1.tb1 set a = a+1 where b = (select c from tb2 where tb2.c = 1)`,
			ExpectedSql: `select count(*) from db1.tb1 where b = (select c from tb2 where tb2.c = 1)`,
		},
		{
			TestName:    `[update]need convert with subquery`,
			Sql:         `update db1.tb1 set (a,b) = (select c,d from tb2 where tb2.e=2)`,
			ExpectedSql: `select count(*) from db1.tb1`,
		},

		// delete stmt
		{
			TestName:    `[delete]need convert`,
			Sql:         `delete from db1.tb1`,
			ExpectedSql: `select count(*) from db1.tb1`,
		},
		{
			TestName:    `[delete]need convert`,
			Sql:         `delete from db1.tb1 where a = 1`,
			ExpectedSql: `select count(*) from db1.tb1 where a = 1`,
		},
		{
			TestName:    `[delete]need convert with returning`,
			Sql:         `delete from db1.tb1 where a = 1 returning *`,
			ExpectedSql: `select count(*) from db1.tb1 where a = 1`,
		},
		//{ // todo luowei
		//	TestName:    `[delete]need convert with join`,
		//	RawStmt:         `delete from tb1 using tb6 where tb6.b = tb1.a`,
		//	ExpectedSql: `select count(*) from tb1 join tb6 on tb6.b = tb1.a`,
		//},
	}

	for _, in := range inputs {
		t.Run(in.TestName, func(t *testing.T) {
			actualSql, count, err := impl.convertSql(in.Sql)
			assert.NoError(t, err)
			assert.Equal(t, in.ExpectedCount, count)
			assert.Equal(t, in.ExpectedSql, strings.ToLower(actualSql))
		})
	}

	// 不是dml的sql
	args := []string{
		`create table tb1(a int)`,
		`alter table tb1 add column a int`,
		`drop database db1`,
		`drop table tb1`,
	}
	for _, s := range args {
		t.Run("not dml", func(t *testing.T) {
			_, _, err := impl.convertSql(s)
			assert.EqualError(t, err, "only support estimating affected rows of DML")
		})
	}

	// 不支持的语法
	args2 := []struct {
		TestName string
		Sql      string
		errMsg   string
	}{
		{
			TestName: `select union`,
			Sql:      `select a from b union select x from y limit 10`,
			errMsg:   `do not support union clause`,
		}, {
			TestName: `select with`,
			Sql: `
WITH RECURSIVE t(n) AS (
    VALUES (1)
  UNION ALL
    SELECT n+1 FROM t WHERE n < 100
)
SELECT sum(n) FROM t;`,
			errMsg: `do not support with clause`,
		}, {
			TestName: `select window clause`,
			Sql: `
SELECT sum(salary) OVER w, avg(salary) OVER w
  FROM empsalary
  WINDOW w AS (PARTITION BY depname ORDER BY salary DESC);`,
			errMsg: `do not support window clause`,
		}, {
			TestName: `update with clause`,
			Sql: `
WITH x AS (
   SELECT  psp_id
   FROM    global.prospect
   WHERE   status IN ('new', 'reset')
   ORDER   BY request_ts
   LIMIT   1
   )
UPDATE global.prospect psp
SET    status = status || '*'
FROM   x
WHERE  psp.psp_id = x.psp_id
RETURNING psp.*;`,
			errMsg: `do not support with clause`,
		}, {
			TestName: `delete with clause`,
			Sql: `
WITH max_table as (
    SELECT uid, max(timestamp) - 10000 as mx
    FROM listens 
    GROUP BY uid
) 
DELETE FROM listens 
WHERE timestamp < (SELECT mx
                   FROM max_table 
                   where max_table.uid = listens.uid);`,
			errMsg: `do not support with clause`,
		}, {
			TestName: `insert with clause`,
			Sql: `
WITH get_cust_code_for_cust_id AS (
    SELECT cust_code 
    FROM cust 
    WHERE cust_id=11
)
INSERT INTO public.table_1 (cust_code, issue, status, created_on)
SELECT cust_code, 'New Issue', 'Open', current_timestamp 
FROM get_cust_code_for_cust_id;`,
			errMsg: `do not support with clause`,
		},
	}
	for _, a := range args2 {
		t.Run(a.TestName, func(t *testing.T) {
			_, _, err := impl.convertSql(a.Sql)
			assert.EqualError(t, err, a.errMsg)
		})
	}
}

func TestCheckConvertedSql(t *testing.T) {
	args := []struct {
		ConvertedSql string
		ShouldError  bool
	}{
		{
			ConvertedSql: `select count(*) from tb1`,
			ShouldError:  false,
		}, {
			ConvertedSql: `select count(*) from tb1 where a = 1`,
			ShouldError:  false,
		}, {
			ConvertedSql: `select count(*) from tb1 join tb6 on tb6.b = tb1.a`,
			ShouldError:  false,
		}, {
			ConvertedSql: `select count(*) from db1.tb1 where b = (select c from tb2 where tb2.c = 1)`,
			ShouldError:  false,
		}, {
			ConvertedSql: `select count(*) from db1.tb1 where c in (select id from tb3) limit 10`,
			ShouldError:  false,
		}, {
			ConvertedSql: `select count(*) from db1.tb1 where b = 1 order by a limit 10`,
			ShouldError:  false,
		}, {
			ConvertedSql: `select count(*) from tb1 join tb2 using (b)`,
			ShouldError:  false,
		},
		{
			ConvertedSql: `select * from tb1`,
			ShouldError:  true,
		},
		{
			ConvertedSql: `create table t1(id int ,name varchar(255), primary key(id));`,
			ShouldError:  true,
		},
		{
			ConvertedSql: `create table t1(id int ,name varchar(255), unique(id));`,
			ShouldError:  true,
		},
		{
			ConvertedSql: `create table test_column_type(id int,name varrach(100));`,
			ShouldError:  true,
		},
		{
			ConvertedSql: `alter table test_column_type add new_column varchar1(100);`,
			ShouldError:  true,
		},
		{
			ConvertedSql: `ALTER TABLE test.test_alter_column_type ALTER COLUMN id TYPE VARCHAR1(255) USING id::int;`,
			ShouldError:  true,
		},
		{
			ConvertedSql: `ALTER TABLE test.test_alter_column_type ALTER COLUMN id TYPE VARCHAR(255) USING id::int1;`,
			ShouldError:  true,
		},
	}

	for _, arg := range args {
		t.Run("", func(t *testing.T) {
			err := checkConvertedSql(arg.ConvertedSql)
			if arg.ShouldError {
				if !assert.Error(t, err) {
					t.Error("expect error but actually no error")
				}
			} else {
				if !assert.NoError(t, err) {
					t.Errorf("expect no error but actually got error: %v", err)
				}
			}
		})
	}
}

func TestExtractTableFromSQL(t *testing.T) {
	impl := NewMockDriver(nil, "", "", nil, nil)

	args := []struct {
		Name         string
		SQL          string
		ExpectTables []*driverV2.Table
	}{
		{
			Name: "select sub query",
			SQL:  `SELECT student.sid, sname, ss FROM student, (SELECT sid, avg(score) AS ss FROM sc GROUP BY sid HAVING avg(score) > 60) r WHERE student.sid = r.sid`,
			ExpectTables: []*driverV2.Table{
				{
					Name:   "student",
					Schema: "public",
				},
				{
					Name:   "sc",
					Schema: "public",
				},
			},
		},
	}

	for _, arg := range args {
		t.Run(arg.Name, func(t *testing.T) {
			tbs, err := impl.ExtractTableFromSQL(context.TODO(), arg.SQL)
			assert.NoError(t, err)
			assert.EqualValues(t, arg.ExpectTables, tbs)
		})
	}
}

func TestValidateSqlDataType(t *testing.T) {
	args := []struct {
		sql            string
		dbTypeNameList string
		result         bool
	}{
		{
			sql:            `CREATE TABLE my_schedule (id SERIAL PRIMARY KEY,day weekday,task VARCHAR(255));`,
			dbTypeNameList: "weekday,weekday1,VARCHAR",
			result:         true,
		}, {
			sql:            `CREATE TABLE my_schedule (id SERIAL PRIMARY KEY,day weekdayxxxx,task VARCHAR(255));`,
			dbTypeNameList: "weekday,weekday1, VARCHAR",
			result:         false,
		},
		{
			sql:            `CREATE TABLE my_schedule (id SERIAL PRIMARY KEY,day int,task VARCHAR(255),"create_time" timestamptz(6));`,
			dbTypeNameList: "weekday,weekday1,int4,VARCHAR,timestamptz",
			result:         true,
		}, {
			sql:            `CREATE TABLE my_schedule (id SERIAL PRIMARY KEY,day int,task VARCHAR(255),"create_time" timestamptzabc(6));`,
			dbTypeNameList: "weekday,weekday1,int4,VARCHAR",
			result:         false,
		},
		{
			sql:            `alter table test_column_type add new_column weekday(100);`,
			dbTypeNameList: "weekday,weekday1",
			result:         true,
		}, {
			sql:            `alter table test_column_type add new_column weekdayxxx(100);`,
			dbTypeNameList: "weekday,weekday1",
			result:         false,
		},
		{
			sql:            `alter table test_column_type add new_column timestamptz(6);`,
			dbTypeNameList: "weekday,weekday1,timestamptz",
			result:         true,
		}, {
			sql:            `alter table test_column_type add new_column timestamptzok(100);`,
			dbTypeNameList: "weekday,weekday1",
			result:         false,
		},
	}

	for _, arg := range args {
		t.Run("", func(t *testing.T) {
			ast, err := SqlParserFunc(arg.sql)
			assert.NoError(t, err)
			dbTypeNameMap := make(map[string]string)
			dbTypeNameList := strings.Split(arg.dbTypeNameList, ",")
			for _, typeName := range dbTypeNameList {
				if _, ok := dbTypeNameMap[strings.ToLower(typeName)]; !ok {
					dbTypeNameMap[strings.ToLower(typeName)] = strings.ToLower(typeName)
				}
			}
			err = validateDataType(ast.(*parser.RawStmt), dbTypeNameMap)
			if arg.result {
				if !assert.NoError(t, err) {
					t.Errorf("expect no error but actually got error: %v", err)
				}
			} else {
				if !assert.Error(t, err) {
					t.Error("expect error but actually no error")
				}
			}
		})
	}
}
