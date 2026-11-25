
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

MCP_APPS_DIR := ui/mcp-apps
MCP_APPS_VIEW := $(MCP_APPS_DIR)/dist/panels.html
MCP_APPS_SOURCES := $(shell find $(MCP_APPS_DIR)/app -type f) $(MCP_APPS_DIR)/package.json \
	$(MCP_APPS_DIR)/package-lock.json $(MCP_APPS_DIR)/tsconfig.json $(MCP_APPS_DIR)/vite.config.ts

## Build the server, with the MCP Apps views when npm is installed (perses_render_panels needs them).
.PHONY: build
build:
	@if command -v npm >/dev/null 2>&1; then \
		$(MAKE) --no-print-directory $(MCP_APPS_VIEW); \
	elif [ -f $(MCP_APPS_VIEW) ]; then \
		echo ">> npm not found: embedding the MCP Apps views of a previous build, which may be outdated"; \
	else \
		echo ">> npm not found: building without the MCP Apps views (perses_render_panels will not be available)"; \
	fi
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/perses-mcp-server ./cmd/permcp

.PHONY: generate-goreleaser
generate-goreleaser:
	$(GO) run ./scripts/generate-goreleaser/generate-goreleaser.go

## Cross build binaries for all platforms (Use "make build" in development)
.PHONY: cross-build
cross-build: checkmcpapps generate-goreleaser ## Cross build binaries for all platforms (Use "make build" in development)
	goreleaser release --snapshot --clean

.PHONY: cross-release
cross-release: checkmcpapps generate-goreleaser
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

# Installs the dependencies of the views once, again only when package-lock.json changed: npm ci writes
# node_modules/.package-lock.json.
$(MCP_APPS_DIR)/node_modules/.package-lock.json: $(MCP_APPS_DIR)/package-lock.json
	cd $(MCP_APPS_DIR) && npm ci

## Build the MCP Apps views embedded in the server (ui/mcp-apps/dist, not committed).
.PHONY: build-mcp-apps
build-mcp-apps: $(MCP_APPS_DIR)/node_modules/.package-lock.json
	@echo ">> building the MCP Apps views"
	cd $(MCP_APPS_DIR) && npm run build

# Rebuilds the views only when their sources changed.
$(MCP_APPS_VIEW): $(MCP_APPS_SOURCES)
	@$(MAKE) --no-print-directory build-mcp-apps

.PHONY: test-mcp-apps
test-mcp-apps: $(MCP_APPS_DIR)/node_modules/.package-lock.json
	@echo ">> checking the MCP Apps views"
	cd $(MCP_APPS_DIR) && npm run typecheck && npm test

## Build the MCP Apps views and fail when the server doesn't embed them: the releases must have them.
.PHONY: checkmcpapps
checkmcpapps: build-mcp-apps
	PERSES_MCP_REQUIRE_VIEWS=true $(GO) test -count=1 ./ui/...
