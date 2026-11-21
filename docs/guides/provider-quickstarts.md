# Provider Quick Starts

Runnable examples for each provider Iris supports. Every provider is constructed
the same way — build a provider, wrap it in a `core.Client`, then use the fluent
chat builder — so switching providers is a two-line change.

Ollama's quick start lives in the [root README](../../README.md#using-ollama)
because it needs no API key. For the full feature matrix, model catalogs, and
per-provider endpoint/auth details, see [Provider Comparison](../PROVIDERS.md).

## Using Anthropic Claude

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

## Using Google Gemini

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
if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
    fmt.Println("Thinking:", resp.Reasoning.Summary[0])
}
```

## Using xAI Grok

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

    // Send a chat request using Grok 4.7
    resp, err := client.Chat(xai.ModelGrok47).
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
// Use reasoning with grok-4.7
resp, err := client.Chat(xai.ModelGrok47).
    User("Solve this step by step: If I have 5 apples and give away half...").
    ReasoningEffort(core.ReasoningEffortHigh).
    GetResponse(ctx)

// Access reasoning if available
if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
    fmt.Println("Thinking:", resp.Reasoning.Summary[0])
}
```

## Using Z.ai GLM

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

## Using Perplexity Search

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

### Search Options and Citations

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

## Using ElevenLabs (Speech & Voice)

ElevenLabs is a speech synthesis, transcription, and real-time voice provider (`Chat` and `StreamChat` return `core.ErrNotSupported`). Wrap `elevenlabs.NewFromEnv()` in `core.Client` for TTS/STT or pair it with any Iris chat LLM inside `voice.Pipeline`:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/petal-labs/iris/core"
    "github.com/petal-labs/iris/providers/elevenlabs"
)

func main() {
    provider, err := elevenlabs.NewFromEnv()
    if err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }
    client := core.NewClient(provider)

    // Text-to-speech synthesis
    speech, err := client.Synthesize(context.Background(), &core.SpeechRequest{
        Model:   elevenlabs.ModelElevenFlashV25,
        VoiceID: "JBFqnCBsd6RMkjVDRZzb", // public "George" voice
        Text:    "Hello from Iris and ElevenLabs.",
        Format:  core.AudioFormatMP3_44100_128,
    })
    if err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }

    // Transcribe the generated audio back with Scribe v2
    transcript, err := client.Transcribe(context.Background(), &core.TranscriptionRequest{
        Model: elevenlabs.ModelScribeV2,
        Audio: &speech.Audio,
    })
    if err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }

    fmt.Println(transcript.Text)
}
```

For real-time WebSocket STT/TTS, full-duplex `voice.Pipeline` agents, Speech Engine upstream servers, hosted ElevenAgents sessions, voice discovery, and opt-in voice management, see the [Speech and Audio Guide](speech.md) and the runnable examples in [`examples/speech/`](../../examples/README.md) and [`examples/voice/`](../../examples/README.md).

---

See also: [Provider Comparison](../PROVIDERS.md) · [Speech and Audio Guide](speech.md) · [Documentation index](../README.md)
