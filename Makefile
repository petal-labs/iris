# Iris SDK Makefile

.PHONY: all build test lint fmt vet clean install-hooks help

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/erikhoward/iris/cli/commands.Version=$(VERSION) \
	-X github.com/erikhoward/iris/cli/commands.Commit=$(COMMIT) \
	-X github.com/erikhoward/iris/cli/commands.BuildDate=$(BUILD_DATE)"

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

# Run tests with coverage
test-cover:
	go test -cover ./...

# Lint: format check and vet
lint: fmt-check vet

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
	go build $(LDFLAGS) -o bin/iris ./cli/cmd/iris

# Install the CLI locally with version information
install-cli:
	go install $(LDFLAGS) ./cli/cmd/iris

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
	@echo "  test-cover     Run tests with coverage"
	@echo "  lint           Run fmt-check and vet"
	@echo "  fmt-check      Check if files are formatted"
	@echo "  fmt            Format all Go files"
	@echo "  vet            Run go vet"
	@echo "  clean          Clean build artifacts"
	@echo "  install-hooks  Install git pre-commit hooks"
	@echo "  build-cli      Build the CLI to bin/iris (with version info)"
	@echo "  install-cli    Install the CLI locally (with version info)"
	@echo "  test-integration  Run integration tests (requires API keys)"
	@echo "  help           Show this help"
