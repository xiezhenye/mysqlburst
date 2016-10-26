# mysqlburst
A mysql pressure test tool that makes large quantity of short connections

# usage
      
e.g. `./mysqlburst -c 100 -r 1000 -a 127.0.0.1:3306 -d mysql -u user -p pswd -q 'select * from user limit 1' -i 1`
      
      params:
      -a string
            mysql server address (default "127.0.0.1:3306")
      -c int
            concurrency (default 100)
      -cto duration
            connect timeout (default 1s)
      -d string
            database (default "mysql")
      -i int
            summery interval (sec)
      -l    enable driver log, will be written to stderr
      -p string
            password
      -q value
            queries
      -qps int
            max qps. <= 0 means no limit
      -r int
            rounds (default 1000)
      -rto duration
            read timeout (default 5s)
      -u string
            user (default "root")
      -wto duration
            write timeout (default 5s)


output may likes this:

      time: 999.917 ms
      connect  count: 27124      failed: 0        avg: 7.298 ms       min: 4.418 ms       max: 159.979 ms     stddev: 12.091 ms      err: -
      query    count: 27124      failed: 0        avg: 0.481 ms       min: 0.174 ms       max: 147.570 ms     stddev: 3.358 ms       err: -
      read     count: 27124      failed: 0        avg: 0.000 ms       min: 0.000 ms       max: 0.037 ms       stddev: 0.000 ms       err: -
      total    count: 27124      failed: 0        avg: 7.781 ms       min: 4.729 ms       max: 160.729 ms     stddev: 12.547 ms      err: -

      time: 999.794 ms
      connect  count: 29090      failed: 0        avg: 6.490 ms       min: 4.762 ms       max: 17.331 ms      stddev: 0.691 ms       err: -
      query    count: 29090      failed: 0        avg: 0.356 ms       min: 0.173 ms       max: 10.547 ms      stddev: 0.239 ms       err: -
      read     count: 29090      failed: 0        avg: 0.000 ms       min: 0.000 ms       max: 0.024 ms       stddev: 0.000 ms       err: -
      total    count: 29090      failed: 0        avg: 6.847 ms       min: 5.022 ms       max: 17.861 ms      stddev: 0.731 ms       err: -

      time: 1000.037 ms
      connect  count: 29450      failed: 0        avg: 6.390 ms       min: 4.798 ms       max: 18.382 ms      stddev: 0.689 ms       err: -
      query    count: 29450      failed: 0        avg: 0.366 ms       min: 0.173 ms       max: 11.494 ms      stddev: 0.318 ms       err: -
      read     count: 29450      failed: 0        avg: 0.000 ms       min: 0.000 ms       max: 0.391 ms       stddev: 0.002 ms       err: -
      total    count: 29450      failed: 0        avg: 6.757 ms       min: 5.078 ms       max: 18.752 ms      stddev: 0.769 ms       err: -


param `-q` can be applied more than one times to send multi queries in a connection.

