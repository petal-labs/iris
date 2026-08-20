# Batch API

Submit requests for asynchronous processing at reduced cost. Batch support is an
optional provider capability — probe for it with `core.AsBatchProvider`.

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

---

See also: [Provider Comparison](../PROVIDERS.md#feature-support-matrix) · [Documentation index](../README.md)
