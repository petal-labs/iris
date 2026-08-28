# Development Guide

Everything needed to build, test, and extend Iris locally.

## Project Structure

```
iris/
├── core/           # Core SDK types and client
├── providers/      # LLM provider implementations
│   ├── internal/   # Shared provider internals (normalize, toolcalls, etc.)
│   ├── openai/     # OpenAI provider (Batch API, files, vector stores, images)
│   ├── anthropic/  # Anthropic Claude provider
│   ├── gemini/     # Google Gemini provider (images)
│   ├── xai/        # xAI Grok provider
│   ├── zai/        # Z.ai GLM provider
│   ├── perplexity/ # Perplexity Search provider (web search + citations)
│   ├── ollama/     # Ollama provider (local and cloud, embeddings)
│   ├── huggingface/# Hugging Face Inference Providers router
│   ├── azurefoundry/# Azure AI Foundry provider (Entra ID auth, embeddings)
│   └── voyageai/   # Voyage AI provider (embeddings, reranking)
├── tools/          # Tool/function calling framework + middleware
├── iristest/       # Test utilities (MockProvider, RecordingProvider)
├── contrib/        # Optional integrations (OpenTelemetry hook)
├── cmd/            # Command entry points
│   ├── iris/       # CLI entry point
│   └── gen-models/ # Model catalog generator
├── cli/            # Command-line interface implementation
│   ├── commands/   # CLI commands
│   ├── config/     # Configuration loading
│   └── keystore/   # Encrypted key storage
└── tests/
    └── integration/ # Provider integration + conformance harness
```

## Prerequisites
- Go 1.24 or later
- Make (optional, for using Makefile commands)

## Getting Started

```bash
# Clone the repository
git clone https://github.com/petal-labs/iris.git
cd iris

# Install git hooks (recommended - prevents formatting issues)
make install-hooks
# or: ./scripts/setup-hooks.sh
```

## Makefile Commands

```bash
make build          # Build all packages
make test           # Run all tests
make test-v         # Run tests with verbose output
make test-cover     # Run tests with coverage
make lint           # Check formatting and run go vet
make fmt            # Auto-fix formatting issues
make vet            # Run go vet
make install-hooks  # Install git pre-commit hooks
make build-cli      # Build CLI to bin/iris (with version info)
make install-cli    # Install CLI locally (with version info)
make test-integration # Run integration tests
make help           # Show all available commands
```

## Building the CLI

The CLI is built with version information injected at build time:

```bash
# Build with version info
make build-cli

# Check version
./bin/iris version
# Output:
# iris v0.18.0
#   commit:     abc1234
#   built:      2026-08-17T12:00:00Z
#   go version: go1.24.0
#   platform:   darwin/arm64

# JSON output
./bin/iris version --json
# Output: {"version":"v0.18.0","commit":"abc1234","buildDate":"2026-08-17T12:00:00Z","goVersion":"go1.24.0","platform":"darwin/arm64"}
```

## Building (without Make)

```bash
# Build everything (SDK + examples)
go build ./...

# Run tests
go test ./...

# Check formatting
gofmt -l .

# Fix formatting
gofmt -w .

# Build CLI with version injection
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
go build -ldflags "-X github.com/petal-labs/iris/cli/commands.Version=$VERSION \
  -X github.com/petal-labs/iris/cli/commands.Commit=$COMMIT \
  -X github.com/petal-labs/iris/cli/commands.BuildDate=$DATE" \
  -o bin/iris ./cmd/iris
```

## Running Tests

```bash
# Run unit tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

## Integration Tests

Integration tests require API keys and make real API calls:

```bash
# Set required environment variables
export OPENAI_API_KEY=your-key
export ANTHROPIC_API_KEY=your-key  # optional
export GEMINI_API_KEY=your-key     # optional
export XAI_API_KEY=your-key        # optional
export ZAI_API_KEY=your-key        # optional
export HF_TOKEN=your-token         # optional

# Run integration tests
go test -tags=integration ./tests/integration/...
```

**CI Behavior**: In CI environments, integration tests fail loudly if required secrets are missing (instead of silently skipping). Set `IRIS_SKIP_INTEGRATION=1` to explicitly skip integration tests in CI.

Provider chat conformance scenarios are centralized in `tests/integration/chat_conformance_test.go` and reused by provider-specific integration files.

## Git Hooks

The repository includes a pre-commit hook that automatically checks:
- `gofmt` - Ensures all Go files are properly formatted
- `go vet` - Catches common mistakes

Install the hooks after cloning:
```bash
make install-hooks
```

This prevents CI failures due to formatting issues.

## Module Structure

Iris uses a Go workspace with three modules:

```
iris/
├── go.mod             # Main SDK module (github.com/petal-labs/iris)
├── go.work            # Workspace file for local development
├── contrib/otel/
│   └── go.mod         # OpenTelemetry integration module
└── examples/
    └── go.mod         # Examples module (github.com/petal-labs/iris/examples)
```

The workspace keeps local imports pointed at the current SDK checkout. Go's
`./...` pattern does not cross nested module boundaries, so include nested
modules explicitly when validating them. CI uses:

```bash
go build ./... ./contrib/otel/...
go test ./... ./contrib/otel/...
```

This ensures changes to interfaces such as `core.TelemetryHook` are compiled
against the OpenTelemetry integration on every pull request.

**Importing the SDK:**

```go
import (
    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/openai"
    "github.com/petal-labs/iris/providers/anthropic"
    "github.com/petal-labs/iris/providers/gemini"
    "github.com/petal-labs/iris/providers/xai"
    "github.com/petal-labs/iris/providers/zai"
    "github.com/petal-labs/iris/providers/perplexity"
    "github.com/petal-labs/iris/providers/ollama"
    "github.com/petal-labs/iris/providers/huggingface"
    "github.com/petal-labs/iris/providers/azurefoundry"
    "github.com/petal-labs/iris/providers/voyageai"
    "github.com/petal-labs/iris/tools"
    "github.com/petal-labs/iris/iristest"  // Test utilities
)
```

## Running Examples

Examples are in a separate module but can be run from the project root thanks to the Go workspace:

```bash
# Run from project root
go run ./examples/chat/basic
go run ./examples/chat/streaming
go run ./examples/chat/responses-api
go run ./examples/chat/ollama-basic
go run ./examples/chat/huggingface-basic
go run ./examples/chat/xai-basic
go run ./examples/chat/zai-basic
go run ./examples/tools/weather

# Or from the examples directory
cd examples
go run ./chat/basic
go run ./chat/responses-api
go run ./chat/ollama-basic
```

See [examples/README.md](../examples/README.md) for detailed documentation on each example.

## Provider Registry

Providers self-register via `init()` functions, making it easy to add new providers:

```go
// In providers/myprovider/register.go
func init() {
    providers.Register("myprovider", func(apiKey string) core.Provider {
        return New(apiKey)
    })
}
```

List registered providers:

```go
import (
    // The registry itself.
    "github.com/petal-labs/iris/providers"

    // Provider packages register themselves via init(). Import the ones
    // you want available — the registry is empty without these imports.
    _ "github.com/petal-labs/iris/providers/anthropic"
    _ "github.com/petal-labs/iris/providers/azurefoundry"
    _ "github.com/petal-labs/iris/providers/gemini"
    _ "github.com/petal-labs/iris/providers/huggingface"
    _ "github.com/petal-labs/iris/providers/ollama"
    _ "github.com/petal-labs/iris/providers/openai"
    _ "github.com/petal-labs/iris/providers/perplexity"
    _ "github.com/petal-labs/iris/providers/voyageai"
    _ "github.com/petal-labs/iris/providers/xai"
    _ "github.com/petal-labs/iris/providers/zai"
)

fmt.Println(providers.List())
// [anthropic azurefoundry gemini huggingface ollama openai perplexity voyageai xai zai]
```

---

See also: [Architecture Design Decisions](ARCHITECTURE.md) · [CONTRIBUTING.md](../CONTRIBUTING.md) · [Documentation index](README.md)
