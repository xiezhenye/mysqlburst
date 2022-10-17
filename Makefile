pwd=$(shell pwd)

.PHONY : all clean

all : mysqlburst

mysqlburst:
	go build

clean:
	rm -rf mysqlburst
	

