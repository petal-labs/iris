# Iris SDK Makefile

.PHONY: all build test lint lint-ci install-golangci-lint fmt vet clean install-hooks help coverage

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/petal-labs/iris/cli/commands.Version=$(VERSION) \
	-X github.com/petal-labs/iris/cli/commands.Commit=$(COMMIT) \
	-X github.com/petal-labs/iris/cli/commands.BuildDate=$(BUILD_DATE)"

# Keep this pin synchronized with .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION := v2.5.0
GOLANGCI_LINT_VERSION_NUMBER := $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))
GOLANGCI_LINT ?= golangci-lint

# Default target
all: lint test build

# Build all packages
build:
	go build ./...

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Run tests with coverage summary
test-cover:
	go test -cover ./...

# Generate coverage profile for CI/Codecov
coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Coverage report generated: coverage.out"

# Generate and view HTML coverage report
coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML report generated: coverage.html"

# Run the same formatting, vet, and golangci-lint checks as CI
lint: fmt-check vet lint-ci

# Run the CI-pinned golangci-lint version and repository configuration
lint-ci:
	@if ! command -v "$(GOLANGCI_LINT)" >/dev/null 2>&1; then \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION) is required."; \
		echo "Run 'make install-golangci-lint' and retry."; \
		exit 1; \
	fi
	@VERSION_OUTPUT=$$("$(GOLANGCI_LINT)" version 2>&1); \
	case "$$VERSION_OUTPUT" in \
		*"version $(GOLANGCI_LINT_VERSION_NUMBER)"*) ;; \
		*) \
			echo "Expected golangci-lint $(GOLANGCI_LINT_VERSION), but found: $$VERSION_OUTPUT"; \
			echo "Run 'make install-golangci-lint' and retry."; \
			exit 1; \
		;; \
	esac
	"$(GOLANGCI_LINT)" run

# Install the exact golangci-lint release used by CI
install-golangci-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Check formatting (fails if files need formatting)
fmt-check:
	@echo "Checking gofmt..."
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "The following files need formatting:"; \
		echo "$$UNFORMATTED"; \
		echo ""; \
		echo "Run 'make fmt' to fix."; \
		exit 1; \
	fi
	@echo "All files formatted correctly."

# Format all Go files
fmt:
	gofmt -w .

# Run go vet
vet:
	go vet ./...

# Clean build artifacts
clean:
	go clean ./...

# Install git hooks
install-hooks:
	./scripts/setup-hooks.sh

# Build the CLI with version information
build-cli:
	go build $(LDFLAGS) -o bin/iris ./cmd/iris

# Install the CLI locally with version information
install-cli:
	go install $(LDFLAGS) ./cmd/iris

# Run integration tests (requires API keys)
test-integration:
	go test -tags=integration ./tests/integration/...

# Help
help:
	@echo "Iris SDK Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all            Run lint, test, and build (default)"
	@echo "  build          Build all packages"
	@echo "  test           Run all tests"
	@echo "  test-v         Run tests with verbose output"
	@echo "  test-cover     Run tests with coverage summary"
	@echo "  coverage       Generate coverage.out profile for Codecov"
	@echo "  coverage-html  Generate HTML coverage report"
	@echo "  lint           Run fmt-check, vet, and the CI-pinned golangci-lint"
	@echo "  lint-ci        Run golangci-lint with CI version validation"
	@echo "  install-golangci-lint  Install the golangci-lint version used by CI"
	@echo "  fmt-check      Check if files are formatted"
	@echo "  fmt            Format all Go files"
	@echo "  vet            Run go vet"
	@echo "  clean          Clean build artifacts"
	@echo "  install-hooks  Install git pre-commit hooks"
	@echo "  build-cli      Build the CLI to bin/iris (with version info)"
	@echo "  install-cli    Install the CLI locally (with version info)"
	@echo "  test-integration  Run integration tests (requires API keys)"
	@echo "  help           Show this help"
