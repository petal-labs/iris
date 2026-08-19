# Iris
[![Build Status](https://github.com/petal-labs/iris/actions/workflows/ci.yml/badge.svg)](https://github.com/petal-labs/iris/actions/workflows/ci.yml)&nbsp;
[![codecov](https://codecov.io/gh/petal-labs/iris/graph/badge.svg?token=OJP9V6R441)](https://codecov.io/gh/petal-labs/iris)&nbsp;
[![GoDoc](https://godoc.org/github.com/petal-labs/iris?status.svg)](https://godoc.org/github.com/petal-labs/iris)&nbsp;
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/petal-labs/iris/blob/main/LICENSE)

Iris is a Go SDK and CLI for building AI-powered applications. It provides a unified interface for working with large language models (LLMs), making it easy to integrate AI capabilities into your Go projects.

## Why Iris?

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

## Features

### SDK Features
- Fluent chat builder with `System()`, `User()`, `Assistant()`, `Temperature()`, `MaxTokens()`, and `Tools()`
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
## Installation

### SDK

```bash
go get github.com/petal-labs/iris
```

### CLI

```bash
go install github.com/petal-labs/iris/cli/cmd/iris@latest
```

## Quick Start

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

### Using Anthropic Claude

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/anthropic"
)

func main() {
    // Create an Anthropic provider
    provider := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))

    // Create a client
    client := core.NewClient(provider)

    // Send a chat request (using latest Claude Sonnet 5)
    resp, err := client.Chat("claude-sonnet-5").
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

### Using Google Gemini

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/gemini"
)

func main() {
    // Create a Gemini provider
    provider := gemini.New(os.Getenv("GEMINI_API_KEY"))

    // Create a client
    client := core.NewClient(provider)

    // Send a chat request (using latest Gemini 3.6 Flash)
    resp, err := client.Chat("gemini-3.6-flash").
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

Gemini models with thinking/reasoning support:

```go
// Use reasoning with Gemini 2.5 models (budget-based)
resp, err := client.Chat("gemini-2.5-pro").
    User("Solve this complex problem step by step").
    ReasoningEffort(core.ReasoningEffortHigh).
    GetResponse(ctx)

// Access reasoning if available
if resp.Reasoning != nil && resp.Reasoning.Output != "" {
    fmt.Println("Thinking:", resp.Reasoning.Output)
}
```

### Using xAI Grok

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/xai"
)

func main() {
    // Create an xAI provider
    provider := xai.New(os.Getenv("XAI_API_KEY"))

    // Create a client
    client := core.NewClient(provider)

    // Send a chat request using Grok 4.5
    resp, err := client.Chat(xai.ModelGrok45).
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

xAI Grok models with reasoning support:

```go
// Use reasoning with grok-3-mini (only model that exposes reasoning_content)
resp, err := client.Chat(xai.ModelGrok3Mini).
    User("Solve this step by step: If I have 5 apples and give away half...").
    ReasoningEffort(core.ReasoningEffortHigh).
    GetResponse(ctx)

// Access reasoning if available (grok-3-mini only)
if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
    fmt.Println("Thinking:", resp.Reasoning.Summary[0])
}
```

### Using Z.ai GLM

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/zai"
)

func main() {
    // Create a Z.ai provider
    provider := zai.New(os.Getenv("ZAI_API_KEY"))

    // Create a client
    client := core.NewClient(provider)

    // Send a chat request using GLM-5.2
    resp, err := client.Chat(zai.ModelGLM52).
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

Z.ai GLM models with thinking support:

```go
// Use thinking mode with GLM-5.2 (enabled by default)
resp, err := client.Chat(zai.ModelGLM52).
    User("Solve this step by step: What is 15% of 240?").
    ReasoningEffort(core.ReasoningEffortHigh).
    GetResponse(ctx)

// Access reasoning if available
if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
    fmt.Println("Thinking:", resp.Reasoning.Summary[0])
}
```

### Using Perplexity Search

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/perplexity"
)

func main() {
    // Create a Perplexity provider
    provider := perplexity.New(os.Getenv("PERPLEXITY_API_KEY"))

    // Create a client
    client := core.NewClient(provider)

    // Send a search-grounded chat request
    resp, err := client.Chat(perplexity.ModelSonar).
        System("You are a helpful assistant.").
        User("What are the latest developments in AI?").
        GetResponse(context.Background())

    if err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }

    fmt.Println(resp.Output)
}
```

#### Search Options and Citations

Control how Perplexity grounds its search and retrieve the sources it used. Domain filters accept a `-` prefix to exclude a domain, and `Recency` limits result freshness (`hour`, `day`, `week`, `month`, `year`):

```go
resp, err := client.Chat(perplexity.ModelSonar).
    User("Summarize the latest Go release notes").
    SearchOptions(&core.SearchOptions{
        SearchDomainFilter: []string{"go.dev", "-old.docs.example.com"},
        Recency:            core.SearchRecencyMonth,
        Mode:               core.SearchModeWeb, // or SearchModeAcademic / SearchModeSEC
    }).
    GetResponse(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Output)
for _, url := range resp.Citations {
    fmt.Println("source:", url)
}
```

Citations are also populated on the final response of a stream (`core.DrainStream` returns them). Requests carrying `SearchOptions` against providers without web-search support fail fast with `core.ErrSearchUnsupported` before any HTTP call is made.

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

### Timeouts

`Chat`/`GetResponse` and `Stream` calls have a default 120-second execution timeout (`core.DefaultTimeout`). Adjust it client-wide with `core.WithTimeout(d)`, or per-call with `.Timeout(d)`:

```go
// Client-wide: raise the default to 5 minutes
client := core.NewClient(provider, core.WithTimeout(5*time.Minute))

// Disable the default entirely (unbounded, unless ctx has its own deadline)
client := core.NewClient(provider, core.WithTimeout(0))

// Per-call override
resp, err := client.Chat("gpt-5.6").
    User("Hello").
    Timeout(30 * time.Second).
    GetResponse(context.Background())

if errors.Is(err, core.ErrTimeout) {
    // errors.Is(err, context.DeadlineExceeded) also holds
    fmt.Println("request timed out")
}
```

Precedence: a deadline already on the caller's `ctx` always wins; otherwise the per-call `.Timeout(d)` applies; otherwise the client's default. This timeout covers `Chat`/`Stream` only. Non-chat unary operations (embeddings, batch, files, images, vector stores) are instead bounded by the per-provider `WithTimeout`/`Config.Timeout` option (default 600s), or by a `context.WithTimeout` deadline you supply yourself.

### Streaming Responses

```go
stream, err := client.Chat("gpt-5.6").
    User("Write a short poem about Go.").
    Stream(context.Background())

if err != nil {
    log.Fatal(err)
}

// Print chunks as they arrive
for chunk := range stream.Ch {
    fmt.Print(chunk.Delta)
}
fmt.Println()

// Or use DrainStream to collect everything
resp, err := core.DrainStream(ctx, stream)
```

### Warning Hooks

Route non-fatal SDK warnings (for example, mismatched tool result IDs) into your application logger:

```go
client := core.NewClient(provider,
    core.WithWarningHandler(func(msg string) {
        log.Printf("iris warning: %s", msg)
    }),
)
```

### Using Tools

```go
// Define a tool
weatherTool := mytools.NewWeatherTool()

resp, err := client.Chat("gpt-5.6").
    User("What's the weather in San Francisco?").
    Tools(weatherTool).
    GetResponse(ctx)

if len(resp.ToolCalls) > 0 {
    // Handle tool calls
    for _, call := range resp.ToolCalls {
        fmt.Printf("Tool: %s, Args: %s\n", call.Name, call.Arguments)
    }
}
```

### Tool Middleware and Validation

Wrap tools with middleware before passing them to `Tools(...)` or invoking them directly:

```go
logger := log.New(os.Stdout, "tool ", 0)
weatherTool := mytools.NewWeatherTool()

wrappedTool := tools.ApplyMiddleware(
    weatherTool,
    tools.WithBasicValidation(),
    tools.WithTimeout(5*time.Second),
    tools.WithLogging(logger),
)

resp, err := client.Chat("gpt-5.6").
    User("What's the weather in San Francisco?").
    Tools(wrappedTool).
    GetResponse(ctx)
```

Use `tools.WithValidation(...)` with a custom schema validator when you want JSON-schema enforcement. Tool schemas are propagated automatically through `ToolContext`.

### Structured Output

Constrain model output to valid JSON or a specific JSON Schema:

```go
// JSON mode - model outputs valid JSON
resp, err := client.Chat("gpt-5.6").
    User("List 3 programming languages with their year created").
    ResponseJSON().
    GetResponse(ctx)

// Parse the JSON response
var languages []struct {
    Name string `json:"name"`
    Year int    `json:"year"`
}
json.Unmarshal([]byte(resp.Output), &languages)
```

For strict schema enforcement:

```go
schema := &core.JSONSchemaDefinition{
    Name:   "person",
    Strict: true,
    Schema: json.RawMessage(`{
        "type": "object",
        "additionalProperties": false,
        "properties": {
            "name": {"type": "string"},
            "age": {"type": "integer"}
        },
        "required": ["name", "age"]
    }`),
}

resp, err := client.Chat("gpt-5.6").
    User("Extract: John is 30 years old").
    ResponseJSONSchema(schema).
    GetResponse(ctx)

// Output is guaranteed to match the schema
```

`ResponseJSONSchema` is strict-by-default: it forces `schema.Strict = true` and validates the schema up front, so every object node in the schema must set `"additionalProperties": false` and list all of its properties in `"required"`. A schema that doesn't meet those constraints returns `core.ErrInvalidSchema` before any request is sent. Use `ResponseJSONSchemaNonStrict(schema)` to opt out and skip that validation. Requesting a schema against a provider or model that doesn't support structured output returns `core.ErrStructuredOutputUnsupported`, also before the call is made. Structured output is currently supported on OpenAI (both the Chat Completions and Responses API, GPT-5.x) and Google Gemini; other providers reject `ResponseJSONSchema` requests with that error. Plain `ResponseJSON()` (JSON mode) is not gated and works across providers that support `json_object`-style output.

### Conversation Management

The `Conversation` type manages message history automatically:

```go
// Create a conversation with a system prompt
conv := core.NewConversation(client, "gpt-5.6",
    core.WithSystemMessage("You are a helpful assistant."),
)

// Send messages - history is managed automatically
resp1, _ := conv.Send("What is Go?")
resp2, _ := conv.Send("What are its main features?") // Remembers context

// Streaming responses
stream, _ := conv.Stream("Tell me more about concurrency")
for chunk := range stream.Ch {
    fmt.Print(chunk.Delta)
}
```

### Batch API

Submit requests for async processing at 50% cost savings:

```go
// Check if provider supports batch
bp, ok := core.AsBatchProvider(provider)
if !ok {
    log.Fatal("Provider does not support batch API")
}

// Create batch requests
requests := []core.BatchRequest{
    {CustomID: "req-1", Request: core.ChatRequest{Model: "gpt-5.6", Messages: msgs1}},
    {CustomID: "req-2", Request: core.ChatRequest{Model: "gpt-5.6", Messages: msgs2}},
}

// Submit batch
batchID, _ := bp.CreateBatch(ctx, requests)

// Wait for completion (with polling)
waiter := core.NewBatchWaiter(bp).
    WithPollInterval(30 * time.Second).
    WithMaxWait(24 * time.Hour)

results, _ := waiter.WaitAndCollect(ctx, batchID)

for _, result := range results {
    if result.IsSuccess() {
        fmt.Printf("%s: %s\n", result.CustomID, result.Response.Output)
    }
}
```

### Testing Utilities

The `testing` package provides utilities for deterministic tests:

```go
import "github.com/petal-labs/iris/testing"

// MockProvider for predictable responses
mock := testing.NewMockProvider().
    WithResponse("Hello!").
    WithResponse("Follow-up response")

client := core.NewClient(mock)
resp, _ := client.Chat("any-model").User("Hi").GetResponse(ctx)
// resp.Output == "Hello!"

// RecordingProvider for capturing interactions
recorder := testing.NewRecordingProvider(realProvider)
client := core.NewClient(recorder)

// ... run your code ...

// Inspect recorded calls
for _, call := range recorder.Calls() {
    fmt.Printf("Model: %s, Messages: %d\n", call.Request.Model, len(call.Request.Messages))
}
```

### Image Generation

Generate images using OpenAI's image models:

```go
provider := openai.New(os.Getenv("OPENAI_API_KEY"))

// Generate an image
resp, err := provider.GenerateImage(ctx, &core.ImageGenerateRequest{
    Model:   openai.ModelGPTImage2,
    Prompt:  "A serene mountain landscape at sunset",
    Size:    core.ImageSize1024x1024,
    Quality: core.ImageQualityHigh,
})

// Save the image
data, _ := resp.Data[0].GetBytes()
os.WriteFile("landscape.png", data, 0644)
```

#### Streaming Partial Images

```go
stream, _ := provider.StreamImage(ctx, &core.ImageGenerateRequest{
    Model:         openai.ModelGPTImage1,
    Prompt:        "A futuristic cityscape",
    PartialImages: 3,
})

for chunk := range stream.Ch {
    // Process partial image
    fmt.Printf("Partial %d received\n", chunk.PartialImageIndex)
}

final := <-stream.Final
// Save final image
```

#### Editing Images

```go
imageData, _ := os.ReadFile("input.png")

resp, _ := provider.EditImage(ctx, &core.ImageEditRequest{
    Model:  openai.ModelGPTImage1,
    Prompt: "Add a rainbow in the sky",
    Images: []core.ImageInput{
        {Data: imageData},
    },
    InputFidelity: core.ImageInputFidelityHigh,
})
```

#### Supported Image Models

| Model | Description |
|-------|-------------|
| `gpt-image-2` | Latest GPT Image model |
| `gpt-image-1.5` | GPT Image 1.5 |
| `gpt-image-1` | Standard GPT Image |
| `gpt-image-1-mini` | Fast, cost-effective |
| `dall-e-3` | High quality (deprecated May 2026) |
| `dall-e-2` | Lower cost, inpainting (deprecated May 2026) |

### Using the Responses API (GPT-5.6)

GPT-5+ models automatically use OpenAI's Responses API, which provides advanced features like reasoning, built-in tools, and response chaining.

```go
// GPT-5.6 uses the Responses API automatically
resp, err := client.Chat("gpt-5.6").
    Instructions("You are a helpful research assistant.").
    User("What are the latest developments in quantum computing?").
    ReasoningEffort(core.ReasoningEffortHigh).
    WebSearch().
    GetResponse(ctx)

if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Output)

// Access reasoning summary if available
if resp.Reasoning != nil {
    for _, summary := range resp.Reasoning.Summary {
        fmt.Println("Reasoning:", summary)
    }
}

// Response chaining - continue from a previous response
followUp, err := client.Chat("gpt-5.6").
    ContinueFrom(resp.ID).
    User("Can you elaborate on the most promising approach?").
    GetResponse(ctx)
```

### Using the CLI

```bash
# Set up your API key (stored encrypted)
iris keys set openai
iris keys set anthropic
iris keys set gemini
iris keys set xai
iris keys set zai
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

# Initialize a new project
iris init myproject
```

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
├── testing/        # Test utilities (MockProvider, RecordingProvider)
├── contrib/        # Optional integrations (OpenTelemetry hook)
├── cli/            # Command-line interface
│   ├── cmd/iris/   # CLI entry point
│   ├── commands/   # CLI commands
│   ├── config/     # Configuration loading
│   └── keystore/   # Encrypted key storage
└── tests/
    └── integration/ # Provider integration + conformance harness
```

## Configuration

Iris looks for configuration at `~/.iris/config.yaml`. The schema is:

| Field | Purpose |
|---|---|
| `default_provider` | Provider used when `--provider` is omitted |
| `default_model` | Model used when `--model` is omitted |
| `providers.<id>.base_url` | Override a provider's API base URL |

```yaml
default_provider: openai
default_model: gpt-5.6  # or gpt-4o for older models

providers:
  ollama:
    # Custom base URL for a remote/local Ollama instance (default: http://localhost:11434)
    base_url: http://remote-host:11434
```

API keys are **not** configured here. The CLI reads provider keys from the encrypted keystore (`iris keys set <provider>`, see [Security](#security)); the SDK reads them from environment variables when you use the provider packages directly.

## Security

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

## Supported Providers

| Provider | Status | Features |
|----------|--------|----------|
| OpenAI | Supported | Chat, Streaming, Tools, Batch API, Structured Output, Embeddings, Images, Responses API (GPT-5+) |
| Anthropic | Supported | Chat, Streaming, Tools, Reasoning |
| Google Gemini | Supported | Chat, Streaming, Tools, Reasoning, Structured Output, Images |
| xAI Grok | Supported | Chat, Streaming, Tools, Reasoning |
| Z.ai GLM | Supported | Chat, Streaming, Tools, Thinking |
| Perplexity | Supported | Chat, Streaming, Tools, Reasoning, Web Search + Citations |
| Ollama | Supported | Chat, Streaming, Tools, Thinking, Embeddings |
| Hugging Face | Supported | Chat, Streaming, Tools (Inference Providers router, model discovery) |
| Azure AI Foundry | Supported | Chat, Streaming, Tools, Reasoning, Structured Output, Embeddings (Entra ID auth) |
| Voyage AI | Supported | Embeddings, Contextualized Embeddings, Reranking (no chat) |

### xAI Grok Models

| Model ID | Features |
|----------|----------|
| `grok-4.5` | Chat, Streaming, Tools, Reasoning (latest) |
| `grok-4.3` | Chat, Streaming, Tools, Reasoning |
| `grok-4.20-multi-agent-beta-0309` | Chat, Streaming, Tools, Reasoning (multi-agent beta) |
| `grok-4.20-beta-0309-reasoning` | Chat, Streaming, Tools, Reasoning (beta) |
| `grok-4.20-beta-0309-non-reasoning` | Chat, Streaming, Tools (beta) |
| `grok-4.1` | Chat, Streaming, Tools |
| `grok-4-1-fast-non-reasoning` | Chat, Streaming, Tools (default for CLI) |
| `grok-4-1-fast-reasoning` | Chat, Streaming, Tools, Reasoning |
| `grok-4` | Chat, Streaming, Tools, Reasoning |
| `grok-4-fast-non-reasoning` | Chat, Streaming, Tools |
| `grok-4-fast-reasoning` | Chat, Streaming, Tools, Reasoning |
| `grok-3` | Chat, Streaming, Tools, Reasoning |
| `grok-3-mini` | Chat, Streaming, Tools, Reasoning (exposes reasoning_content) |
| `grok-code-fast` | Chat, Streaming, Tools (code-optimized) |
| `grok-build-0.1` | Chat, Streaming, Tools, Reasoning (build/agentic) |

### Z.ai GLM Models

| Model ID | Features |
|----------|----------|
| `glm-5.2` | Chat, Streaming, Tools, Thinking (latest) |
| `glm-5.1` | Chat, Streaming, Tools, Thinking |
| `glm-5` | Chat, Streaming, Tools, Thinking |
| `glm-5-turbo` | Chat, Streaming, Tools, Thinking |
| `glm-5v-turbo` | Chat, Streaming, Tools, Thinking, Vision |
| `glm-4.7` | Chat, Streaming, Tools, Thinking |
| `glm-4.7-flash` | Chat, Streaming, Tools (default for CLI) |
| `glm-4.7-flashx` | Chat, Streaming, Tools |
| `glm-4.6` | Chat, Streaming, Tools, Thinking |
| `glm-4.6v` | Chat, Streaming, Tools, Thinking, Vision |
| `glm-4.6v-flash` | Chat, Streaming, Tools, Vision |
| `glm-4.6v-flashx` | Chat, Streaming, Tools, Vision |
| `glm-4.5` | Chat, Streaming, Tools, Thinking |
| `glm-4.5v` | Chat, Streaming, Tools, Thinking, Vision |
| `glm-4.5-x` | Chat, Streaming, Tools |
| `glm-4.5-air` | Chat, Streaming, Tools |
| `glm-4.5-airx` | Chat, Streaming, Tools |
| `glm-4.5-flash` | Chat, Streaming, Tools |
| `glm-for-coding` | Chat, Streaming, Tools, Thinking (code-optimized) |
| `glm-4-32b-0414-128k` | Chat, Streaming, Tools (128K context) |

### Perplexity Models

| Model ID | Features |
|----------|----------|
| `sonar` | Chat, Streaming, Tools, Web Search (lightweight) |
| `sonar-pro` | Chat, Streaming, Tools, Web Search (advanced) |
| `sonar-reasoning-pro` | Chat, Streaming, Tools, Web Search, Reasoning |
| `sonar-deep-research` | Chat, Streaming, Web Search, Reasoning (research) |

### Gemini Models

| Model ID | Features |
|----------|----------|
| `gemini-3.6-flash` | Chat, Streaming, Tools, Reasoning (thinkingLevel, latest) |
| `gemini-3.5-flash` | Chat, Streaming, Tools, Reasoning (thinkingLevel) |
| `gemini-3.5-flash-lite` | Chat, Streaming, Tools, Reasoning (thinkingLevel) |
| `gemini-3.1-pro-preview` | Chat, Streaming, Tools, Reasoning (thinkingLevel) |
| `gemini-3.1-flash-lite` | Chat, Streaming, Tools, Reasoning (thinkingLevel) |
| `gemini-3.1-flash-image-preview` | Chat, Streaming, Image Generation |
| `gemini-3-pro-preview` | Chat, Streaming, Tools, Reasoning (thinkingLevel) |
| `gemini-3-flash-preview` | Chat, Streaming, Tools, Reasoning (thinkingLevel) |
| `gemini-3-pro-image-preview` | Image Generation (Nano Banana Pro) |
| `gemini-2.5-pro` | Chat, Streaming, Tools, Reasoning (thinkingBudget) |
| `gemini-2.5-flash` | Chat, Streaming, Tools, Reasoning (thinkingBudget) |
| `gemini-2.5-flash-lite` | Chat, Streaming, Tools, Reasoning (thinkingBudget) |
| `gemini-2.5-flash-image` | Image Generation (Nano Banana) |
| `gemini-2.0-flash-lite` | Chat, Streaming |

### Ollama Models

Ollama supports any model you have pulled locally. Use `ollama pull <model>` to download models.

| Model ID | Features |
|----------|----------|
| `llama3.2` | Chat, Streaming, Tools |
| `llama3.2:70b` | Chat, Streaming, Tools |
| `mistral` | Chat, Streaming, Tools |
| `mixtral` | Chat, Streaming, Tools |
| `qwen3` | Chat, Streaming, Tools, Thinking |
| `gemma3` | Chat, Streaming |
| `deepseek-coder` | Chat, Streaming |
| `codellama` | Chat, Streaming |

See https://ollama.com/library for all available models.

## Development

### Prerequisites
- Go 1.24 or later
- Make (optional, for using Makefile commands)

### Getting Started

```bash
# Clone the repository
git clone https://github.com/petal-labs/iris.git
cd iris

# Install git hooks (recommended - prevents formatting issues)
make install-hooks
# or: ./scripts/setup-hooks.sh
```

### Makefile Commands

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

### Building the CLI

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

### Building (without Make)

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
  -o bin/iris ./cli/cmd/iris
```

### Running Tests

```bash
# Run unit tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Integration Tests

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

### Git Hooks

The repository includes a pre-commit hook that automatically checks:
- `gofmt` - Ensures all Go files are properly formatted
- `go vet` - Catches common mistakes

Install the hooks after cloning:
```bash
make install-hooks
```

This prevents CI failures due to formatting issues.

### Module Structure

Iris uses a Go workspace with two modules:

```
iris/
├── go.mod        # Main SDK module (github.com/petal-labs/iris)
├── go.work       # Workspace file for local development
└── examples/
    └── go.mod    # Examples module (github.com/petal-labs/iris/examples)
```

The workspace allows you to develop on both modules simultaneously. When you run `go build ./...` or `go test ./...` from the root, it builds/tests both modules.

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
    "github.com/petal-labs/iris/testing"  // Test utilities
)
```

### Running Examples

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

See [examples/README.md](examples/README.md) for detailed documentation on each example.

### Provider Registry

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

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, test tiers, and PR expectations.
