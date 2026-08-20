# Timeouts and Warning Hooks

How Iris bounds request duration and how to route non-fatal SDK warnings into
your own logger.

## Execution Timeouts

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

## Warning Hooks

Route non-fatal SDK warnings (for example, mismatched tool result IDs or unsupported multimodal parts) into your application logger:

```go
client := core.NewClient(provider,
    core.WithWarningHandler(func(msg string) {
        log.Printf("iris warning: %s", msg)
    }),
)
```

---

See also: [Default Execution Timeout (rationale)](../ARCHITECTURE.md#default-execution-timeout) · [Documentation index](../README.md)
