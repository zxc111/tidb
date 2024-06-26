// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package executor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/testkit"
)

func TestWindowFunctions(t *testing.T) {
	store := testkit.CreateMockStore(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("set @@tidb_window_concurrency = 1")
	tk.MustExec("set @@tidb_enable_pipelined_window_function = 0")
	doTestWindowFunctions(tk)

	// TestWindowParallelFunctions
	tk.MustExec("set @@tidb_window_concurrency = 4")
	tk.MustExec("set @@tidb_enable_pipelined_window_function = 0")
	doTestWindowFunctions(tk)

	// TestPipelinedWindowFunctions
	tk.MustExec("set @@tidb_window_concurrency = 1")
	tk.MustExec("set @@tidb_enable_pipelined_window_function = 1")
	doTestWindowFunctions(tk)

	// TestPipelinedWindowParallelFunctions
	tk.MustExec("set @@tidb_window_concurrency = 4")
	tk.MustExec("set @@tidb_enable_pipelined_window_function = 1")
	doTestWindowFunctions(tk)
}

func doTestWindowFunctions(tk *testkit.TestKit) {
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t (a int, b int, c int)")
	tk.MustExec("set @@tidb_enable_window_function = 1")
	defer func() {
		tk.MustExec("set @@tidb_enable_window_function = 0")
	}()
	tk.MustExec("insert into t values (1,2,3),(4,3,2),(2,3,4)")
	tk.MustQuery("select count(a) over () from t").
		Check(testkit.Rows("3", "3", "3"))
	tk.MustQuery("select sum(a) over () + count(a) over () from t").
		Check(testkit.Rows("10", "10", "10"))
	tk.MustQuery("select sum(a) over (partition by a) from t").Sort().
		Check(testkit.Rows("1", "2", "4"))
	tk.MustQuery("select 1 + sum(a) over (), count(a) over () from t").
		Check(testkit.Rows("8 3", "8 3", "8 3"))
	tk.MustQuery("select sum(t1.a) over() from t t1, t t2").
		Check(testkit.Rows("21", "21", "21", "21", "21", "21", "21", "21", "21"))
	tk.MustQuery("select _tidb_rowid, sum(t.a) over() from t").
		Check(testkit.Rows("1 7", "2 7", "3 7"))

	tk.MustQuery("select a, row_number() over() from t").
		Check(testkit.Rows("1 1", "4 2", "2 3"))
	tk.MustQuery("select a, row_number() over(partition by a) from t").Sort().
		Check(testkit.Rows("1 1", "2 1", "4 1"))

	tk.MustQuery("select a, sum(a) over(rows between unbounded preceding and 1 following) from t").
		Check(testkit.Rows("1 5", "4 7", "2 7"))
	tk.MustQuery("select a, sum(a) over(rows between 1 preceding and 1 following) from t").
		Check(testkit.Rows("1 5", "4 7", "2 6"))
	tk.MustQuery("select a, sum(a) over(rows between unbounded preceding and 1 preceding) from t").
		Check(testkit.Rows("1 <nil>", "4 1", "2 5"))

	tk.MustExec("drop table t")
	tk.MustExec("create table t(a int, b date)")
	tk.MustExec("insert into t values (null,null),(1,20190201),(2,20190202),(3,20190203),(5,20190205)")
	tk.MustQuery("select a, sum(a) over(order by a range between 1 preceding and 2 following) from t").
		Check(testkit.Rows("<nil> <nil>", "1 6", "2 6", "3 10", "5 5"))
	tk.MustQuery("select a, sum(a) over(order by a desc range between 1 preceding and 2 following) from t").
		Check(testkit.Rows("5 8", "3 6", "2 6", "1 3", "<nil> <nil>"))
	tk.MustQuery("select a, b, sum(a) over(order by b range between interval 1 day preceding and interval 2 day following) from t").
		Check(testkit.Rows("<nil> <nil> <nil>", "1 2019-02-01 6", "2 2019-02-02 6", "3 2019-02-03 10", "5 2019-02-05 5"))
	tk.MustQuery("select a, b, sum(a) over(order by b desc range between interval 1 day preceding and interval 2 day following) from t").
		Check(testkit.Rows("5 2019-02-05 8", "3 2019-02-03 6", "2 2019-02-02 6", "1 2019-02-01 3", "<nil> <nil> <nil>"))

	tk.MustExec("drop table t")
	tk.MustExec("CREATE TABLE t (id INTEGER, sex CHAR(1))")
	tk.MustExec("insert into t values (1, 'M'), (2, 'F'), (3, 'F'), (4, 'F'), (5, 'M'), (10, NULL), (11, NULL)")
	tk.MustQuery("SELECT sex, id, RANK() OVER (PARTITION BY sex ORDER BY id DESC) FROM t").Sort().
		Check(testkit.Rows("<nil> 10 2", "<nil> 11 1", "F 2 3", "F 3 2", "F 4 1", "M 1 2", "M 5 1"))

	tk.MustExec("drop table t")
	tk.MustExec("create table t(a int, b int)")
	tk.MustExec("insert into t values (1,1),(1,2),(2,1),(2,2)")
	tk.MustQuery("select a, b, rank() over() from t").
		Check(testkit.Rows("1 1 1", "1 2 1", "2 1 1", "2 2 1"))
	tk.MustQuery("select a, b, rank() over(order by a) from t").
		Check(testkit.Rows("1 1 1", "1 2 1", "2 1 3", "2 2 3"))
	tk.MustQuery("select a, b, rank() over(order by a, b) from t").
		Check(testkit.Rows("1 1 1", "1 2 2", "2 1 3", "2 2 4"))

	tk.MustQuery("select a, b, dense_rank() over() from t").
		Check(testkit.Rows("1 1 1", "1 2 1", "2 1 1", "2 2 1"))
	tk.MustQuery("select a, b, dense_rank() over(order by a) from t").
		Check(testkit.Rows("1 1 1", "1 2 1", "2 1 2", "2 2 2"))
	tk.MustQuery("select a, b, dense_rank() over(order by a, b) from t").
		Check(testkit.Rows("1 1 1", "1 2 2", "2 1 3", "2 2 4"))

	tk.MustQuery("select row_number() over(rows between 1 preceding and 1 following) from t").
		Check(testkit.Rows("1", "2", "3", "4"))
	tk.MustQuery("show warnings").
		Check(testkit.Rows("Note 3599 Window function 'row_number' ignores the frame clause of window '<unnamed window>' and aggregates over the whole partition"))

	tk.MustQuery("select a, sum(a) over() from t").
		Check(testkit.Rows("1 6", "1 6", "2 6", "2 6"))
	tk.MustQuery("select a, sum(a) over(order by a) from t").
		Check(testkit.Rows("1 2", "1 2", "2 6", "2 6"))
	tk.MustQuery("select a, sum(a) over(order by a, b) from t").
		Check(testkit.Rows("1 1", "1 2", "2 4", "2 6"))

	tk.MustQuery("select a, first_value(a) over(), last_value(a) over() from t").
		Check(testkit.Rows("1 1 2", "1 1 2", "2 1 2", "2 1 2"))
	tk.MustQuery("select a, first_value(a) over(rows between 1 preceding and 1 following), last_value(a) over(rows between 1 preceding and 1 following) from t").
		Check(testkit.Rows("1 1 1", "1 1 2", "2 1 2", "2 2 2"))
	tk.MustQuery("select a, first_value(a) over(rows between 1 following and 1 following), last_value(a) over(rows between 1 following and 1 following) from t").
		Check(testkit.Rows("1 1 1", "1 2 2", "2 2 2", "2 <nil> <nil>"))
	tk.MustQuery("select a, first_value(rand(0)) over(), last_value(rand(0)) over() from t").
		Check(testkit.Rows("1 0.15522042769493574 0.33109208227236947", "1 0.15522042769493574 0.33109208227236947",
			"2 0.15522042769493574 0.33109208227236947", "2 0.15522042769493574 0.33109208227236947"))

	tk.MustQuery("select a, b, cume_dist() over() from t").
		Check(testkit.Rows("1 1 1", "1 2 1", "2 1 1", "2 2 1"))
	tk.MustQuery("select a, b, cume_dist() over(order by a) from t").
		Check(testkit.Rows("1 1 0.5", "1 2 0.5", "2 1 1", "2 2 1"))
	tk.MustQuery("select a, b, cume_dist() over(order by a, b) from t").
		Check(testkit.Rows("1 1 0.25", "1 2 0.5", "2 1 0.75", "2 2 1"))

	tk.MustQuery("select a, nth_value(a, null) over() from t").
		Check(testkit.Rows("1 <nil>", "1 <nil>", "2 <nil>", "2 <nil>"))
	tk.MustQuery("select a, nth_value(a, 1) over() from t").
		Check(testkit.Rows("1 1", "1 1", "2 1", "2 1"))
	tk.MustQuery("select a, nth_value(a, 4) over() from t").
		Check(testkit.Rows("1 2", "1 2", "2 2", "2 2"))
	tk.MustQuery("select a, nth_value(a, 5) over() from t").
		Check(testkit.Rows("1 <nil>", "1 <nil>", "2 <nil>", "2 <nil>"))

	tk.MustQuery("select ntile(3) over() from t").
		Check(testkit.Rows("1", "1", "2", "3"))
	tk.MustQuery("select ntile(2) over() from t").
		Check(testkit.Rows("1", "1", "2", "2"))
	tk.MustQuery("select ntile(null) over() from t").
		Check(testkit.Rows("<nil>", "<nil>", "<nil>", "<nil>"))

	tk.MustQuery("select a, percent_rank() over() from t").
		Check(testkit.Rows("1 0", "1 0", "2 0", "2 0"))
	tk.MustQuery("select a, percent_rank() over(order by a) from t").
		Check(testkit.Rows("1 0", "1 0", "2 0.6666666666666666", "2 0.6666666666666666"))
	tk.MustQuery("select a, b, percent_rank() over(order by a, b) from t").
		Check(testkit.Rows("1 1 0", "1 2 0.3333333333333333", "2 1 0.6666666666666666", "2 2 1"))

	tk.MustQuery("select a, lead(a) over (), lag(a) over() from t").
		Check(testkit.Rows("1 1 <nil>", "1 2 1", "2 2 1", "2 <nil> 2"))
	tk.MustQuery("select a, lead(a, 0) over(), lag(a, 0) over() from t").
		Check(testkit.Rows("1 1 1", "1 1 1", "2 2 2", "2 2 2"))
	tk.MustQuery("select a, lead(a, 1, a) over(), lag(a, 1, a) over() from t").
		Check(testkit.Rows("1 1 1", "1 2 1", "2 2 1", "2 2 2"))
	tk.MustQuery("select a, lead(a, 1, 'lead') over(), lag(a, 1, 'lag') over() from t").
		Check(testkit.Rows("1 1 lag", "1 2 1", "2 2 1", "2 lead 2"))

	tk.MustQuery("SELECT CUME_DIST() OVER (ORDER BY null);").
		Check(testkit.Rows("1"))

	tk.MustQuery("select lead(a) over(partition by null) from t").Sort().Check(testkit.Rows("1", "2", "2", "<nil>"))

	tk.MustExec("drop table if exists issue10494")
	tk.MustExec("create table issue10494(a INT, b CHAR(1), c DATETIME, d BLOB)")
	tk.MustExec("insert into issue10494 VALUES (1,'x','2010-01-01','blob'), (2, 'y', '2011-01-01', ''), (3, 'y', '2012-01-01', ''), (4, 't', '2012-01-01', 'blob'), (5, null, '2013-01-01', null)")
	tk.MustQuery("SELECT a, b, c, SUM(a) OVER (RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) FROM issue10494 order by a;").Check(
		testkit.Rows(
			"1 x 2010-01-01 00:00:00 15",
			"2 y 2011-01-01 00:00:00 15",
			"3 y 2012-01-01 00:00:00 15",
			"4 t 2012-01-01 00:00:00 15",
			"5 <nil> 2013-01-01 00:00:00 15",
		),
	)

	tk.MustExec("drop table if exists td_dec")
	tk.MustExec("CREATE TABLE td_dec (id DECIMAL(10,2), sex CHAR(1));")
	tk.MustExec("insert into td_dec value (2.0, 'F'), (NULL, 'F'), (1.0, 'F')")
	tk.MustQuery("SELECT id, FIRST_VALUE(id) OVER w FROM td_dec WINDOW w AS (ORDER BY id);").Check(
		testkit.Rows("<nil> <nil>", "1.00 <nil>", "2.00 <nil>"),
	)

	tk.MustQuery("select sum(a) over w, sum(b) over w from t window w as (order by a)").
		Check(testkit.Rows("2 3", "2 3", "6 6", "6 6"))
	tk.MustQuery("select row_number() over w, sum(b) over w from t window w as (order by a)").
		Check(testkit.Rows("1 3", "2 3", "3 6", "4 6"))
	tk.MustQuery("select row_number() over w, sum(b) over w from t window w as (rows between 1 preceding and 1 following)").
		Check(testkit.Rows("1 3", "2 4", "3 5", "4 3"))

	tk.Session().GetSessionVars().MaxChunkSize = 1
	tk.MustQuery("select a, row_number() over (partition by a) from t").Sort().
		Check(testkit.Rows("1 1", "1 2", "2 1", "2 2"))
}

func TestWindowFunctionsDataReference(t *testing.T) {
	// see https://github.com/pingcap/tidb/issues/11614
	store := testkit.CreateMockStore(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t(a int, b int)")
	tk.MustExec("insert into t values (2,1),(2,2),(2,3)")

	tk.Session().GetSessionVars().MaxChunkSize = 2
	tk.MustQuery("select a, b, rank() over (partition by a order by b) from t").
		Check(testkit.Rows("2 1 1", "2 2 2", "2 3 3"))
	tk.MustQuery("select a, b, PERCENT_RANK() over (partition by a order by b) from t").
		Check(testkit.Rows("2 1 0", "2 2 0.5", "2 3 1"))
	tk.MustQuery("select a, b, CUME_DIST() over (partition by a order by b) from t").
		Check(testkit.Rows("2 1 0.3333333333333333", "2 2 0.6666666666666666", "2 3 1"))

	// see https://github.com/pingcap/tidb/issues/12415
	tk.MustQuery("select b, first_value(b) over (order by b RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) from t").
		Check(testkit.Rows("1 1", "2 1", "3 1"))
	tk.MustQuery("select b, first_value(b) over (order by b ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) from t").
		Check(testkit.Rows("1 1", "2 1", "3 1"))
}

func TestSlidingWindowFunctions(t *testing.T) {
	store := testkit.CreateMockStore(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test;")
	enablePipeLined := []string{"0", "1"}
	idTypes := []string{"FLOAT", "DOUBLE"}
	useHighPrecisions := []string{"ON", "OFF"}
	for _, enabled := range enablePipeLined {
		tk.MustExec(fmt.Sprintf("set @@tidb_enable_pipelined_window_function=%s;", enabled))
		for _, idType := range idTypes {
			for _, useHighPrecision := range useHighPrecisions {
				tk.MustExec("drop table if exists t;")
				tk.MustExec(fmt.Sprintf("CREATE TABLE t (id %s, sex CHAR(1));", idType))
				tk.MustExec(fmt.Sprintf("SET SESSION windowing_use_high_precision = %s;", useHighPrecision))
				baseTestSlidingWindowFunctions(tk)
			}
		}
	}
}

func baseTestSlidingWindowFunctions(tk *testkit.TestKit) {
	tk.MustExec("insert into t values (1,'M')")
	tk.MustExec("insert into t values (2,'F')")
	tk.MustExec("insert into t values (3,'F')")
	tk.MustExec("insert into t values (4,'F')")
	tk.MustExec("insert into t values (5,'M')")
	tk.MustExec("insert into t values (10,null)")
	tk.MustExec("insert into t values (11,null)")

	tk.MustExec("PREPARE p FROM 'SELECT sex, COUNT(id) OVER (ORDER BY id ROWS BETWEEN ? PRECEDING and ? PRECEDING) FROM t';")
	tk.MustExec("SET @p1= 1;")
	tk.MustExec("SET @p2= 2;")
	tk.MustQuery("EXECUTE p USING @p1, @p2;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))
	tk.MustQuery("EXECUTE p USING @p2, @p1;").
		Check(testkit.Rows("M 0", "F 1", "F 2", "F 2", "M 2", "<nil> 2", "<nil> 2"))
	tk.MustExec("DROP PREPARE p;")
	tk.MustExec("PREPARE p FROM 'SELECT sex, COUNT(id) OVER (ORDER BY id ROWS BETWEEN ? FOLLOWING and ? FOLLOWING) FROM t';")
	tk.MustExec("SET @p1= 1;")
	tk.MustExec("SET @p2= 2;")
	tk.MustQuery("EXECUTE p USING @p2, @p1;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))
	tk.MustQuery("EXECUTE p USING @p1, @p2;").
		Check(testkit.Rows("M 2", "F 2", "F 2", "F 2", "M 2", "<nil> 1", "<nil> 0"))
	tk.MustExec("DROP PREPARE p;")

	// COUNT ROWS
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2", "F 2", "F 2", "F 2", "M 2", "<nil> 1", "<nil> 0"))
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id ROWS BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id ROWS BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M 0", "F 1", "F 2", "F 2", "M 2", "<nil> 2", "<nil> 2"))
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING and 3 PRECEDING) FROM t;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))

	// COUNT RANGE
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id RANGE BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2", "F 2", "F 2", "F 1", "M 0", "<nil> 1", "<nil> 0"))
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id RANGE BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id RANGE BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M 0", "F 1", "F 2", "F 2", "M 2", "<nil> 0", "<nil> 1"))
	tk.MustQuery("SELECT sex, COUNT(id) OVER (ORDER BY id RANGE BETWEEN 1 PRECEDING and 3 PRECEDING) FROM t;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))

	// SUM ROWS
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 5", "F 7", "F 9", "F 15", "M 21", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id ROWS BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id ROWS BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 3", "F 5", "M 7", "<nil> 9", "<nil> 15"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING and 3 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 3", "F 6", "F 10", "F 15", "M 25", "<nil> 36", "<nil> 36"))

	// SUM RANGE
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id RANGE BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 5", "F 7", "F 9", "F 5", "M <nil>", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id RANGE BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id RANGE BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 3", "F 5", "M 7", "<nil> <nil>", "<nil> 10"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 6", "F 10", "F 14", "F 12", "M 9", "<nil> 21", "<nil> 21"))
	tk.MustQuery("SELECT sex, SUM(id) OVER (ORDER BY id DESC RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("<nil> 21", "<nil> 21", "M 12", "F 14", "F 10", "F 6", "M 3"))

	// AVG ROWS
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2.5", "F 3.5", "F 4.5", "F 7.5", "M 10.5", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id ROWS BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id ROWS BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 1.5", "F 2.5", "M 3.5", "<nil> 4.5", "<nil> 7.5"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING and 3 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 1.5", "F 2", "F 2.5", "F 3", "M 4.166666666666667", "<nil> 5.142857142857143", "<nil> 5.142857142857143"))

	// AVG RANGE
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id RANGE BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2.5", "F 3.5", "F 4.5", "F 5", "M <nil>", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id RANGE BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id RANGE BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 1.5", "F 2.5", "M 3.5", "<nil> <nil>", "<nil> 10"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2", "F 2.5", "F 3.5", "F 4", "M 4.5", "<nil> 10.5", "<nil> 10.5"))
	tk.MustQuery("SELECT sex, AVG(id) OVER (ORDER BY id DESC RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("<nil> 10.5", "<nil> 10.5", "M 4", "F 3.5", "F 2.5", "F 2", "M 1.5"))

	// BIT_XOR ROWS
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 1", "F 7", "F 1", "F 15", "M 1", "<nil> 11", "<nil> 0"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id ROWS BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id ROWS BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M 0", "F 1", "F 3", "F 1", "M 7", "<nil> 1", "<nil> 15"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING and 3 PRECEDING) FROM t;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 3", "F 0", "F 4", "F 1", "M 11", "<nil> 0", "<nil> 0"))

	// BIT_XOR RANGE
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id RANGE BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 1", "F 7", "F 1", "F 5", "M 0", "<nil> 11", "<nil> 0"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id RANGE BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 0", "F 0", "F 0", "F 0", "M 0", "<nil> 0", "<nil> 0"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id RANGE BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M 0", "F 1", "F 3", "F 1", "M 7", "<nil> 0", "<nil> 10"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 0", "F 4", "F 0", "F 2", "M 1", "<nil> 1", "<nil> 1"))
	tk.MustQuery("SELECT sex, BIT_XOR(id) OVER (ORDER BY id DESC RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("<nil> 1", "<nil> 1", "M 2", "F 0", "F 4", "F 0", "M 3"))

	// MIN ROWS
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2", "F 3", "F 4", "F 5", "M 10", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id ROWS BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id ROWS BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 1", "F 2", "M 3", "<nil> 4", "<nil> 5"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING and 3 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 1", "F 1", "F 1", "F 1", "M 1", "<nil> 1", "<nil> 1"))

	// MIN RANGE
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id RANGE BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2", "F 3", "F 4", "F 5", "M <nil>", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id RANGE BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id RANGE BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 1", "F 2", "M 3", "<nil> <nil>", "<nil> 10"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 1", "F 1", "F 2", "F 3", "M 4", "<nil> 10", "<nil> 10"))
	tk.MustQuery("SELECT sex, MIN(id) OVER (ORDER BY id DESC RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("<nil> 10", "<nil> 10", "M 3", "F 2", "F 1", "F 1", "M 1"))

	// MAX ROWS
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id ROWS BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 3", "F 4", "F 5", "F 10", "M 11", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id ROWS BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id ROWS BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 2", "F 3", "M 4", "<nil> 5", "<nil> 10"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING and 3 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 2", "F 3", "F 4", "F 5", "M 10", "<nil> 11", "<nil> 11"))

	// MAX RANGE
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id RANGE BETWEEN 1 FOLLOWING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 3", "F 4", "F 5", "F 5", "M <nil>", "<nil> 11", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id RANGE BETWEEN 3 FOLLOWING and 1 FOLLOWING) FROM t;").
		Check(testkit.Rows("M <nil>", "F <nil>", "F <nil>", "F <nil>", "M <nil>", "<nil> <nil>", "<nil> <nil>"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id RANGE BETWEEN 2 PRECEDING and 1 PRECEDING) FROM t;").
		Check(testkit.Rows("M <nil>", "F 1", "F 2", "F 3", "M 4", "<nil> <nil>", "<nil> 10"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("M 3", "F 4", "F 5", "F 5", "M 5", "<nil> 11", "<nil> 11"))
	tk.MustQuery("SELECT sex, MAX(id) OVER (ORDER BY id DESC RANGE BETWEEN 1 PRECEDING and 2 FOLLOWING) FROM t;").
		Check(testkit.Rows("<nil> 11", "<nil> 11", "M 5", "F 5", "F 4", "F 3", "M 2"))
}

func testReturnColumnNullableAttribute(tk *testkit.TestKit, funcName string, isNullable bool) {
	rs, err := tk.ExecWithContext(context.Background(), fmt.Sprintf("select %s over (partition by p order by o rows between 1 preceding and 1 following) as a from agg;", funcName))
	tk.RequireNoError(err, "testReturnColumnNullableAttribute get error")
	retField := rs.Fields()[0]
	if isNullable {
		tk.RequireNotEqual(mysql.NotNullFlag, (retField.Column.FieldType.GetFlag() & mysql.NotNullFlag), fmt.Sprintf("%s window function's return column should have nullable attribute", funcName))
	} else {
		tk.RequireEqual(mysql.NotNullFlag, (retField.Column.FieldType.GetFlag() & mysql.NotNullFlag), fmt.Sprintf("%s window function's return column should not have nullable attribute", funcName))
	}
	rs.Close()
}

func TestIssue45964And46050(t *testing.T) {
	store := testkit.CreateMockStore(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec(`drop table if exists agg;`)
	tk.MustExec("create table agg(p int not null, o int not null, v int not null);")
	tk.MustExec(`INSERT INTO agg VALUES (0, 0, 1), (1, 1, 2), (1, 2, 3), (1, 3, 4), (1, 4, 5), (2, 5, 6), (2, 6, 7);`)

	testReturnColumnNullableAttribute(tk, "first_value(v)", true)
	testReturnColumnNullableAttribute(tk, "last_value(v)", true)
	testReturnColumnNullableAttribute(tk, "nth_value(v, 2)", true)
	testReturnColumnNullableAttribute(tk, "lead(v)", true)
	testReturnColumnNullableAttribute(tk, "lead(v, 1)", true)
	testReturnColumnNullableAttribute(tk, "lead(1, 1, 1)", false)
	testReturnColumnNullableAttribute(tk, "lead(1, 1, null)", true)
	testReturnColumnNullableAttribute(tk, "lead(null, 1, 1)", true)
	testReturnColumnNullableAttribute(tk, "lead(v, 1, 1)", false)
	testReturnColumnNullableAttribute(tk, "lead(v, 1, null)", true)
	testReturnColumnNullableAttribute(tk, "lag(v)", true)
	testReturnColumnNullableAttribute(tk, "lag(v, 1)", true)
	testReturnColumnNullableAttribute(tk, "lag(1, 1, 1)", false)
	testReturnColumnNullableAttribute(tk, "lag(1, 1, null)", true)
	testReturnColumnNullableAttribute(tk, "lag(null, 1, 1)", true)
	testReturnColumnNullableAttribute(tk, "lag(v, 1, 1)", false)
	testReturnColumnNullableAttribute(tk, "lag(v, 1, null)", true)
	testReturnColumnNullableAttribute(tk, "ntile(2)", true)
	testReturnColumnNullableAttribute(tk, "sum(v)", true)
	testReturnColumnNullableAttribute(tk, "count(v)", false)
	testReturnColumnNullableAttribute(tk, "row_number()", false)
	testReturnColumnNullableAttribute(tk, "rank()", false)
	testReturnColumnNullableAttribute(tk, "dense_rank()", false)
	testReturnColumnNullableAttribute(tk, "cume_dist()", false)
	testReturnColumnNullableAttribute(tk, "percent_rank()", false)
}
