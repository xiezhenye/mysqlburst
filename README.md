# mysqlburst
A mysql pressure test tool that can make large quantity of connections

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
      -r int
            rounds (default 1000)
      -rto duration
            read timeout (default 5s)
      -u string
            user (default "root")
      -wto duration
            write timeout (default 5s)
