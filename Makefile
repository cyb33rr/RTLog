VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -ldflags "-s -w -X github.com/cyb33rr/rtlog/cmd.Version=$(VERSION)"
BINARY  = rtlog

.PHONY: build test clean

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

clean:
	rm -f $(BINARY)
