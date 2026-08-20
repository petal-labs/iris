# Conversation Management

`core.Conversation` keeps message history for you, including assistant tool
calls, across both non-streaming and streaming turns.

The `Conversation` type manages message history automatically:

```go
// Create a conversation with a system prompt
ctx := context.Background()
conv := core.NewConversation(ctx, client, "gpt-5.6",
    core.WithSystemMessage("You are a helpful assistant."),
)

// Send messages - history is managed automatically
resp1, _ := conv.Send(ctx, "What is Go?")
resp2, _ := conv.Send(ctx, "What are its main features?") // Remembers context

// Streaming responses
stream, _ := conv.Stream(ctx, "Tell me more about concurrency")
for chunk := range stream.Ch {
    fmt.Print(chunk.Delta)
}
```

The same context is propagated through the provider request and every `Memory`
operation. Assistant tool calls are retained in history; after executing them,
append their results before continuing:

```go
resp, _ := conv.Send(ctx, "What's the weather in Tokyo?")
results := executeTools(resp.ToolCalls)
conv.AddToolResults(ctx, results)
resp, _ = conv.Send(ctx, "Summarize the result.")
```

---

See also: [Tools and Function Calling](tools.md) · [Documentation index](../README.md)
