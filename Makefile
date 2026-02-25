
GO   ?= go
GOCI ?= golangci-lint
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date +%Y-%m-%d)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PKG_LDFLAGS := github.com/prometheus/common/version
LDFLAGS := -s -w -X ${PKG_LDFLAGS}.Version=${VERSION} -X ${PKG_LDFLAGS}.Revision=${COMMIT} -X ${PKG_LDFLAGS}.BuildDate=${DATE} -X ${PKG_LDFLAGS}.Branch=${BRANCH}

export LDFLAGS
export DATE

.PHONY: build
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/perses-mcp-server ./cmd/permcp

.PHONY: generate-goreleaser
generate-goreleaser:
	$(GO) run ./scripts/generate-goreleaser/generate-goreleaser.go

## Cross build binaries for all platforms (Use "make build" in development)
.PHONY: cross-build
cross-build: generate-goreleaser ## Cross build binaries for all platforms (Use "make build" in development)
	goreleaser release --snapshot --clean

.PHONY: cross-release
cross-release: generate-goreleaser
	goreleaser release --clean

.PHONY: checkstyle
checkstyle:
	@echo ">> checking Go code style"
	$(GOCI) run --timeout 5m		

.PHONY: checkformat
checkformat:
	@echo ">> checking go code format"
	! gofmt -d $$(find . -name '*.go' -print) | grep '^'

.PHONY: checkunused
checkunused:
	@echo ">> running check for unused/missing packages in go.mod"
	go mod tidy
	@git diff --exit-code -- go.sum go.mod
	
.PHONY: test
test:
	@echo ">> running all tests"
	$(GO) test -count=1 -v ./...	
