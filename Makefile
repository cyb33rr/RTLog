VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -ldflags "-s -w -X github.com/cyb33rr/rtlog/cmd.Version=$(VERSION)"
BINARY  = rtlog

.PHONY: build test clean release

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

clean:
	rm -f $(BINARY)
	rm -f rtlog-*

release:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .
