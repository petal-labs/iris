# Architecture Design Decisions

This document explains the key architectural decisions in the Iris SDK and the rationale behind them.

## Why Streaming Is First-Class

### Decision
Streaming is not an afterthought in Iris. The `ChatStream` type with its three-channel design (Ch, Err, Final) is a core primitive, and all providers must implement `StreamChat`.

### Rationale

**User Experience**: LLM responses can take seconds to generate. Users expect to see tokens as they're generated, not wait for the entire response. Streaming provides immediate feedback and perceived performance improvement.

**Memory Efficiency**: Streaming avoids buffering entire responses in memory before returning. For long responses, this significantly reduces memory pressure.

**Cancellation Support**: Users may want to stop generation early. Streaming with context cancellation allows graceful termination without wasting API calls or compute.

**Tool Call Handling**: Modern LLMs interleave text output with tool calls. Streaming surfaces tool calls as they occur, enabling faster tool-driven workflows.

### Design Details

```go
type ChatStream struct {
    Ch    <-chan ChatChunk   // Text deltas in order
    Err   <-chan error       // At most one error
    Final <-chan *ChatResponse // Complete response with usage/tool calls
}
```

**Why Three Channels?**

- `Ch`: Separating content from errors allows clean iteration (`for chunk := range stream.Ch`)
- `Err`: A dedicated error channel prevents mixing error handling with content reading
- `Final`: Provides the complete response with usage statistics and tool calls, which aren't available until stream completion

**Provider Contract**: Providers MUST:
- Close all three channels when finished
- Terminate promptly on context cancellation
- Send at most one error on Err
- Send exactly one response on Final (or zero on setup failure)

### Alternatives Considered

**Single Channel with Union Type**: Could return `StreamEvent` with either content, error, or final. Rejected because it complicates the common case of just reading text deltas.

**Callback-Based API**: Pass functions for each event type. Rejected because it's harder to compose and doesn't leverage Go's channel-based concurrency.

---

## Why Provider Is an Interface, Not a Struct

### Decision
`Provider` is defined as an interface that all implementations must satisfy, rather than a concrete struct with provider-specific logic.

### Rationale

**Pluggability**: New providers can be added without modifying core code. Third parties can implement custom providers without forking the SDK.

**Testability**: Tests can use mock providers that implement the interface. No need to make real API calls in unit tests.

**Provider Isolation**: Each provider's implementation details (authentication, request formats, error mapping) are encapsulated. Core code never imports provider packages.

**Go Idiom**: Go favors small interfaces. The five-method Provider interface is minimal yet complete for chat functionality.

### Interface Design

```go
type Provider interface {
    ID() string
    Models() []ModelInfo
    Supports(feature Feature) bool
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    StreamChat(ctx context.Context, req *ChatRequest) (*ChatStream, error)
}
```

**Why These Five Methods?**

- `ID()`: Identifies the provider for telemetry and debugging
- `Models()`: Enables discovery of available models and their capabilities
- `Supports(feature)`: Runtime feature detection without reflection
- `Chat()`: Synchronous request for simple use cases
- `StreamChat()`: Streaming request for real-time output

### Optional Interfaces

Additional capabilities use separate interfaces:

```go
type ImageGenerator interface {
    GenerateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error)
    EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error)
    StreamImage(ctx context.Context, req *ImageGenerateRequest) (*ImageStream, error)
}
```

This allows providers to opt into capabilities without polluting the core interface.

### Alternatives Considered

**Single Struct with Provider Field**: A monolithic struct with a `providerType` field and switch statements. Rejected because it would require modifying core code for each new provider.

**Configuration-Based Providers**: Pass a configuration struct to a generic provider. Rejected because providers have fundamentally different APIs, authentication mechanisms, and response formats.

---

## Why ChatBuilder Is Not Thread-Safe

### Decision
`ChatBuilder` explicitly documents that it is NOT thread-safe and should not be shared across goroutines.

### Rationale

**Performance**: Synchronization (mutexes, atomic operations) has overhead. Since builders are typically used for a single request, this overhead provides no benefit.

**Go Idiom**: Go prefers explicit sharing over implicit synchronization. The pattern of "create, configure, execute" doesn't require concurrent access to the builder.

**Simplicity**: Thread-safe builders would need careful handling of slice appends, pointer assignments, and the timeout field. This complexity would obscure the simple builder pattern.

**Copy Semantics**: The `Clone()` method provides explicit copying for concurrent use cases. This is more efficient than synchronization when parallelism is actually needed.

### Safe Usage Pattern

```go
// Thread-safe: Clone before goroutine
base := client.Chat(model).System("You are helpful").Temperature(0.7)

go func() {
    resp1, _ := base.Clone().User("Question 1").GetResponse(ctx)
}()

go func() {
    resp2, _ := base.Clone().User("Question 2").GetResponse(ctx)
}()
```

### What Is Thread-Safe?

- `Client`: Safe for concurrent use (immutable after creation)
- `Provider`: SHOULD be safe (documented per-provider)
- `ChatStream`: Channels safe to read from one goroutine

### Alternatives Considered

**Always Thread-Safe**: Add mutex protection to ChatBuilder. Rejected due to performance overhead and complexity for the common single-threaded case.

**Builder Pool**: Pool and reuse builders with synchronization. Rejected because builders are cheap to allocate and the pooling logic adds complexity.

---

## Why Tools Use json.RawMessage

### Decision
Tool arguments and schemas use `json.RawMessage` instead of parsed Go types.

```go
type ToolCall struct {
    ID        string          `json:"id"`
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments"`
}

type ToolSchema struct {
    JSONSchema json.RawMessage `json:"json_schema"`
}
```

### Rationale

**Preserves Raw JSON**: Different providers may return semantically equivalent but syntactically different JSON. `json.RawMessage` preserves the exact bytes the model returned, avoiding reformatting issues.

**Deferred Parsing**: The core SDK doesn't need to understand tool argument structure. Parsing is deferred to the tool implementation, which knows its expected schema.

**Flexibility**: Users can define any JSON schema. Using `map[string]any` or struct types would limit expressiveness or require reflection.

**Round-Trip Safety**: When passing tool results back to the model, the exact JSON is preserved. Some models are sensitive to whitespace or key ordering changes.

### Usage Pattern

```go
// In tool implementation
func (t *WeatherTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
    var params struct {
        Location string `json:"location"`
        Units    string `json:"units"`
    }
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, err
    }
    return t.getWeather(params.Location, params.Units)
}
```

### Alternatives Considered

**map[string]any**: Loses type information and requires type assertions everywhere.

**Struct per Tool**: Would require code generation or reflection to handle arbitrary schemas.

**Generic Type Parameter**: `ToolCall[T any]` would complicate the ChatResponse type and require users to know types at compile time.

---

## Why Sentinel Errors with ProviderError

### Decision
The SDK defines sentinel errors (`ErrRateLimited`, `ErrUnauthorized`, etc.) AND a `ProviderError` struct with full context.

```go
var ErrRateLimited = errors.New("rate limited")

type ProviderError struct {
    Provider  string
    Status    int
    RequestID string
    Code      string
    Message   string
    Err       error // Wraps sentinel error
}
```

### Rationale

**Classification**: Sentinel errors enable simple classification with `errors.Is()`:
```go
if errors.Is(err, core.ErrRateLimited) {
    // Handle rate limiting
}
```

**Full Context**: `ProviderError` provides debugging information (request ID, status code, provider-specific error codes) without forcing callers to parse error messages.

**Error Chaining**: `ProviderError.Unwrap()` returns the sentinel, supporting both classification and context extraction:
```go
var pe *core.ProviderError
if errors.As(err, &pe) {
    log.Printf("Provider %s returned %d: %s", pe.Provider, pe.Status, pe.Message)
}
```

### Retry Integration

The retry policy uses sentinel errors to determine retryability:

```go
func isRetryable(err error) bool {
    if errors.Is(err, ErrUnauthorized) { return false }  // Don't retry auth errors
    if errors.Is(err, ErrRateLimited) { return true }    // Retry rate limits
    if errors.Is(err, ErrServer) { return true }         // Retry server errors
    // ...
}
```

---

## Why Exponential Backoff with Jitter

### Decision
The default retry policy uses exponential backoff with configurable jitter.

### Rationale

**Thundering Herd Prevention**: Without jitter, all clients retry at the same time after a rate limit, causing another spike. Jitter spreads retries over time.

**Backoff Growth**: Exponential growth (1s, 2s, 4s, 8s...) quickly reaches meaningful delays while starting with fast initial retries.

**Configurability**: The `RetryConfig` allows tuning for different use cases:
```go
type RetryConfig struct {
    MaxRetries int           // Maximum retry attempts
    BaseDelay  time.Duration // Initial delay
    MaxDelay   time.Duration // Delay cap
    Jitter     float64       // Randomization factor (0.0-1.0)
}
```

### Formula

```
delay = baseDelay * 2^attempt * (1 + random(-jitter, +jitter))
delay = min(delay, maxDelay)
```

---

## Why Features Are Explicit, Not Implicit

### Decision
Providers explicitly declare capabilities via `Supports(feature Feature)` rather than using reflection or interface assertions.

### Rationale

**Model-Level Granularity**: Within a single provider, different models may support different features. `Supports()` can check model-specific capabilities at runtime.

**No Reflection**: Reflection is slow and opaque. Explicit feature flags are fast and debuggable.

**Discovery**: Users can query capabilities before making requests:
```go
for _, model := range provider.Models() {
    if model.HasCapability(core.FeatureReasoning) {
        fmt.Printf("%s supports reasoning\n", model.ID)
    }
}
```

**Documentation**: Feature constants serve as documentation. New capabilities are added as new constants, making API evolution explicit.

---

## Summary of Design Principles

1. **Strong Typing Over Reflection**: Prefer explicit Go types and interfaces
2. **Provider Isolation**: All provider-specific logic stays in provider packages
3. **Streaming-First**: Streaming is a core primitive, not an afterthought
4. **Concurrency Safety**: Clear documentation of what's safe to share
5. **Minimal Magic**: No global singletons, no hidden goroutines
6. **Pluggability**: Interfaces enable extension without modification
7. **Error Context**: Rich errors with classification and debugging info
