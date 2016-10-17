pwd=$(shell pwd)

.PHONY : all clean driver

all : mysqlburst

driver:
	GOPATH=$(pwd) go get github.com/go-sql-driver/mysql

mysqlburst: driver
	GOPATH=$(pwd) go build src/mysqlburst.go

clean:
	rm -rf mysqlburst pkg

