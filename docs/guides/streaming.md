# Streaming Responses

Streaming is a first-class primitive in Iris — every provider implements it.
`Stream` returns a `*core.ChatStream` with three channels: `Ch` for deltas,
`Err` for errors, and `Final` for the completed response.

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

Streaming rationale and the three-channel design are covered in
[Architecture Design Decisions](../ARCHITECTURE.md#why-streaming-is-first-class).

---

See also: [Timeouts and Warning Hooks](timeouts-and-warnings.md) · [Documentation index](../README.md)
