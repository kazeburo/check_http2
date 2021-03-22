VERSION=0.0.2
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} "

all: check_http2

.PHONY: check_http2

check_http2: main.go
	go build $(LDFLAGS) -o check_http2 main.go

linux: main.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o check_http2 main.go

check:
	go test ./...

fmt:
	go fmt ./...

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin main
