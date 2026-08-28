# Iris
[![Build Status](https://github.com/petal-labs/iris/actions/workflows/ci.yml/badge.svg)](https://github.com/petal-labs/iris/actions/workflows/ci.yml)&nbsp;
[![codecov](https://codecov.io/gh/petal-labs/iris/graph/badge.svg?token=OJP9V6R441)](https://codecov.io/gh/petal-labs/iris)&nbsp;
[![GoDoc](https://godoc.org/github.com/petal-labs/iris?status.svg)](https://godoc.org/github.com/petal-labs/iris)&nbsp;
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/petal-labs/iris/blob/main/LICENSE)

Iris is a Go SDK and CLI for building AI-powered applications. It provides a unified interface for working with large language models (LLMs), making it easy to integrate AI capabilities into your Go projects.

## 🤔 Why Iris?

Building AI applications often requires:
- Managing multiple LLM provider APIs with different interfaces
- Handling streaming responses, retries, and error normalization
- Securely storing and managing API keys
- Building reusable chat and tool-driven workflows

Iris solves these problems by providing:
- **Unified SDK**: A consistent Go API across providers (OpenAI, Anthropic, Google Gemini, xAI Grok, Z.ai GLM, Perplexity, Ollama, Hugging Face, Azure AI Foundry, Voyage AI)
- **Fluent Builder Pattern**: Intuitive, chainable API for constructing requests
- **Built-in Streaming**: First-class support for streaming responses with proper channel handling
- **Secure Key Management**: Encrypted local storage for API keys
- **CLI Tool**: Quickly test models and manage projects from the command line

## ✨ Features

### SDK Features
- Fluent chat builder with `System()`, `User()`, `Assistant()`, `Temperature()`, `MaxTokens()`, and `Tools()`
- Native token counting for Anthropic and Gemini through `core.TokenCounter`
- Non-streaming and streaming response modes
- Tool/function calling support
- Tool middleware stack for logging, timeout, rate limiting, cache, validation, retry, and circuit breaking
- **Structured Output** with `ResponseJSON()` and `ResponseJSONSchema()` for type-safe responses
- **Conversation Management** with built-in `Conversation` type supporting streaming
- **Batch API** for async processing at 50% cost savings (OpenAI)
- **Testing Utilities** with `MockProvider` and `RecordingProvider`
- **Responses API support** for GPT-5+ models with reasoning, built-in tools (web search, code interpreter), and response chaining
- Automatic retry with exponential backoff
- Telemetry hooks for observability
- Configurable non-fatal warning routing with `core.WithWarningHandler(...)`
- Normalized error types across providers, including `core.ErrTimeout`, `core.ErrInvalidSchema`, and `core.ErrStructuredOutputUnsupported`, plus a `Body` field on `core.ProviderError` carrying the raw (truncated) response body

### CLI Features
- `iris chat` - Send chat completions from the terminal
- `iris keys` - Securely manage API keys with AES-256-GCM encryption and Argon2id key derivation (`set`, `list`, `delete`, `migrate`)
- `iris init` - Scaffold new Iris projects

## 📦 Installation

### SDK

```bash
go get github.com/petal-labs/iris
```

### CLI

```bash
go install github.com/petal-labs/iris/cmd/iris@latest
```

The CLI reads the module version recorded by Go, so binaries installed with
`@latest` report that released version from `iris version`. Release downloads
also include an injected commit and build date. Other source builds report the
module or VCS-derived pseudo-version when Go records one, falling back to `dev`
when build information is unavailable; commit and build date remain unknown
without release linker flags.

## 🚀 Quick Start

### Using the SDK

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/openai"
)

func main() {
    // Create a provider
    provider := openai.New(os.Getenv("OPENAI_API_KEY"))

    // Create a client
    client := core.NewClient(provider)

    // Send a chat request
    resp, err := client.Chat("gpt-5.6").
        System("You are a helpful assistant.").
        User("What is the capital of France?").
        Temperature(0.7).
        GetResponse(context.Background())

    if err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }

    fmt.Println(resp.Output)
    fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)
}
```

### Using Ollama

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/ollama"
)

func main() {
    // Create a local Ollama provider (no API key needed)
    provider := ollama.New()

    // Or connect to a remote Ollama instance:
    // provider := ollama.New(ollama.WithBaseURL("http://remote-host:11434"))

    // Or use Ollama Cloud:
    // provider := ollama.New(
    //     ollama.WithCloud(),
    //     ollama.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
    // )

    // Create a client
    client := core.NewClient(provider)

    // Send a chat request - use any model you have pulled
    resp, err := client.Chat("llama3.2").
        System("You are a helpful assistant.").
        User("What is the capital of France?").
        GetResponse(context.Background())

    if err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }

    fmt.Println(resp.Output)
}
```

Ollama models with thinking support:

```go
// Use thinking with models like qwen3
resp, err := client.Chat("qwen3").
    User("Solve this step by step: What is 15% of 240?").
    ReasoningEffort(core.ReasoningEffortHigh).
    GetResponse(ctx)

// Access reasoning if available
if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
    fmt.Println("Thinking:", resp.Reasoning.Summary[0])
}
```

### Using the CLI

```bash
# Set up your API key (stored encrypted)
iris keys set openai
iris keys set anthropic
iris keys set gemini
iris keys set xai
iris keys set zai
iris keys set huggingface
iris keys set perplexity
iris keys set voyageai
iris keys set azurefoundry
iris keys set ollama  # Only needed for Ollama Cloud

# Upgrade the keystore to master-key encryption (V2)
iris keys migrate  # Requires IRIS_KEYSTORE_KEY (see Security below)

# Chat with OpenAI
iris chat --provider openai --model gpt-5.6 --prompt "Hello, world!"

# Chat with Anthropic Claude
iris chat --provider anthropic --model claude-sonnet-5 --prompt "Hello, world!"

# Chat with Google Gemini
iris chat --provider gemini --model gemini-3.6-flash --prompt "Hello, world!"

# Chat with xAI Grok
iris chat --provider xai --model grok-4.5 --prompt "Hello, world!"

# Chat with Z.ai GLM
iris chat --provider zai --model glm-5.2 --prompt "Hello, world!"

# Chat with local Ollama (no API key needed)
iris chat --provider ollama --model llama3.2 --prompt "Hello, world!"

# Chat with GPT-5 (uses Responses API automatically)
iris chat --provider openai --model gpt-5 --prompt "Explain quantum entanglement"

# Stream responses
iris chat --provider openai --model gpt-5.6 --prompt "Tell me a story" --stream
iris chat --provider anthropic --model claude-sonnet-5 --prompt "Tell me a story" --stream

# Get JSON output
iris chat --provider openai --model gpt-5.6 --prompt "Hello" --json

# Initialize a standalone project (creates main.go, go.mod, and tools/)
iris init myproject --provider perplexity
cd myproject
export PERPLEXITY_API_KEY=<your-key>
go run .
```

## 📚 Documentation

The guides below live in [`docs/`](docs/README.md). Start there for the full index.

| Guide | What it covers |
|---|---|
| [Provider Quick Starts](docs/guides/provider-quickstarts.md) | Anthropic, Gemini, xAI Grok, Z.ai GLM, and Perplexity examples |
| [Provider Comparison](docs/PROVIDERS.md) | Feature matrix, model catalogs, endpoints, and auth per provider |
| [Streaming Responses](docs/guides/streaming.md) | `ChatStream` channels and `DrainStream` |
| [Timeouts and Warning Hooks](docs/guides/timeouts-and-warnings.md) | Execution deadlines, precedence rules, non-fatal warning routing |
| [Tools and Function Calling](docs/guides/tools.md) | The `tools.Tool` interface and the middleware stack |
| [Structured Output](docs/guides/structured-output.md) | `ResponseJSON()` and strict `ResponseJSONSchema()` |
| [Conversation Management](docs/guides/conversations.md) | `core.Conversation`, history, and tool results |
| [Multimodal and Images](docs/guides/multimodal.md) | Image inputs, generation, editing, and streaming partials |
| [Responses API](docs/guides/responses-api.md) | Reasoning, built-in tools, and response chaining on GPT-5+ |
| [Batch API](docs/guides/batch-api.md) | Async submission and polling for cost savings |
| [Testing Utilities](docs/guides/testing.md) | `MockProvider` and `RecordingProvider` |
| [Architecture Decisions](docs/ARCHITECTURE.md) | Why the SDK is shaped the way it is |
| [Security Guide](docs/SECURITY.md) | Keystore internals, the `Secret` type, CI/CD practices |
| [Development Guide](docs/DEVELOPMENT.md) | Project layout, Makefile targets, tests, provider registry |

## ⚙️ Configuration

Iris looks for configuration at `~/.iris/config.yaml`. The schema is:

| Field | Purpose |
|---|---|
| `default_provider` | Provider used when `--provider` is omitted |
| `default_model` | Model used when `--model` is omitted |
| `providers.<id>.base_url` | Override a provider's API base URL |
| `providers.azurefoundry.endpoint` | Azure AI Foundry resource endpoint |
| `providers.azurefoundry.deployment_id` | Optional Azure deployment ID |
| `providers.azurefoundry.api_version` | Optional Azure API version override |
| `providers.azurefoundry.use_openai_endpoint` | Use Azure OpenAI deployment-style request paths |

```yaml
default_provider: openai
default_model: gpt-5.6  # or gpt-4o for older models

providers:
  ollama:
    # Custom base URL for a remote/local Ollama instance (default: http://localhost:11434)
    base_url: http://remote-host:11434
  azurefoundry:
    endpoint: https://my-resource.openai.azure.com
    deployment_id: production-chat
    use_openai_endpoint: true
```

API keys are **not** configured here. The CLI reads provider keys from the encrypted keystore (`iris keys set <provider>`, see [Security](#-security)); the SDK reads them from environment variables when you use the provider packages directly. For Azure AI Foundry, `AZURE_AI_ENDPOINT` and `AZURE_AI_DEPLOYMENT_ID` are also accepted when their corresponding config fields are omitted. `base_url` remains a backward-compatible alias for the Azure endpoint.

## 🔒 Security

### Setting Up Keystore Encryption

For production use, set a master encryption key for the keystore:

```bash
# Generate a strong random key
export IRIS_KEYSTORE_KEY=$(openssl rand -base64 32)

# Add to your shell profile for persistence
echo 'export IRIS_KEYSTORE_KEY="your-key-here"' >> ~/.bashrc
```

All keystore-backed commands (`iris keys`, `iris chat`) honor `IRIS_KEYSTORE_KEY`. When it is set, Iris uses the V2 keystore format with:
- **Argon2id** key derivation (OWASP recommended parameters)
- **AES-256-GCM** authenticated encryption
- Per-file random salt and nonce

Stores written by older versions remain readable (decryption falls back to the legacy key), and they are re-encrypted under your master key on the next write. To upgrade an existing store explicitly:

```bash
iris keys migrate
# Re-encrypts ~/.iris/keys.enc in V2 format; the previous file is kept at ~/.iris/keys.enc.bak
```

Without `IRIS_KEYSTORE_KEY`, Iris falls back to V1 mode which derives keys from machine-specific data. This is convenient for development but less secure for production — the CLI prints a warning on every keystore operation while in this mode.

### API Key Protection

Iris uses a `Secret` type that prevents accidental logging of API keys:

```go
secret := core.NewSecret(os.Getenv("OPENAI_API_KEY"))
fmt.Println(secret)        // Prints: [REDACTED]
apiKey := secret.Expose()  // Access actual value when needed
```

`core.NewSecret` trims leading/trailing whitespace (including the trailing newlines some secret managers, like Azure Key Vault, append), so `Expose()` never returns a padded credential and `IsEmpty()` correctly treats whitespace-only values as empty. Passing an empty OpenAI key surfaces a descriptive `core.ErrUnauthorized` before any HTTP call is made.

See [docs/SECURITY.md](docs/SECURITY.md) for comprehensive security documentation.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🤝 Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, test tiers, and PR expectations.
