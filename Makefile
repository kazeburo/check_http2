VERSION=0.0.26
GITCOMMIT?=$(shell git describe --dirty --always)
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.commit=${GITCOMMIT}"

all: check_http2

.PHONY: check_http2 linux check lint

check_http2: *.go
	go build $(LDFLAGS) -o check_http2

linux: *.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o check_http2

check:
	go test -v ./...

lint:
	golangci-lint run ./...
