# Talia Makefile
# Comprehensive build automation for the Talia domain checker

# Variables
# TODO(sustanza): Ensure `$(MAIN_PATH)` exists by adding `cmd/talia/main.go`
# that calls the library `RunCLI` and exits with its return code (AGENTS.md structure).
BINARY_NAME := talia
MAIN_PATH := ./cmd/talia
GO := go
GOLANGCI_LINT := golangci-lint
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html
BUILD_DIR := ./dist
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Go build flags
GOFLAGS := -v
TEST_FLAGS := -race -cover
BENCH_FLAGS := -bench=. -benchmem -run=^#

# Platform-specific variables
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

# Default target
.DEFAULT_GOAL := help

# Phony targets
.PHONY: help all build clean test test-verbose test-race test-coverage \
        lint lint-fix fmt vet security bench run install uninstall \
        deps deps-update deps-tidy release-dry release docker-build \
        docker-run check pre-commit ci

## help: Display this help message
help:
	@echo "$(BLUE)Talia - Domain Availability Checker$(NC)"
	@echo "$(GREEN)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(BLUE)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

## all: Build and test everything
all: clean fmt vet lint test build
	@echo "$(GREEN)✓ All tasks completed successfully$(NC)"

## build: Build the binary for current platform
build:
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✓ Binary built: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

## build-all: Build for multiple platforms
build-all: clean
	@echo "$(BLUE)Building for multiple platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	# macOS AMD64
	@GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	# macOS ARM64
	@GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	# Linux ARM64
	@GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	# Windows AMD64
	@GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "$(GREEN)✓ Multi-platform build complete$(NC)"

## clean: Remove build artifacts
clean:
	@echo "$(BLUE)Cleaning...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@rm -f $(BINARY_NAME)
	@$(GO) clean -cache
	@echo "$(GREEN)✓ Cleaned$(NC)"

## run: Run the application with example args
run: build
	@echo "$(BLUE)Running $(BINARY_NAME)...$(NC)"
	@$(BUILD_DIR)/$(BINARY_NAME) --help

## install: Install the binary to GOPATH/bin
install: build
	@echo "$(BLUE)Installing $(BINARY_NAME)...$(NC)"
	@$(GO) install $(LDFLAGS) $(MAIN_PATH)
	@echo "$(GREEN)✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)$(NC)"

## uninstall: Remove the installed binary
uninstall:
	@echo "$(BLUE)Uninstalling $(BINARY_NAME)...$(NC)"
	@rm -f $(GOPATH)/bin/$(BINARY_NAME)
	@echo "$(GREEN)✓ Uninstalled$(NC)"

##@ Testing

## test: Run tests
test:
	@echo "$(BLUE)Running tests...$(NC)"
	@$(GO) test $(TEST_FLAGS) ./...
	@echo "$(GREEN)✓ Tests passed$(NC)"

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "$(BLUE)Running tests (verbose)...$(NC)"
	@$(GO) test $(TEST_FLAGS) -v ./...

## test-race: Run tests with race detector
test-race:
	@echo "$(BLUE)Running tests with race detector...$(NC)"
	@$(GO) test -race ./...
	@echo "$(GREEN)✓ No race conditions detected$(NC)"

## test-coverage: Generate test coverage report
test-coverage:
	@echo "$(BLUE)Generating coverage report...$(NC)"
	@$(GO) test -coverprofile=$(COVERAGE_FILE) ./...
	@$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@$(GO) tool cover -func=$(COVERAGE_FILE) | tail -1
	@echo "$(GREEN)✓ Coverage report generated: $(COVERAGE_HTML)$(NC)"

## test-integration: Run integration tests (if any)
test-integration:
	@echo "$(BLUE)Running integration tests...$(NC)"
	@$(GO) test -tags=integration $(TEST_FLAGS) ./...
	@echo "$(GREEN)✓ Integration tests passed$(NC)"

## bench: Run benchmarks
bench:
	@echo "$(BLUE)Running benchmarks...$(NC)"
	@$(GO) test $(BENCH_FLAGS) ./...

##@ Code Quality

## fmt: Format code
fmt:
	@echo "$(BLUE)Formatting code...$(NC)"
	@$(GO) fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

## vet: Run go vet
vet:
	@echo "$(BLUE)Running go vet...$(NC)"
	@$(GO) vet ./...
	@echo "$(GREEN)✓ No vet issues$(NC)"

## lint: Run golangci-lint
lint:
	@echo "$(BLUE)Running linter...$(NC)"
	@$(GOLANGCI_LINT) run ./...
	@echo "$(GREEN)✓ No lint issues$(NC)"

## lint-fix: Run golangci-lint with auto-fix
lint-fix:
	@echo "$(BLUE)Running linter with auto-fix...$(NC)"
	@$(GOLANGCI_LINT) run --fix ./...
	@echo "$(GREEN)✓ Lint issues fixed$(NC)"

## security: Run security checks
security:
	@echo "$(BLUE)Running security checks...$(NC)"
	@gosec -quiet ./...
	@echo "$(GREEN)✓ No security issues found$(NC)"

## check: Run all quality checks
check: fmt vet lint security
	@echo "$(GREEN)✓ All checks passed$(NC)"

##@ Dependencies

## deps: Download dependencies
deps:
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	@$(GO) mod download
	@echo "$(GREEN)✓ Dependencies downloaded$(NC)"

## deps-update: Update dependencies
deps-update:
	@echo "$(BLUE)Updating dependencies...$(NC)"
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

## deps-tidy: Tidy dependencies
deps-tidy:
	@echo "$(BLUE)Tidying dependencies...$(NC)"
	@$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencies tidied$(NC)"

## deps-verify: Verify dependencies
deps-verify:
	@echo "$(BLUE)Verifying dependencies...$(NC)"
	@$(GO) mod verify
	@echo "$(GREEN)✓ Dependencies verified$(NC)"

##@ Docker

## docker-build: Build Docker image
docker-build:
	@echo "$(BLUE)Building Docker image...$(NC)"
	@docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .
	@echo "$(GREEN)✓ Docker image built$(NC)"

## docker-run: Run Docker container
docker-run:
	@echo "$(BLUE)Running Docker container...$(NC)"
	@docker run --rm $(BINARY_NAME):latest --help

##@ Release

## release-dry: Dry run of goreleaser
release-dry:
	@echo "$(BLUE)Running release dry-run...$(NC)"
	@goreleaser release --snapshot --skip-publish --clean
	@echo "$(GREEN)✓ Release dry-run complete$(NC)"

## release: Create a new release
release:
	@echo "$(BLUE)Creating release...$(NC)"
	@goreleaser release --clean
	@echo "$(GREEN)✓ Release created$(NC)"

## version: Display version information
version:
	@echo "$(BLUE)Version Information:$(NC)"
	@echo "  Version: $(VERSION)"
	@echo "  Commit:  $(COMMIT)"
	@echo "  Built:   $(BUILD_TIME)"

##@ CI/CD

## ci: Run CI pipeline locally
ci: clean deps check test-race test-coverage build
	@echo "$(GREEN)✓ CI pipeline complete$(NC)"

## pre-commit: Run pre-commit checks
pre-commit: fmt vet lint test
	@echo "$(GREEN)✓ Pre-commit checks passed$(NC)"

##@ Utilities

## tools: Install required tools
tools:
	@echo "$(BLUE)Installing tools...$(NC)"
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	@$(GO) install github.com/goreleaser/goreleaser@latest
	@echo "$(GREEN)✓ Tools installed$(NC)"

## init: Initialize project for development
init: tools deps
	@echo "$(GREEN)✓ Project initialized$(NC)"

## watch: Watch for changes and rebuild
watch:
	@echo "$(BLUE)Watching for changes...$(NC)"
	@while true; do \
		$(MAKE) build; \
		echo "$(YELLOW)Waiting for changes...$(NC)"; \
		fswatch -1 -r --exclude .git --exclude $(BUILD_DIR) .; \
	done

## examples: Run example commands
examples: build
	@echo "$(BLUE)Running examples...$(NC)"
	@echo "$(YELLOW)Example 1: Check single domain$(NC)"
	@echo '  $$ $(BUILD_DIR)/$(BINARY_NAME) --whois=whois.verisign-grs.com:43 example.json'
	@echo ""
	@echo "$(YELLOW)Example 2: Generate domain suggestions$(NC)"
	@echo '  $$ OPENAI_API_KEY=your-key $(BUILD_DIR)/$(BINARY_NAME) --suggest=5 --prompt="tech startup" domains.json'
	@echo ""
	@echo "$(YELLOW)Example 3: Grouped output$(NC)"
	@echo '  $$ $(BUILD_DIR)/$(BINARY_NAME) --whois=whois.verisign-grs.com:43 --grouped-output domains.json'

## stats: Show code statistics
stats:
	@echo "$(BLUE)Code Statistics:$(NC)"
	@echo "$(YELLOW)Lines of code:$(NC)"
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l | tail -1
	@echo "$(YELLOW)Number of Go files:$(NC)"
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | wc -l
	@echo "$(YELLOW)Test files:$(NC)"
	@find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | wc -l
	@echo "$(YELLOW)Package count:$(NC)"
	@go list ./... | wc -l

## todo: Show TODO/FIXME comments
todo:
	@echo "$(BLUE)TODO/FIXME items:$(NC)"
	@grep -r "TODO\|FIXME" --include="*.go" . || echo "$(GREEN)No TODO/FIXME items found$(NC)"
