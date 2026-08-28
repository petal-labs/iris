# Testing Utilities

The `testing` package provides deterministic stand-ins for real providers so
your tests never make network calls.

```go
import "github.com/petal-labs/iris/testing"

// MockProvider for predictable responses
mock := testing.NewMockProvider().
    WithResponse(core.ChatResponse{Output: "Hello!"}).
    WithResponse(core.ChatResponse{Output: "Follow-up response"})

client := core.NewClient(mock)
resp, _ := client.Chat("any-model").User("Hi").GetResponse(ctx)
// resp.Output == "Hello!"

// Bare mocks support chat-builder features, including JSON schema and search.
// The first WithFeatures call replaces those defaults when a test needs to
// model an unsupported capability.
limitedMock := testing.NewMockProvider().WithFeatures(core.FeatureChat)

// RecordingProvider for capturing interactions
recorder := testing.NewRecordingProvider(realProvider)
client := core.NewClient(recorder)

// ... run your code ...

// Inspect recorded calls
for _, call := range recorder.Recordings() {
    fmt.Printf("Model: %s, Messages: %d\n", call.Request.Model, len(call.Request.Messages))
}

// Optional interfaces remain available through the core discovery helpers.
embedder, ok := core.AsEmbeddingProvider(recorder)
```

`RecordingProvider` records both unary and stream requests. Streaming records
keep the cloned request and timing/error metadata; their `Response` field is
nil because the response is delivered asynchronously through `ChatStream`.
The recorder implements `core.ProviderUnwrapper`, and all `core.As*` helpers
follow nested wrappers while preserving a clean `(nil, false)` result for
unsupported capabilities.

---

See also: [Development Guide](../DEVELOPMENT.md#running-tests) · [Documentation index](../README.md)
