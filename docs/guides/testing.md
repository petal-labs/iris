# Testing Utilities

The `testing` package provides deterministic stand-ins for real providers so
your tests never make network calls.

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

---

See also: [Development Guide](../DEVELOPMENT.md#running-tests) · [Documentation index](../README.md)
