
GO   ?= go
GOCI ?= golangci-lint

.PHONY: build
build:
	go build -o bin/

.PHONY: permcp
permcp:
	$(GO) build -o bin/permcp ./cmd/permcp


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