BINARY       := crafty
CMD_DIR      := ./cmd/crafty
BUILD_DIR    := bin
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT       := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE         := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS      := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GO           ?= go

.PHONY: all build run tidy fmt vet lint test cover clean \
        security govulncheck gosec release-snapshot help

all: build ## Default target — build the binary

build: ## Build the crafty binary into bin/
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

run: build ## Build and run
	$(BUILD_DIR)/$(BINARY)

tidy: ## go mod tidy
	$(GO) mod tidy

fmt: ## gofmt + goimports
	$(GO) fmt ./...
	@command -v goimports >/dev/null 2>&1 && goimports -w -local github.com/TyphoonMC/Crafty . || true

vet: ## go vet
	$(GO) vet ./...

lint: ## golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 || { echo "install: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

test: ## Run tests with race detector
	$(GO) test -race -v ./...

cover: ## Generate HTML coverage report
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

security: govulncheck gosec ## Run all security scans

govulncheck: ## govulncheck (requires: go install golang.org/x/vuln/cmd/govulncheck@latest)
	@command -v govulncheck >/dev/null 2>&1 || $(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

gosec: ## gosec (requires: go install github.com/securego/gosec/v2/cmd/gosec@latest)
	@command -v gosec >/dev/null 2>&1 || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec ./...

release-snapshot: ## Local release dry-run via goreleaser
	@command -v goreleaser >/dev/null 2>&1 || { echo "install: https://goreleaser.com/install/"; exit 1; }
	goreleaser release --snapshot --clean

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR) coverage.out coverage.html dist

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
