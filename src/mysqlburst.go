package main

import (
	"github.com/go-sql-driver/mysql"
	"database/sql/driver"
	"fmt"
	"time"
	"sync"
	"runtime"
	"flag"
	"math/big"
	"math"
	"io"
)

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
	err                    error
	time                   time.Duration
}

type SummeryResult struct {
	count             int64
	failCount         int64
	totalTime         time.Duration
	totalSquareTime   big.Int
	maxTime           time.Duration
	minTime           time.Duration
	avgTime           time.Duration
	stddevTime        time.Duration
	lastError         error
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

func testOnce(dsn string, queries []string, result *[STAGE_MAX]TestResult) {
	for i := byte(0); i < STAGE_MAX; i++ {
		(*result)[i].ok = false
	}
	beforeConn := time.Now()
	db, err := (mysql.MySQLDriver{}).Open(dsn)
	if err != nil {
		(*result)[STAGE_CONN].err = err
		return
	}
	(*result)[STAGE_CONN].ok = true
	afterConn := time.Now()
	(*result)[STAGE_CONN].time = afterConn.Sub(beforeConn)
	defer db.Close()
	afterRead := afterConn
	(*result)[STAGE_QUERY].time = 0
	(*result)[STAGE_READ].time = 0
	for _, query := range queries {
		beforeQuery := time.Now()
		rows, err := db.(driver.Queryer).Query(query, []driver.Value{})
		if err != nil {
			(*result)[STAGE_QUERY].err = err
			(*result)[STAGE_QUERY].ok = false
			return
		}

		afterQuery := time.Now()
		(*result)[STAGE_QUERY].ok = true
		(*result)[STAGE_QUERY].time += afterQuery.Sub(beforeQuery)

		err = rows.Close() // Close() will read all rows
		if err != nil && err != io.EOF {
			(*result)[STAGE_READ].err = err
			(*result)[STAGE_READ].ok = false
			return
		}
		afterRead = time.Now()
		(*result)[STAGE_READ].ok = true
		(*result)[STAGE_READ].time += afterRead.Sub(afterQuery)
	}
	(*result)[STAGE_TOTAL].ok = true
	(*result)[STAGE_TOTAL].time = afterRead.Sub(beforeConn)
}

func testRoutine(dsn string, queries []string, n int, outChan chan<- [STAGE_MAX]TestResult) {
	var result [STAGE_MAX]TestResult
	result[STAGE_CONN].stage = STAGE_CONN
	result[STAGE_QUERY].stage = STAGE_QUERY
	result[STAGE_READ].stage = STAGE_READ
	result[STAGE_TOTAL].stage = STAGE_TOTAL

	for i := 0; i < n; i++ {
		testOnce(dsn, queries, &result)
		outChan <-result
	}
}


func summeryRoutine(inChan <-chan [STAGE_MAX]TestResult, outChan chan<- [STAGE_MAX]SummeryResult, summeryIntervalSecond int) {
	var ret   [STAGE_MAX]SummeryResult
	var bigA  big.Int
	var ticker *time.Ticker
	for i := byte(0); i < STAGE_MAX; i++ {
		ret[i].minTime = math.MaxInt64
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
				if result[i].err != nil {
					ret[i].lastError = result[i].err
				}
			}

		}
		if summeryIntervalSecond > 0 {
			select {
			case <-ticker.C:
				for i := byte(0); i < STAGE_MAX; i++ {
					ret[i].Summery()
				}
				outChan<-ret
				ret = [STAGE_MAX]SummeryResult{}
				for i := byte(0); i < STAGE_MAX; i++ {
					ret[i].minTime = math.MaxInt64
				}
			default:
				//
			}
		}
	}
	for i := byte(0); i < STAGE_MAX; i++ {
		ret[i].Summery()
	}
	outChan<-ret
	close(outChan)
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

func msStr(t time.Duration) string {
	return fmt.Sprintf("%0.3f ms", float64(int64(t) / 1000) / 1000.0)
}

type NullLogger struct{}
func (*NullLogger) Print(v ...interface{}) {
}

type arrayFlags []string
func (self *arrayFlags) String() string {
	return fmt.Sprintf("%v", *self)
}
func (self *arrayFlags) Set(value string) error {
	*self = append(*self, value)
	return nil
}

// mysqlburst -c 2000 -r 30 -d 'mha:M616VoUJBnYFi0L02Y24@tcp(10.200.180.54:3342)/x?timeout=5s&readTimeout=3s&writeTimeout=3s'
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	//go func() {
	//      http.ListenAndServe("localhost:6060", nil)
	//}()
	//driver.ErrBadConn = errors.New(driver.ErrBadConn)
	procs := 0
	rounds := 0
	dsn := ""
	//query := ""
	var queries arrayFlags
	summeryIntervalSec := 0
	flag.IntVar(&procs, "c", 1000, "concurrency")
	flag.IntVar(&rounds, "r", 100, "rounds")
	flag.StringVar(&dsn, "d", "mysql:@tcp(127.0.0.1:3306)/mysql?timeout=5s&readTimeout=5s&writeTimeout=5s", "dsn")
	//flag.StringVar(&query, "q", "select 1", "sql")
	flag.Var(&queries, "q", "queries")
	flag.IntVar(&summeryIntervalSec, "i", 0, "summery interval (sec)")
	flag.Parse()

	mysql.SetLogger(&NullLogger{})

	wg := sync.WaitGroup{}
	wg.Add(procs)
	resultChan := make(chan [STAGE_MAX]TestResult, 5000)
	summeryChan := make(chan [STAGE_MAX]SummeryResult, 10)
	go func() {
		for i := 0; i < procs; i++ {
			go func() {
				testRoutine(dsn, queries, rounds, resultChan)
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
		for i, title := range titles {
			errStr := "-"
			lastErr := summery[i].lastError
			if lastErr != nil {
				errStr = lastErr.Error()
			}
			fmt.Printf("%-8s count: %-10d failed: %-8d avg: %-14s min: %-14s max: %-14s stddev: %-14s err: %s\n",
				title, summery[i].count, summery[i].failCount,
				msStr(summery[i].avgTime), msStr(summery[i].minTime), msStr(summery[i].maxTime), msStr(summery[i].stddevTime),
				errStr)
		}
		fmt.Println()
	}
}


