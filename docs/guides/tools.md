# Tools and Function Calling

Iris models tools as a small interface rather than a reflection-driven registry,
so the compiler guarantees every tool you register carries its parameters schema
to the provider.

## Defining and Calling Tools

Tools implement the `tools.Tool` interface — `Name()`, `Description()`, and `Schema()` come from `core.Tool`; `Call()` makes the tool executable. Any `tools.Tool` can be passed straight to `Tools(...)`:

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

A minimal tool without execution logic only needs the three `core.Tool` methods (return `core.ToolSchema{}` for a no-parameter tool). The schema is part of the interface, so the compiler guarantees every tool you register carries its parameters schema to the provider.

## Tool Middleware and Validation

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

---

See also: [Why Tools Use json.RawMessage](../ARCHITECTURE.md#why-tools-use-jsonrawmessage) · [Documentation index](../README.md)
