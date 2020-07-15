pwd=$(shell pwd)

.PHONY : all clean driver

all : mysqlburst

driver: src/vendor/github.com/xiezhenye/go-sql-driver-mysql

src/vendor/github.com/xiezhenye/go-sql-driver-mysql:
	mkdir -p src/vendor
	GOPATH=$(pwd) go get github.com/xiezhenye/go-sql-driver-mysql
	mv src/github.com src/vendor

mysqlburst: driver mysqlburst.go src/burst/*.go
	GOPATH=$(pwd) go build mysqlburst.go

clean:
	rm -rf mysqlburst pkg
	

