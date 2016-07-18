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
)

type TestResult struct {
	connOk bool
	queryOk bool
	connTime time.Duration
	queryTime time.Duration
}

type SummeryResult struct {
	count                 int64
	connFailCount         int64
	queryFailCount        int64

	totalConnTime         time.Duration
	totalQueryTime        time.Duration
	totalSquareConnTime   big.Int
	totalSquareQueryTime  big.Int

	maxConnTime           time.Duration
	minConnTime           time.Duration
	avgConnTime           time.Duration
	stddevConnTime        time.Duration

	maxQueryTime          time.Duration
	minQueryTime          time.Duration
	avgQueryTime          time.Duration
	stddevQueryTime       time.Duration
}

func testOnce(dsn, query string) TestResult {
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

	stmt, err := db.Prepare(query)
	if err != nil {
		result.queryOk = false
		return result
	}
	defer stmt.Close()

	rows, err := stmt.Query([]driver.Value{})
	if err != nil {
		//fmt.Println(err.Error())
		result.queryOk = false
		return result
	}
	result.queryOk = true
	defer rows.Close()
	afterQuery := time.Now()
	result.queryTime = afterQuery.Sub(afterConn)
	return result
}

func testRoutine(dsn, query string, n int, outChan chan<- TestResult) {
	for i := 0; i < n; i++ {
		outChan <- testOnce(dsn, query)
	}
}

func summeryRoutine(inChan <-chan TestResult) SummeryResult {
	var ret   SummeryResult
	var bigA  big.Int
	ret.minConnTime = math.MaxInt64
	ret.minQueryTime = math.MaxInt64
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
	}
	ret.Summery()
	return ret
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

	flag.IntVar(&procs, "c", 1000, "concurrency")
	flag.IntVar(&rounds, "r", 100, "rounds")
	flag.StringVar(&dsn, "d", "mysql:@tcp(127.0.0.1:3306)/mysql?timeout=5s&readTimeout=5s&writeTimeout=5s", "dsn")
	flag.StringVar(&query, "q", "select 1", "sql")
	flag.Parse()


	mysql.SetLogger(&NullLogger{})

	wg := sync.WaitGroup{}
	wg.Add(procs)
	resultChan := make(chan TestResult, 5000)
	testBegin := time.Now()
	go func() {
		for i := 0; i < procs; i++ {
			go func() {
				testRoutine(dsn, query, rounds, resultChan)
				wg.Done()
			}()
		}
		wg.Wait()
		close(resultChan)
	}()
	summery := summeryRoutine(resultChan)
	testEnd := time.Now()
	fmt.Printf("total tests: %d\n", summery.count);
	fmt.Printf("failed connections: %d\n", summery.connFailCount);
	fmt.Printf("failed queries: %d\n", summery.queryFailCount);
	fmt.Printf("test time: %s\n", testEnd.Sub(testBegin).String())

	fmt.Println("connect time")
	fmt.Printf("avg: %s\n", summery.avgConnTime.String())
	fmt.Printf("min: %s\n", summery.minConnTime.String())
	fmt.Printf("max: %s\n", summery.maxConnTime.String())
	fmt.Printf("stddev: %s\n", summery.stddevConnTime.String())

	fmt.Println("query time")
	fmt.Printf("avg: %s\n", summery.avgQueryTime.String())
	fmt.Printf("min: %s\n", summery.minQueryTime.String())
	fmt.Printf("max: %s\n", summery.maxQueryTime.String())
	fmt.Printf("stddev: %s\n", summery.stddevQueryTime.String())
}

