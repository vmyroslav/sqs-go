BUILD_DIR ?= out
VENDOR_DIR = vendor

GOLANGCI_LINT_VERSION ?= 1.55.1

GO ?= go
GOLANGCI_LINT ?= golangci-lint

all: clean
.PHONY: all

clean:
	@rm -f ./bin/*

.PHONY: test
test: test-unit

## Run unit tests
.PHONY: test-unit
test-unit:
	@echo ">> Unit test"
	@$(GO) test -gcflags=-l -coverprofile=unit.coverprofile -covermode=atomic -race ./...

lint:
	@echo ">> Linting"
	@$(GOLANGCI_LINT) run