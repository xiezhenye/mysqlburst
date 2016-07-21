# mysqlburst
A mysql pressure test tool that can make large quantity of connections

# usage

    mysqlburst --help
    Usage of /data/users/xiezhenye/mysqlburst/mysqlburst:
      -c int
        	concurrency (default 1000)
      -d string
        	dsn (default "mysql:@tcp(127.0.0.1:3306)/mysql?timeout=5s&readTimeout=5s&writeTimeout=5s")
      -i int
        	summery interval (sec)
      -q string
        	sql (default "select 1")
      -r int
        	rounds (default 100)
