
GO   ?= go
GOCI ?= golangci-lint

.PHONY: build
build:
	go build -o bin/

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
