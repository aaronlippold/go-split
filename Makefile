.PHONY: all build test lint vet fmt check clean install

# Binary name
BINARY := go-split
# Build directory
BUILD_DIR := bin
# Version (from git tag or dev)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
# Build flags
LDFLAGS := -ldflags "-s -w -X github.com/aaronlippold/go-split/internal/cli.Version=$(VERSION)"

all: check build

## Build

build: ## Build the binary
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/go-split

install: ## Install to GOPATH/bin
	go install $(LDFLAGS) ./cmd/go-split

## Testing

test: ## Run tests
	go test -v -race ./...

test-coverage: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## Linting

lint: ## Run golangci-lint
	golangci-lint run --timeout=5m

vet: ## Run go vet
	go vet ./...

fmt: ## Check formatting
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

fmt-fix: ## Fix formatting
	gofmt -w .

## All checks

check: fmt vet lint test ## Run all checks

## Cleanup

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## Dependencies

deps: ## Download dependencies
	go mod download

tidy: ## Tidy go.mod
	go mod tidy

## Release (local)

snapshot: ## Build snapshot release (no publish)
	goreleaser release --snapshot --clean

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
