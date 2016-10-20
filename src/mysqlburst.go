package main

import (
	mysql "github.com/xiezhenye/go-sql-driver-mysql"
	"database/sql/driver"
	"fmt"
	"time"
	"sync"
	"runtime"
	"flag"
	"math/big"
	"math"
	"io"
	"os"
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
	count           int64
	failCount       int64
	totalTime       time.Duration
	totalSquareTime big.Int
	maxTime         time.Duration
	minTime         time.Duration
	avgTime         time.Duration
	stddevTime      time.Duration
	lastError       error
}

type SummerySet struct {
	startTime       time.Time
	endTime         time.Time
	summery         [STAGE_MAX]SummeryResult
}

func (self *SummerySet) Init() {
	for i := byte(0); i < STAGE_MAX; i++ {
		self.summery[i].minTime = math.MaxInt64
	}
	self.startTime = time.Now()
}

func (self *SummerySet) Summery() {
	self.endTime = time.Now()
	for i := byte(0); i < STAGE_MAX; i++ {
		self.summery[i].Summery()
	}
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

func collectInto(result *[STAGE_MAX]TestResult, ret *SummerySet) {
	var bigA  big.Int
	for i := byte(0); i < STAGE_MAX; i++ {
		summery := &(ret.summery[i])
		summery.count++
		if result[i].ok {
			if result[i].time > summery.maxTime {
				summery.maxTime = result[i].time
			}
			if result[i].time < summery.minTime {
				summery.minTime = result[i].time
			}
			summery.totalTime+= result[i].time
			bigA.SetInt64((int64)(result[i].time)).Mul(&bigA, &bigA)
			summery.totalSquareTime.Add(&(summery.totalSquareTime), &bigA)
		} else {
			summery.failCount++
			if result[i].err != nil {
				summery.lastError = result[i].err
			}
		}
	}
}

func summeryRoutine(inChan <-chan [STAGE_MAX]TestResult, outChan chan<- SummerySet, summeryIntervalSecond int) {
	var ret   SummerySet
	var ticker *time.Ticker
	ret.Init()

	if summeryIntervalSecond > 0 {
		summeryInterval := time.Second * time.Duration(summeryIntervalSecond)
		ticker = time.NewTicker(summeryInterval)
		loop:
		for {
			select {
			case result, ok := <-inChan:
				if !ok {
					break loop;
				}
				collectInto(&result, &ret)
			case <-ticker.C:
				ret.Summery()
				outChan<-ret
				ret = SummerySet{}
				ret.Init()
			}
		}
	} else {
		for result := range inChan {
			collectInto(&result, &ret)
		}
	}
	ret.Summery()
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

func badArg() {
	flag.Usage()
	os.Exit(1)
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
	driverLog := false
	var queries arrayFlags
	summeryIntervalSec := 0
	myCfg := mysql.Config{
		Net: "tcp",
		InterpolateParams: true,
		MaxPacketAllowed: 16777216,
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s: [options]\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "  e.g. ./mysqlburst -c 100 -r 1000 -a 127.0.0.1:3306 -d mysql -u user -p pswd -q 'select * from user limit 1' -i 1")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
	}
	// user_test:test@tcp(10.215.20.22:4006)/test?timeout=100ms&readTimeout=2s&writeTimeout=2s&interpolateParams=true&maxPacketAllowed=16777216
	flag.IntVar(&procs, "c", 100, "concurrency")
	flag.IntVar(&rounds, "r", 1000, "rounds")
	//flag.StringVar(&dsn, "d", "mysql:@tcp(127.0.0.1:3306)/mysql?timeout=5s&readTimeout=5s&writeTimeout=5s", "dsn")
	flag.StringVar(&myCfg.Addr, "a", "127.0.0.1:3306", "mysql server address")
	flag.StringVar(&myCfg.User, "u", "root", "user")
	flag.StringVar(&myCfg.Passwd, "p", "", "password")
	flag.StringVar(&myCfg.DBName, "d", "mysql", "database")
	flag.DurationVar(&myCfg.Timeout, "cto", 1 * time.Second, "connect timeout")
	flag.DurationVar(&myCfg.ReadTimeout, "rto", 5 * time.Second, "read timeout")
	flag.DurationVar(&myCfg.WriteTimeout, "wto", 5 * time.Second, "write timeout")
	flag.Var(&queries, "q", "queries")
	flag.BoolVar(&driverLog, "l", false, "enable driver log, will be written to stderr")
	flag.IntVar(&summeryIntervalSec, "i", 0, "summery interval (sec)")
	flag.Parse()

	if !driverLog {
		mysql.SetLogger(&NullLogger{})
	}
	if len(queries) == 0 {
		badArg()
		return
	}
	dsn = myCfg.FormatDSN()
	wg := sync.WaitGroup{}
	wg.Add(procs)
	resultChan := make(chan [STAGE_MAX]TestResult, procs * 8)
	summeryChan := make(chan SummerySet, 16)
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

	titles := [STAGE_MAX]string{
		"connect", "query", "read", "total",
	}
	for set := range summeryChan {
		fmt.Printf("time: %s\n", msStr(set.endTime.Sub(set.startTime)))
		for i, title := range titles {
			summery := set.summery[i]
			errStr := "-"
			lastErr := summery.lastError
			if lastErr != nil {
				errStr = lastErr.Error()
			}
			fmt.Printf("%-8s count: %-10d failed: %-8d avg: %-14s min: %-14s max: %-14s stddev: %-14s err: %s\n",
				title, summery.count, summery.failCount,
				msStr(summery.avgTime), msStr(summery.minTime), msStr(summery.maxTime), msStr(summery.stddevTime),
				errStr)
		}
		fmt.Println()
	}
}


