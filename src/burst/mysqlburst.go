package burst

import (
	"database/sql/driver"
	"flag"
	"fmt"
	mysql "github.com/xiezhenye/go-sql-driver-mysql"
	"io"
	"math"
	"math/big"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	StageConn  byte = 0
	StageQuery byte = 1
	StageRead  byte = 2
	StageTotal byte = 3
	//
	StageMax byte = 4
)

type TestResult struct {
	//stage                  byte
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
	summery         [StageMax]SummeryResult
}

func (s *SummerySet) Init() {
	for i := byte(0); i < StageMax; i++ {
		s.summery[i].minTime = math.MaxInt64
	}
	s.startTime = time.Now()
}

func (s *SummerySet) Summery() {
	s.endTime = time.Now()
	for i := byte(0); i < StageMax; i++ {
		s.summery[i].Summery()
	}
}


type Config struct {
	procs   int
	rounds  int
	repeat  int
	qps     int
	dsn     string
	queries []string
	short   bool
}

type Routine struct {
	db     driver.Conn
	config Config
	//end         time.Time
}

type Test struct {
	routine     *Routine
	result [StageMax]TestResult

	beforeConn  time.Time
	afterConn   time.Time
	beforeQuery time.Time
	afterQuery  time.Time
	afterRead   time.Time

}

func (t *Test) testOnce() {
	//for i := byte(0); i < StageMax; i++ {
	//	t.result[i].ok = false
	//}
	config := t.routine.config
	beforeConn := time.Now()
	var afterConn time.Time
	if config.short {
		db, err := (mysql.MySQLDriver{}).Open(config.dsn)
		if err != nil {
			t.result[StageConn].err = err
			return
		}
		afterConn = time.Now()
		t.routine.db = db

		defer db.Close()
	} else {
		afterConn = beforeConn
	}
	t.result[StageConn].ok = true
	t.result[StageConn].time = afterConn.Sub(beforeConn)

	t.result[StageQuery].time = 0
	t.result[StageRead].time = 0
	for i := 0; i < config.repeat; i++ {
		for _, query := range config.queries {
			beforeQuery := time.Now()
			rows, err := t.routine.db.(driver.Queryer).Query(query, []driver.Value{})
			if err != nil {
				t.result[StageQuery].err = err
				t.result[StageQuery].ok = false
				return
			}

			afterQuery := time.Now()
			t.result[StageQuery].ok = true
			t.result[StageQuery].time += afterQuery.Sub(beforeQuery)

			err = rows.Close() // Close() will read all rows
			if err != nil && err != io.EOF {
				t.result[StageRead].err = err
				t.result[StageRead].ok = false
				return
			}
			afterRead := time.Now()
			t.result[StageRead].ok = true
			t.result[StageRead].time += afterRead.Sub(afterQuery)
		}
	}

	t.result[StageTotal].ok = true
	t.result[StageTotal].time = time.Now().Sub(beforeConn)
}

func (r *Routine) run(outChan chan<- [StageMax]TestResult) {
	rate := float64(r.config.qps) / float64(r.config.procs)
	var test, prevTest Test
	if !r.config.short {
		db, err := (mysql.MySQLDriver{}).Open(r.config.dsn)
		if err != nil {
			test.result[StageConn].err = err
			return
		}
		r.db = db
	}
	for i := 0; i < r.config.rounds; i++ {
		prevTest, test = test, Test{}
		if rate > 0 {
			d := time.Duration(rand.ExpFloat64() * float64(time.Second) / rate)
			if prevTest.result[StageTotal].time < d {
				time.Sleep(d - prevTest.result[StageTotal].time)
			}
		}
		test.testOnce()
		outChan <-test.result
	}
	if !r.config.short {
		r.db.Close()
	}
}

func collectInto(result *[StageMax]TestResult, ret *SummerySet) {
	var bigA  big.Int
	for i := byte(0); i < StageMax; i++ {
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

func summeryRoutine(inChan <-chan [StageMax]TestResult, outChan chan<- SummerySet, summeryIntervalSecond int) {
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

func (r *SummeryResult) Summery() {
	var bigA, big2 big.Int
	var bigR1, bigR2, big1N, big1N1 big.Rat
	big2.SetInt64(2)
	//  ∑(i-miu)2 = ∑(i2)-(∑i)2/n
	n := r.count - r.failCount
	if n > 1 {
		r.avgTime = (time.Duration)((int64)(r.totalTime) / n)

		big1N.SetInt64(n).Inv(&big1N)                         // 1/n
		big1N1.SetInt64(n-1).Inv(&big1N1)                     // 1/(n-1)
		bigA.SetInt64((int64)(r.totalTime)).Mul(&bigA, &bigA) // (∑i)2
		bigR1.SetInt(&bigA).Mul(&bigR1, &big1N)               // (∑i)2/n
		bigR2.SetInt(&r.totalSquareTime).Sub(&bigR2, &bigR1)
		s2, _ := bigR2.Mul(&bigR2, &big1N1).Float64()
		r.stddevTime = (time.Duration)((int64)(math.Sqrt(s2)))
	}
	if r.minTime == math.MaxInt64 {
		r.minTime = 0
	}
}

func msStr(t time.Duration) string {
	return fmt.Sprintf("%0.3f ms", float64(int64(t) / 1000) / 1000.0)
}

type NullLogger struct{}
func (*NullLogger) Print(v ...interface{}) {
}

type arrayFlags []string
func (a *arrayFlags) String() string {
	return fmt.Sprintf("%v", *a)
}
func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func badArg() {
	flag.Usage()
	os.Exit(1)
}

// mysqlburst -c 2000 -r 30 -d 'mha:M616VoUJBnYFi0L02Y24@tcp(10.200.180.54:3342)/x?timeout=5s&readTimeout=3s&writeTimeout=3s'
func Main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	//go func() {
	//      http.ListenAndServe("localhost:6060", nil)
	//}()
	//driver.ErrBadConn = errors.New(driver.ErrBadConn)
	config := Config {
		qps: math.MaxInt32,
		repeat: 1,
	}
	//procs := 0
	//rounds := 0
	//repeat := 1
	//dsn := ""
	driverLog := false
	var queries arrayFlags
	summeryIntervalSec := 0
	myCfg := mysql.Config{
		Net: "tcp",
		InterpolateParams: true,
		MaxAllowedPacket: 16777216,
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "  e.g. ./mysqlburst -c 100 -r 1000 -a 127.0.0.1:3306 -d mysql -u user -p pswd -q 'select * from user limit 1' -i 1")
		fmt.Fprintln(os.Stderr, "params:")
		flag.PrintDefaults()
	}

	flag.IntVar(&config.procs, "c", 100, "concurrency")
	flag.IntVar(&config.rounds, "r", 1000, "rounds")
	flag.IntVar(&config.repeat, "n", 1, "repeat queries in a connection")
	flag.IntVar(&config.qps, "qps", 0, "max qps. <= 0 means no limit")
	flag.StringVar(&myCfg.Addr, "a", "127.0.0.1:3306", "mysql server address")
	flag.StringVar(&myCfg.User, "u", "root", "user")
	flag.StringVar(&myCfg.Passwd, "p", "", "password")
	flag.StringVar(&myCfg.DBName, "d", "mysql", "database")
	flag.DurationVar(&myCfg.Timeout, "cto", 1 * time.Second, "connect timeout")
	flag.DurationVar(&myCfg.ReadTimeout, "rto", 5 * time.Second, "read timeout")
	flag.DurationVar(&myCfg.WriteTimeout, "wto", 5 * time.Second, "write timeout")
	flag.Var(&queries, "q", "queries")
	flag.BoolVar(&driverLog, "l", false, "enable driver log, will be written to stderr")
	flag.BoolVar(&config.short, "t", true, "use short connection, reconnect before each test")
	flag.IntVar(&summeryIntervalSec, "i", 0, "summery interval (sec)")
	flag.Parse()

	if !driverLog {
		mysql.SetLogger(&NullLogger{})
	}
	if len(queries) == 0 {
		badArg()
		return
	}
	config.dsn = myCfg.FormatDSN()
	config.queries = queries
	wg := sync.WaitGroup{}
	wg.Add(config.procs)
	resultChan := make(chan [StageMax]TestResult, config.procs * 8)
	summeryChan := make(chan SummerySet, 16)
	go func() {
		for i := 0; i < config.procs; i++ {
			go func() {
				(&Routine{ config: config }).run(resultChan)
				wg.Done()
			}()
		}
		wg.Wait()
		close(resultChan)
	}()
	go summeryRoutine(resultChan, summeryChan, summeryIntervalSec)

	titles := [StageMax]string{
		"connect", "query", "read", "total",
	}
	for set := range summeryChan {
		fmt.Printf("[ %s ] time: %s\n", set.endTime, msStr(set.endTime.Sub(set.startTime)))
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


