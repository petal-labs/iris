# Responses API (GPT-5+)

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

---

See also: [Provider Quick Starts](provider-quickstarts.md) · [Documentation index](../README.md)
