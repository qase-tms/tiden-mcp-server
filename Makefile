.PHONY: build test install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/qase-tms/tiden-mcp-server/internal/version.Version=$(VERSION)

build:
	go build -ldflags '$(LDFLAGS)' -o bin/tiden-mcp-server ./cmd/tiden-mcp-server

test:
	go test ./...

install:
	go install -ldflags '$(LDFLAGS)' ./cmd/tiden-mcp-server
