package main

import (
	"github.com/go-sql-driver/mysql"
	"database/sql/driver"
	"fmt"
	"time"
	"sync"
	"runtime"
	"flag"
//	"database/sql"
//  _ "github.com/go-sql-driver/mysql"
//  _ "net/http/pprof"
//  "net/http"
	"math/big"
	"math"
	"os"
	"io"
)

/*
type TestResult struct {
	connOk bool
	queryOk bool
	connTime time.Duration
	queryTime time.Duration
}*/


const (
	STAGE_CONN  byte = 0
	STAGE_QUERY byte = 1
	STAGE_READ  byte = 2
	STAGE_TOTAL byte = 3
	//
	STAGE_MAX   byte = 4
)

type TestResult struct {
	stage                  byte
	ok                     bool
	time                   time.Duration
}

/*
type SummeryResult struct {
	count                 int64

	connFailCount         int64
	totalConnTime         time.Duration
	totalSquareConnTime   big.Int
	maxConnTime           time.Duration
	minConnTime           time.Duration
	avgConnTime           time.Duration
	stddevConnTime        time.Duration

	queryFailCount        int64
	totalQueryTime        time.Duration
	totalSquareQueryTime  big.Int
	maxQueryTime          time.Duration
	minQueryTime          time.Duration
	avgQueryTime          time.Duration
	stddevQueryTime       time.Duration
}*/

type SummeryResult struct {
	stage             byte
	count             int64

	failCount         int64
	totalTime         time.Duration
	totalSquareTime   big.Int
	maxTime           time.Duration
	minTime           time.Duration
	avgTime           time.Duration
	stddevTime        time.Duration
}

func getColumnCount(dsn, query string) (int, error) {
	db, err := (mysql.MySQLDriver{}).Open(dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	rows, err := db.(driver.Queryer).Query(query, []driver.Value{})
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	return len(rows.Columns()), nil
}

func testOnce(dsn, query string, row []driver.Value, result [STAGE_MAX]TestResult) {
	result[STAGE_TOTAL].ok = false
	beforeConn := time.Now()
	db, err := (mysql.MySQLDriver{}).Open(dsn)
	if err != nil {
		//fmt.Println(err.Error())
		result[STAGE_CONN].ok = false
		return
	}
	result[STAGE_CONN].ok = true
	afterConn := time.Now()
	result[STAGE_CONN].time = afterConn.Sub(beforeConn)
	defer db.Close()

	rows, err := db.(driver.Queryer).Query(query, []driver.Value{})
	if err != nil {
		//fmt.Println(err.Error())
		result[STAGE_QUERY].ok = false
		return
	}
	afterQuery := time.Now()
	result[STAGE_QUERY].ok = true
	result[STAGE_QUERY].time = afterQuery.Sub(afterConn)
	defer rows.Close()
	for {
		err = rows.Next(row)
		if err != nil {
			break
		}
	}
	if err != io.EOF {
		result[STAGE_QUERY].ok = false
	} else {
		afterRead := time.Now()
		result[STAGE_READ].ok = true
		result[STAGE_TOTAL].ok = true
		result[STAGE_READ].time = afterRead.Sub(afterQuery)
		result[STAGE_TOTAL].time = afterRead.Sub(beforeConn)
	}
}

/*
func testOnce(dsn, query string, row []driver.Value) TestResult {
	result := TestResult{}
	beforeConn := time.Now()
	db, err := (mysql.MySQLDriver{}).Open(dsn)
	if err != nil {
		//fmt.Println(err.Error())
		result.connOk = false
		return result
	}
	result.connOk = true
	afterConn := time.Now()
	result.connTime = afterConn.Sub(beforeConn)
	defer db.Close()

	rows, err := db.(driver.Queryer).Query(query, []driver.Value{})
	if err != nil {
		//fmt.Println(err.Error())
		result.queryOk = false
		return result
	}
	afterQuery := time.Now()
	result.queryTime = afterQuery.Sub(afterConn)
	result.queryOk = true
	defer rows.Close()
	for {
		err = rows.Next(row)
		if err != nil {
			break
		}
	}
	// result.queryTime = afterQuery.Sub(afterConn)
	return result
}*/

func testRoutine(dsn, query string, n int, colNum int, outChan chan<- [STAGE_MAX]TestResult) {
	var result [STAGE_MAX]TestResult
	result[STAGE_CONN].stage = STAGE_CONN
	result[STAGE_QUERY].stage = STAGE_QUERY
	result[STAGE_READ].stage = STAGE_READ
	result[STAGE_TOTAL].stage = STAGE_TOTAL

	row := make([]driver.Value, colNum)
	for i := 0; i < n; i++ {
		testOnce(dsn, query, row, result)
		outChan <-result
	}
}


func summeryRoutine(inChan <-chan [STAGE_MAX]TestResult, outChan chan<- [STAGE_MAX]SummeryResult, summeryIntervalSecond int) {
	var ret   [STAGE_MAX]SummeryResult
	var bigA  big.Int
	var ticker *time.Ticker
	for i := byte(0); i < STAGE_MAX; i++ {
		ret[i].minTime = math.MaxInt64
		ret[i].stage = (byte)(i)
	}

	if summeryIntervalSecond > 0 {
		summeryInterval := time.Second * time.Duration(summeryIntervalSecond)
		ticker = time.NewTicker(summeryInterval)
	}
	for result := range inChan {
		for i := byte(0); i < STAGE_MAX; i++ {
			ret[i].count++
			if result[i].ok {
				if result[i].time > ret[i].maxTime {
					ret[i].maxTime = result[i].time
				}
				if result[i].time < ret[i].minTime {
					ret[i].minTime = result[i].time
				}
				ret[i].totalTime+= result[i].time
				bigA.SetInt64((int64)(result[i].time)).Mul(&bigA, &bigA)
				ret[i].totalSquareTime.Add(&ret[i].totalSquareTime, &bigA)
			} else {
				ret[i].failCount++
			}

			if summeryIntervalSecond > 0 {
				if _, ok := <-ticker.C; ok {
					ret[i].Summery()
					outChan<-ret
					ret = [STAGE_MAX]SummeryResult{}
				}
			}
		}
	}
	for i := byte(0); i < STAGE_MAX; i++ {
		ret[i].Summery()
	}
	outChan<-ret
	return
}

func (self *SummeryResult) Summery() {
	var bigA, big2 big.Int
	var bigR1, bigR2, big1N, big1N1 big.Rat
	big2.SetInt64(2)
	//  ∑(i-miu)2 = ∑(i2)-(∑i)2/n
	n := self.count - self.failCount
	if n > 1 {
		self.avgTime = (time.Duration)((int64)(self.totalTime) / n)

		big1N.SetInt64(n).Inv(&big1N) // 1/n
		big1N1.SetInt64(n-1).Inv(&big1N1) // 1/(n-1)
		bigA.SetInt64((int64)(self.totalTime)).Mul(&bigA, &bigA) // (∑i)2
		bigR1.SetInt(&bigA).Mul(&bigR1, &big1N) // (∑i)2/n
		bigR2.SetInt(&self.totalSquareTime).Sub(&bigR2, &bigR1)
		s2, _ := bigR2.Mul(&bigR2, &big1N1).Float64()
		self.stddevTime = (time.Duration)((int64)(math.Sqrt(s2)))
	}
	if self.minTime == math.MaxInt64 {
		self.minTime = 0
	}
}

/*
func summeryRoutine(inChan <-chan TestResult, outChan chan<- SummeryResult, summeryIntervalSecond int) {
	var ret   SummeryResult
	var bigA  big.Int
	ret.minConnTime = math.MaxInt64
	ret.minQueryTime = math.MaxInt64
	var ticker *time.Ticker
	if summeryIntervalSecond > 0 {
		summeryInterval := time.Second * time.Duration(summeryIntervalSecond)
		ticker = time.NewTicker(summeryInterval)
	}
	for result := range inChan {
		ret.count++
		if result.connOk {
			if result.connTime > ret.maxConnTime {
				ret.maxConnTime = result.connTime
			}
			if result.connTime < ret.minConnTime {
				ret.minConnTime = result.connTime
			}
			ret.totalConnTime+= result.connTime
			bigA.SetInt64((int64)(result.connTime)).Mul(&bigA, &bigA)
			ret.totalSquareConnTime.Add(&ret.totalSquareConnTime, &bigA)
		} else {
			ret.connFailCount++
		}
		if result.queryOk {
			if result.queryTime > ret.maxQueryTime {
				ret.maxQueryTime = result.queryTime
			}
			if result.queryTime < ret.minQueryTime {
				ret.minQueryTime = result.queryTime
			}
			ret.totalQueryTime+= result.queryTime
			bigA.SetInt64((int64)(result.queryTime)).Mul(&bigA, &bigA)
			ret.totalSquareQueryTime.Add(&ret.totalSquareQueryTime, &bigA)
		} else {
			ret.queryFailCount++
		}
		if summeryIntervalSecond > 0 {
			if _, ok := <-ticker.C; ok {
				ret.Summery()
				outChan<-ret
				ret = SummeryResult{}
			}
		}
	}
	ret.Summery()
	outChan<-ret
	return
}

func (self *SummeryResult) Summery() {
	var bigA, big2 big.Int
	var bigR1, bigR2, big1N, big1N1 big.Rat
	big2.SetInt64(2)
	//  ∑(i-miu)2 = ∑(i2)-(∑i)2/n
	n := self.count - self.connFailCount
	if n > 1 {
		self.avgConnTime = (time.Duration)((int64)(self.totalConnTime) / n)

		big1N.SetInt64(n).Inv(&big1N) // 1/n
		big1N1.SetInt64(n-1).Inv(&big1N1) // 1/(n-1)
		bigA.SetInt64((int64)(self.totalConnTime)).Mul(&bigA, &bigA) // (∑i)2
		bigR1.SetInt(&bigA).Mul(&bigR1, &big1N) // (∑i)2/n
		bigR2.SetInt(&self.totalSquareConnTime).Sub(&bigR2, &bigR1)
		s2, _ := bigR2.Mul(&bigR2, &big1N1).Float64()
		self.stddevConnTime = (time.Duration)((int64)(math.Sqrt(s2)))
	}

	n = self.count - self.queryFailCount
	if n > 1 {
		self.avgQueryTime = (time.Duration)((int64)(self.totalQueryTime) / n)

		big1N.SetInt64(n).Inv(&big1N) // 1/n
		big1N1.SetInt64(n-1).Inv(&big1N1) // 1/(n-1)
		bigA.SetInt64((int64)(self.totalQueryTime)).Mul(&bigA, &bigA) // (∑i)2
		bigR1.SetInt(&bigA).Mul(&bigR1, &big1N) // (∑i)2/n
		bigR2.SetInt(&self.totalSquareQueryTime).Sub(&bigR2, &bigR1)
		s2, _ := bigR2.Mul(&bigR2, &big1N1).Float64()
		self.stddevQueryTime = (time.Duration)((int64)(math.Sqrt(s2)))
	}

	if self.minConnTime == math.MaxInt64 {
		self.minConnTime = 0
	}
	if self.minQueryTime == math.MaxInt64 {
		self.minQueryTime = 0
	}
}*/

func msStr(t time.Duration) string {
	return fmt.Sprintf("%0.3f ms", float64(int64(t) / 1000) / 1000.0)
}

type NullLogger struct{}
func (*NullLogger) Print(v ...interface{}) {
}

// mysqlburst -c 2000 -r 30 -d 'mha:M616VoUJBnYFi0L02Y24@tcp(10.200.180.54:3342)/x?timeout=5s&readTimeout=3s&writeTimeout=3s'
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	//go func() {
	//      http.ListenAndServe("localhost:6060", nil)
	//}()

	procs := 0
	rounds := 0
	dsn := ""
	query := ""
	summeryIntervalSec := 0
	flag.IntVar(&procs, "c", 1000, "concurrency")
	flag.IntVar(&rounds, "r", 100, "rounds")
	flag.StringVar(&dsn, "d", "mysql:@tcp(127.0.0.1:3306)/mysql?timeout=5s&readTimeout=5s&writeTimeout=5s", "dsn")
	flag.StringVar(&query, "q", "select 1", "sql")
	flag.IntVar(&summeryIntervalSec, "i", 0, "summery interval (sec)")
	flag.Parse()

	mysql.SetLogger(&NullLogger{})

	colNum, err := getColumnCount(dsn, query)
	if err != nil {
		fmt.Printf("init failed: %s", err)
		os.Exit(2)
	}
	wg := sync.WaitGroup{}
	wg.Add(procs)
	resultChan := make(chan [STAGE_MAX]TestResult, 5000)
	summeryChan := make(chan [STAGE_MAX]SummeryResult, 10)

	go func() {
		for i := 0; i < procs; i++ {
			go func() {
				testRoutine(dsn, query, rounds, colNum, resultChan)
				wg.Done()
			}()
		}
		wg.Wait()
		close(resultChan)
	}()
	go summeryRoutine(resultChan, summeryChan, summeryIntervalSec)

	testBegin := time.Now()
	titles := [STAGE_MAX]string{
		"connect", "query", "read", "total",
	}
	for summery := range summeryChan {
		testEnd := time.Now()
		duration:= testEnd.Sub(testBegin)
		testBegin = testEnd

		fmt.Printf("time: %s\n", duration.String())
		//fmt.Printf("tests: %d\n", summery.count);
		for i, title := range titles {
			fmt.Println(title)
			fmt.Printf("tests: %d\n", summery[i].count);
			fmt.Printf("failed: %d\n", summery[i].failCount);
			fmt.Printf("avg time: %s\n", msStr(summery[i].avgTime))
			fmt.Printf("min time: %s\n", msStr(summery[i].minTime))
			fmt.Printf("max time: %s\n", msStr(summery[i].maxTime))
			fmt.Printf("stddev time: %s\n", msStr(summery[i].stddevTime))
		}
		/*
		fmt.Println("connect time")
		fmt.Printf("failed: %d\n", summery.connFailCount);
		fmt.Printf("avg: %s\n", msStr(summery.avgConnTime))
		fmt.Printf("min: %s\n", msStr(summery.minConnTime))
		fmt.Printf("max: %s\n", msStr(summery.maxConnTime))
		fmt.Printf("stddev: %s\n", msStr(summery.stddevConnTime))

		fmt.Println()
		fmt.Println("query time")
		fmt.Printf("failed: %d\n", summery.queryFailCount);
		fmt.Printf("avg: %s\n", msStr(summery.avgQueryTime))
		fmt.Printf("min: %s\n", msStr(summery.minQueryTime))
		fmt.Printf("max: %s\n", msStr(summery.maxQueryTime))
		fmt.Printf("stddev: %s\n", msStr(summery.stddevQueryTime))
		fmt.Println()
		*/
	}

}

