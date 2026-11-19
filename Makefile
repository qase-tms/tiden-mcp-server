.PHONY: build test install changelog-cut

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/qase-tms/tiden-mcp-server/internal/version.Version=$(VERSION)

build:
	go build -ldflags '$(LDFLAGS)' -o bin/tiden-mcp-server ./cmd/tiden-mcp-server

test:
	go test ./...

install:
	go install -ldflags '$(LDFLAGS)' ./cmd/tiden-mcp-server

# Release PRs only: fold docs/changelog/ entries into CHANGELOG.md.
changelog-cut:
	@test -n "$(RELEASE)" || { echo "usage: make changelog-cut RELEASE=X.Y.Z" >&2; exit 2; }
	go run ./tools/changelog-cut -version $(RELEASE)
