# Provider Comparison

This document provides a comprehensive comparison of the AI providers supported by Iris.
Static model catalogs mirror each provider's `Models()` implementation in `providers/<name>/models.go`.
Ollama is the exception: its catalog is discovered from the configured instance at runtime.

## Feature Support Matrix

Cells reflect each provider's `Supports()` implementation. For chat-based features, availability can also vary by model — see the model-specific tables below.

| Provider | Chat | Streaming | Tool Calling | Reasoning | Built-in Tools | Response Chain | Token Counting | Structured Output | Embeddings | Reranking | Images |
|----------|------|-----------|--------------|-----------|----------------|----------------|----------------|--------------------|------------|-----------|--------|
| OpenAI | Yes | Yes | Yes | Yes* | Yes* | Yes* | No | Yes | Yes | No | Yes |
| Anthropic | Yes | Yes | Yes | Yes | No | No | Yes | No† | No | No | No |
| Gemini | Yes | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | Yes |
| xAI (Grok) | Yes | Yes | Yes | Yes | No | No | No | No† | No | No | No |
| Perplexity | Yes | Yes | Yes | Yes | No | No | No | No† | No | No | No |
| Z.ai (GLM) | Yes | Yes | Yes | Yes | No | No | No | No† | No | No | No |
| Ollama | Yes | Yes | Yes | Yes | No | No | No | No† | Yes | No | No |
| HuggingFace | Yes | Yes | Yes | No | No | No | No | No† | No | No | No |
| Azure AI Foundry | Yes | Yes | Yes | Yes | No | No | No | Yes | Yes | No | No |
| VoyageAI | No | No | No | No | No | No | No | N/A | Yes | Yes | No |

Perplexity additionally supports web-search grounding (`core.SearchOptions`, response citations) via `core.FeatureWebSearch`.

*Feature availability varies by model. See model-specific tables below.

†"No" means a `ResponseJSONSchema` request against this provider fails fast with `core.ErrStructuredOutputUnsupported` before being sent, rather than silently ignoring the schema (the prior behavior). Plain `ResponseJSON()` (`json_object` mode) is unaffected and is not gated by this check.

## Supported Providers

| Provider | Status | Features |
|----------|--------|----------|
| OpenAI | Supported | Chat, Streaming, Tools, Reasoning, Batch API, Structured Output, Embeddings, Images, Responses API (GPT-5+) |
| Anthropic | Supported | Chat, Streaming, Tools, Reasoning, Token Counting |
| Google Gemini | Supported | Chat, Streaming, Tools, Reasoning, Token Counting, Structured Output, Embeddings, Images |
| xAI Grok | Supported | Chat, Streaming, Tools, Reasoning |
| Z.ai GLM | Supported | Chat, Streaming, Tools, Thinking |
| Perplexity | Supported | Chat, Streaming, Tools, Reasoning, Web Search + Citations |
| Ollama | Supported | Chat, Streaming, Tools, Thinking, Embeddings |
| Hugging Face | Supported | Chat, Streaming, Tools (Inference Providers router, model discovery) |
| Azure AI Foundry | Supported | Chat, Streaming, Tools, Reasoning, Structured Output, Embeddings (Entra ID auth) |
| Voyage AI | Supported | Embeddings, Contextualized Embeddings, Reranking (no chat) |

## Provider Details

### OpenAI

**API Endpoint**: `https://api.openai.com/v1`

**Authentication**: API key via `OPENAI_API_KEY` environment variable

**API Types**: the provider picks the route per model, from each model's
`APIEndpoint`. The Route column below records which one applies.
- Chat Completions API — GPT-4o, GPT-4, and GPT-3.5 series
- Responses API — GPT-5.x, GPT-4.1, and o-series models; required for reasoning,
  built-in tools, and response chaining

**Models**:

| Model | Display Name | Route | Reasoning | Built-in Tools | Notes |
|-------|--------------|-------|-----------|----------------|-------|
| gpt-5.6 | GPT-5.6 | Responses | Yes | Yes | Latest flagship |
| gpt-5.6-luna | GPT-5.6 Luna | Responses | Yes | Yes | GPT-5.6 variant |
| gpt-5.6-sol | GPT-5.6 Sol | Responses | Yes | Yes | GPT-5.6 variant |
| gpt-5.6-terra | GPT-5.6 Terra | Responses | Yes | Yes | GPT-5.6 variant |
| gpt-5.5 | GPT-5.5 | Responses | Yes | Yes |  |
| gpt-5.5-pro | GPT-5.5 Pro | Responses | Yes | Yes | Enhanced capabilities |
| gpt-5.3-codex | GPT-5.3 Codex | Responses | Yes | Yes | Code specialized |
| gpt-5.3-codex-spark | GPT-5.3 Codex Spark | Responses | Yes | Yes | Code specialized, lightweight |
| gpt-5.4 | GPT-5.4 | Responses | Yes | Yes |  |
| gpt-5.4-pro | GPT-5.4 Pro | Responses | Yes | Yes | Enhanced capabilities |
| gpt-5.4-mini | GPT-5.4 Mini | Responses | Yes | Yes | Smaller, faster |
| gpt-5.4-nano | GPT-5.4 Nano | Responses | Yes | Yes | Lightweight |
| gpt-5.2 | GPT-5.2 | Responses | Yes | Yes |  |
| gpt-5.2-pro | GPT-5.2 Pro | Responses | Yes | Yes | Enhanced capabilities |
| gpt-5.2-codex | GPT-5.2 Codex | Responses | Yes | Yes | Code specialized |
| gpt-5.1 | GPT-5.1 | Responses | Yes | Yes |  |
| gpt-5.1-codex | GPT-5.1 Codex | Responses | Yes | Yes | Code specialized |
| gpt-5.1-codex-mini | GPT-5.1 Codex Mini | Responses | Yes | Yes | Smaller codex |
| gpt-5.1-codex-max | GPT-5.1 Codex Max | Responses | Yes | Yes | Largest codex |
| gpt-5 | GPT-5 | Responses | Yes | Yes |  |
| gpt-5-mini | GPT-5 Mini | Responses | Yes | Yes | Smaller, faster |
| gpt-5-nano | GPT-5 Nano | Responses | No | Yes | Lightweight |
| gpt-5-pro | GPT-5 Pro | Responses | Yes | Yes | Enhanced capabilities |
| gpt-5-codex | GPT-5 Codex | Responses | Yes | Yes | Code specialized |
| gpt-5-thinking | GPT-5 Thinking | Responses | Yes | Yes | Extended reasoning |
| gpt-4.1 | GPT-4.1 | Responses | No | Yes |  |
| gpt-4.1-mini | GPT-4.1 Mini | Responses | No | Yes |  |
| gpt-4.1-nano | GPT-4.1 Nano | Responses | No | Yes |  |
| gpt-4o | GPT-4o | Chat Completions | No | No | Multimodal |
| gpt-4o-mini | GPT-4o Mini | Chat Completions | No | No | Cost-effective |
| gpt-4-turbo | GPT-4 Turbo | Chat Completions | No | No | Legacy |
| gpt-4 | GPT-4 | Chat Completions | No | No | Legacy |
| gpt-3.5-turbo | GPT-3.5 Turbo | Chat Completions | No | No | Legacy; no structured output |
| gpt-3.5-turbo-16k | GPT-3.5 Turbo 16k | Chat Completions | No | No | Legacy, 16K context |
| gpt-3.5-turbo-instruct | GPT-3.5 Turbo Instruct | Chat Completions | No | No | Legacy; no tool calling |
| o4-mini | o4-mini | Responses | Yes | Yes | Reasoning focused |
| o4-mini-deep-research | o4-mini Deep Research | Responses | Yes | Yes | Research focused |
| o3 | o3 | Responses | Yes | Yes | Reasoning focused |
| o3-pro | o3-pro | Responses | Yes | Yes | Enhanced reasoning |
| o3-mini | o3-mini | Responses | Yes | Yes | Smaller reasoning |
| o1 | o1 | Responses | Yes | No | Reasoning focused |
| o1-pro | o1 Pro | Responses | Yes | No | Enhanced reasoning |

**Image Generation Models**: gpt-image-2, gpt-image-1.5, gpt-image-1, gpt-image-1-mini, dall-e-3, dall-e-2, chatgpt-image-latest

**Structured Output**: `ResponseJSONSchema` works on both the Chat Completions API and the Responses API (GPT-5.x), mapped to each API's native `response_format`/`text.format` shape respectively. Requesting it against a Chat Completions-only model that lacks `core.FeatureStructuredOutput` (e.g. gpt-3.5-turbo) fails with `core.ErrStructuredOutputUnsupported`.

**Multimodal Input**: Image URLs and base64 data URLs are mapped on both Chat Completions and Responses routes. Responses-routed models also accept image file IDs and document inputs.

**Usage Example**:
```go
provider := openai.New(os.Getenv("OPENAI_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(openai.ModelGPT4o).
    User("Hello!").
    GetResponse(ctx)
```

---

### Anthropic

**API Endpoint**: `https://api.anthropic.com/v1`

**Authentication**: API key via `ANTHROPIC_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Notes |
|-------|--------------|-----------|-------|
| claude-opus-5 | Claude Opus 5 | Yes | Latest flagship |
| claude-opus-5-thinking | Claude Opus 5 (Thinking) | Yes | Extended reasoning |
| claude-sonnet-5 | Claude Sonnet 5 | Yes | Balanced performance |
| claude-sonnet-5-thinking | Claude Sonnet 5 (Thinking) | Yes | Extended reasoning |
| claude-fable-5 | Claude Fable 5 | Yes |  |
| claude-opus-4-8 | Claude Opus 4.8 | Yes | High capability |
| claude-opus-4-8-thinking | Claude Opus 4.8 (Thinking) | Yes | Extended reasoning |
| claude-opus-4-7 | Claude Opus 4.7 | Yes | High capability |
| claude-sonnet-4-6 | Claude Sonnet 4.6 | Yes | Balanced performance |
| claude-sonnet-4-6-thinking | Claude Sonnet 4.6 (Thinking) | Yes | Extended reasoning |
| claude-opus-4-6 | Claude Opus 4.6 | Yes | High capability |
| claude-opus-4-6-thinking | Claude Opus 4.6 (Thinking) | Yes | Extended reasoning |
| claude-sonnet-4-5 | Claude Sonnet 4.5 | Yes | Balanced performance |
| claude-sonnet-4-5-thinking | Claude Sonnet 4.5 (Thinking) | Yes | Extended reasoning |
| claude-haiku-4-5 | Claude Haiku 4.5 | Yes | Fast, cost-effective |
| claude-opus-4-5 | Claude Opus 4.5 | Yes | High capability |
| claude-opus-4-5-thinking | Claude Opus 4.5 (Thinking) | Yes | Extended reasoning |
| claude-3-5-haiku-latest | Claude 3.5 Haiku | No | Legacy fast model |

**Special Features**:
- Extended context windows
- Strong instruction following
- Built-in safety guardrails
- Thinking/reasoning modes
- Vision input via hosted image URLs or base64 data URLs
- Native input-token counting through `POST /v1/messages/count_tokens`

**Usage Example**:
```go
provider := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(anthropic.ModelClaudeSonnet46).
    System("You are a helpful assistant.").
    User("Explain quantum computing.").
    GetResponse(ctx)

counter, _ := core.AsTokenCounter(provider)
count, err := counter.CountTokens(ctx, &core.ChatRequest{
    Model: anthropic.ModelClaudeSonnet46,
    Messages: []core.Message{{Role: core.RoleUser, Content: "Explain quantum computing."}},
})
fmt.Println(count.InputTokens)
```

---

### Google Gemini

**API Endpoint**: `https://generativelanguage.googleapis.com/v1beta`

**Authentication**: API key via `GEMINI_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Tool Calling | Notes |
|-------|--------------|-----------|--------------|-------|
| gemini-3.6-flash | Gemini 3.6 Flash | Yes | Yes | Latest (`thinkingLevel`) |
| gemini-3.5-flash | Gemini 3.5 Flash | Yes | Yes | `thinkingLevel` |
| gemini-3.5-flash-lite | Gemini 3.5 Flash Lite | Yes | Yes | `thinkingLevel`, lightweight |
| gemini-3.1-pro-preview | Gemini 3.1 Pro Preview | Yes | Yes | `thinkingLevel` |
| gemini-3.1-flash-lite | Gemini 3.1 Flash Lite | Yes | Yes | `thinkingLevel`, lightweight |
| gemini-3.1-flash-image-preview | Gemini 3.1 Flash Image Preview | No | No | Chat, streaming, image generation |
| gemini-3-pro-preview | Gemini 3 Pro Preview | Yes | Yes | `thinkingLevel` |
| gemini-3-flash-preview | Gemini 3 Flash Preview | Yes | Yes | `thinkingLevel` |
| gemini-3-pro-image-preview | Gemini 3 Pro Image Preview | No | No | Image generation only (Nano Banana Pro) |
| gemini-2.5-pro | Gemini 2.5 Pro | Yes | Yes | `thinkingBudget`, production ready |
| gemini-2.5-flash | Gemini 2.5 Flash | Yes | Yes | `thinkingBudget`, fast |
| gemini-2.5-flash-lite | Gemini 2.5 Flash Lite | Yes | Yes | `thinkingBudget`, lightweight |
| gemini-2.5-flash-image | Gemini 2.5 Flash Image | No | No | Image generation only (Nano Banana) |
| gemini-2.0-flash-lite | Gemini 2.0 Flash Lite | No | No | Legacy lightweight |

**Image Generation Models**: gemini-3.1-flash-image-preview, gemini-3-pro-image-preview, gemini-2.5-flash-image (Nano Banana)

**Embedding Models**:

| Model | Display Name | Input Types | Notes |
|-------|--------------|-------------|-------|
| gemini-embedding-001 | Gemini Embedding 001 | Query, Document | Configurable output dimensions |

**Special Features**:
- Native multimodal support
- Long context windows
- Grounding with Google Search
- Batch text embeddings with query/document retrieval optimization
- Native input-token counting through `POST /v1beta/models/{model}:countTokens`

**Usage Example**:
```go
provider := gemini.New(os.Getenv("GEMINI_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(gemini.ModelGemini25Flash).
    User("Summarize this document.").
    GetResponse(ctx)

embeddingProvider := gemini.New(os.Getenv("GEMINI_API_KEY"))
embeddingResp, err := embeddingProvider.CreateEmbeddings(ctx, &core.EmbeddingRequest{
    Model:     gemini.ModelGeminiEmbedding001,
    Input:     []core.EmbeddingInput{{Text: "A document to index"}},
    InputType: core.InputTypeDocument,
})
```

---

### xAI (Grok)

**API Endpoint**: `https://api.x.ai/v1`

**Authentication**: API key via `XAI_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Notes |
|-------|--------------|-----------|-------|
| grok-4.5 | Grok 4.5 | Yes | Latest |
| grok-4.3 | Grok 4.3 | Yes | |
| grok-4.20-multi-agent-beta-0309 | Grok 4.20 Multi-Agent Beta | Yes | Multi-agent beta |
| grok-4.20-beta-0309-reasoning | Grok 4.20 Beta (Reasoning) | Yes | Beta with reasoning |
| grok-4.20-beta-0309-non-reasoning | Grok 4.20 Beta (Non-Reasoning) | No | Beta without reasoning |
| grok-4.1 | Grok 4.1 | No | |
| grok-4-1-fast-non-reasoning | Grok 4.1 Fast (Non-Reasoning) | No | Fast; default model in `iris init` scaffolds |
| grok-4-1-fast-reasoning | Grok 4.1 Fast (Reasoning) | Yes | Fast with reasoning |
| grok-4 | Grok 4 | Yes | Flagship |
| grok-4-fast-reasoning | Grok 4 Fast (Reasoning) | Yes | Fast with reasoning |
| grok-4-fast-non-reasoning | Grok 4 Fast (Non-Reasoning) | No | Fast without reasoning |
| grok-3 | Grok 3 | Yes | Previous generation |
| grok-3-mini | Grok 3 Mini | Yes | Smaller model; exposes `reasoning_content` |
| grok-code-fast | Grok Code Fast | No | Code specialized |
| grok-build-0.1 | Grok Build 0.1 | Yes | Build / agentic workloads |

**Special Features**:
- Real-time information access
- Distinct reasoning modes
- Multi-agent capabilities (beta)

**Usage Example**:
```go
provider := xai.New(os.Getenv("XAI_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(xai.ModelGrok4).
    User("What's happening in tech today?").
    GetResponse(ctx)
```

---

### Perplexity

**API Endpoint**: `https://api.perplexity.ai`

**Authentication**: API key via `PERPLEXITY_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Tool Calling | Notes |
|-------|--------------|-----------|--------------|-------|
| sonar | Sonar | No | Yes | Fast search |
| sonar-pro | Sonar Pro | No | Yes | Enhanced search |
| sonar-reasoning-pro | Sonar Reasoning Pro | Yes | Yes | Chain of thought |
| sonar-deep-research | Sonar Deep Research | Yes | No | Comprehensive research |

**Special Features**:
- Built-in web search and grounding
- Search controls via `core.SearchOptions` (domain filter with `-` exclusion, recency, search mode) — set with `ChatBuilder.SearchOptions`
- Citation support: response and streamed-final `Citations []string` on `core.ChatResponse`
- Research report generation

**Usage Example**:
```go
provider := perplexity.New(os.Getenv("PERPLEXITY_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(perplexity.ModelSonarPro).
    User("What are the latest developments in AI?").
    SearchOptions(&core.SearchOptions{
        SearchDomainFilter: []string{"arxiv.org"},
        Recency:            core.SearchRecencyMonth,
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

---

### Z.ai (GLM)

**API Endpoint**: `https://open.bigmodel.cn/api/paas/v4`

**Authentication**: API key via `ZAI_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Vision | Notes |
|-------|--------------|-----------|--------|-------|
| glm-5.2 | GLM-5.2 | Yes | No | Latest flagship |
| glm-5.1 | GLM-5.1 | Yes | No | |
| glm-5 | GLM-5 | Yes | No | |
| glm-5-turbo | GLM-5 Turbo | Yes | No | Fast |
| glm-5v-turbo | GLM-5V Turbo | Yes | Yes | Vision capable |
| glm-4.7 | GLM-4.7 | Yes | No | |
| glm-4.7-flash | GLM-4.7 Flash | No | No | Fast; default model in `iris init` scaffolds |
| glm-4.7-flashx | GLM-4.7 FlashX | No | No | Extra fast |
| glm-4.6 | GLM-4.6 | Yes | No | |
| glm-4.6v | GLM-4.6V | Yes | Yes | Vision capable |
| glm-4.6v-flash | GLM-4.6V Flash | No | Yes | Fast vision |
| glm-4.6v-flashx | GLM-4.6V FlashX | No | Yes | Extra fast vision |
| glm-4.5 | GLM-4.5 | Yes | No | |
| glm-4.5v | GLM-4.5V | Yes | Yes | Vision capable |
| glm-4.5-x | GLM-4.5-X | No | No | Extended |
| glm-4.5-air | GLM-4.5 Air | No | No | Lightweight |
| glm-4.5-airx | GLM-4.5 AirX | No | No | Extra lightweight |
| glm-4.5-flash | GLM-4.5 Flash | No | No | Fast |
| glm-for-coding | GLM for Coding | Yes | No | Code specialized |
| glm-4-32b-0414-128k | GLM-4 32B | No | No | 128K context |

**Special Features**:
- Vision models for image understanding
- Chinese language optimization
- Large context windows
- Code-specialized model

**Usage Example**:
```go
provider := zai.New(os.Getenv("ZAI_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(zai.ModelGLM47).
    User("Explain machine learning.").
    GetResponse(ctx)
```

---

### Ollama

**API Endpoint**: `http://localhost:11434` (default local) or `https://ollama.com` (cloud)

**Authentication**: No key required for local; API key for Ollama Cloud

**Models**: Dynamic — `Models()` queries `GET /api/tags` on the configured
instance and returns the installed model IDs. Use `ListModels(ctx)` when the
caller needs an explicit discovery error. Because the upstream endpoint does
not report per-model capabilities, dynamically discovered `ModelInfo` entries
leave `Capabilities` empty.

If discovery fails, `Models()` returns the illustrative fallback catalog below
after a maximum two-second discovery attempt. A successful empty response stays
empty and does not activate the fallback. See
[ollama.com/library](https://ollama.com/library) for everything else.

| Model | Display Name | Reasoning | Tool Calling |
|-------|--------------|-----------|--------------|
| llama3.2 | Llama 3.2 | No | Yes |
| llama3.2:70b | Llama 3.2 70B | No | Yes |
| mistral | Mistral 7B | No | Yes |
| mixtral | Mixtral 8x7B | No | Yes |
| qwen3 | Qwen 3 | Yes | Yes |
| gemma3 | Gemma 3 | No | No |
| deepseek-coder | DeepSeek Coder | No | No |
| codellama | Code Llama | No | No |

**Special Features**:
- Local-first operation
- No API costs for local usage
- Custom model support
- Thinking/reasoning mode for supported models

**Usage Example**:
```go
// Local usage (no API key)
provider := ollama.New()

// Remote instance
provider := ollama.New(ollama.WithBaseURL("http://remote:11434"))

// Ollama Cloud
provider := ollama.New(
    ollama.WithCloud(),
    ollama.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
)

models, err := provider.ListModels(ctx) // explicit /api/tags discovery

client := core.NewClient(provider)
resp, err := client.Chat("llama3.2").
    User("Hello!").
    GetResponse(ctx)
```

---

### HuggingFace

**API Endpoint**: `https://router.huggingface.co/v1`

**Authentication**: HuggingFace token with Inference Providers permission

**Models**: Access to thousands of models across multiple inference providers.

**Provider Routing**:
- `:fastest` - Routes to highest throughput provider
- `:cheapest` - Routes to lowest cost provider
- `:provider-name` - Routes to specific provider (cerebras, together, etc.)

**Special Features**:
- Multi-provider routing
- Model discovery API
- Provider status checking

**Usage Example**:
```go
provider := huggingface.New(os.Getenv("HF_TOKEN"),
    huggingface.WithProviderPolicy("fastest"),
)
client := core.NewClient(provider)

resp, err := client.Chat("meta-llama/Llama-3-8B-Instruct").
    User("Hello!").
    GetResponse(ctx)
```

---

### VoyageAI

**API Endpoint**: `https://api.voyageai.com/v1`

**Authentication**: API key via `VOYAGE_API_KEY` environment variable

**Note**: VoyageAI is an embeddings and reranking provider. It does not support chat completions.

**Embedding Models**:

| Model | Display Name | Notes |
|-------|--------------|-------|
| voyage-4-large | Voyage 4 Large | Highest quality |
| voyage-4 | Voyage 4 | Balanced |
| voyage-4-lite | Voyage 4 Lite | Lightweight |
| voyage-3.5 | Voyage 3.5 | |
| voyage-3-large | Voyage 3 Large | |
| voyage-code-3 | Voyage Code 3 | Code specialized |
| voyage-finance-2 | Voyage Finance 2 | Finance domain |
| voyage-law-2 | Voyage Law 2 | Legal domain |
| voyage-context-3 | Voyage Context 3 | Contextualized embeddings |

**Reranker Models**:

| Model | Display Name |
|-------|--------------|
| rerank-2.5 | Rerank 2.5 |
| rerank-2.5-lite | Rerank 2.5 Lite |
| rerank-2 | Rerank 2 |
| rerank-2-lite | Rerank 2 Lite |

**Usage Example**:
```go
provider := voyageai.New(os.Getenv("VOYAGE_API_KEY"))

// Generate embeddings
embeddings, err := provider.Embed(ctx, &core.EmbeddingRequest{
    Model: voyageai.ModelVoyage4,
    Input: []string{"Hello, world!"},
})

// Rerank results
results, err := provider.Rerank(ctx, &core.RerankRequest{
    Model:     voyageai.ModelRerank25,
    Query:     "machine learning",
    Documents: []string{"doc1", "doc2", "doc3"},
})
```

### Azure AI Foundry

**API Endpoint**: `https://{resource}.services.ai.azure.com/models` (Model Inference API) or `https://{resource}.openai.azure.com` (Azure OpenAI Service, via `WithOpenAIEndpoint()`)

**Authentication**: API key or Microsoft Entra ID (Azure AD) credential. Via environment: `AZURE_AI_ENDPOINT`, `AZURE_AI_API_KEY`, and optional `AZURE_AI_DEPLOYMENT_ID` (required with `WithOpenAIEndpoint()`).

**Models**: Multi-family catalog — OpenAI (GPT), Meta Llama, Mistral, Cohere, and more — including embeddings models. Use `provider.Models()` or `azurefoundry.GetModelInfo(id)` to inspect the static catalog.

**Notes**:
- Unlike other Iris providers, `azurefoundry.New` takes `(endpoint, apiKey, ...)` — the resource endpoint is mandatory.
- Entra ID auth (managed identity, workload identity, etc.) is available via `NewWithCredential(endpoint, credential)`, where `credential` implements `azurefoundry.TokenCredential`.
- The provider registry entry requires the endpoint to come from `AZURE_AI_ENDPOINT`; prefer `New`/`NewFromEnv` for full configuration.
- Vision-capable deployments accept image URLs and base64 data URLs through `UserMultimodal`; support still depends on the deployed model.

**Usage Example**:
```go
provider, err := azurefoundry.NewFromEnv()
// or: provider := azurefoundry.New("https://my-resource.services.ai.azure.com", apiKey)
if err != nil {
    log.Fatal(err)
}
client := core.NewClient(provider)

resp, err := client.Chat("gpt-5.6"). // any deployed model
    User("Hello").
    GetResponse(ctx)
```

## Choosing a Provider

| Use Case | Recommended Provider(s) |
|----------|------------------------|
| General chat and coding | OpenAI (GPT-4o, GPT-5), Anthropic (Claude) |
| Complex reasoning | OpenAI (o-series), Gemini 3, xAI Grok 4 |
| Web search integration | Perplexity |
| Local/private deployment | Ollama |
| Cost-sensitive applications | HuggingFace (routing), Ollama (local) |
| Embeddings and RAG | Gemini, VoyageAI |
| Code generation | OpenAI (Codex models), Anthropic |
| Multimodal (vision) | OpenAI (GPT-4o), Anthropic (Claude), Gemini, Azure AI Foundry vision deployments |
| Image generation | OpenAI (DALL-E, GPT-Image), Gemini (Nano Banana) |

Iris currently maps `Message.Parts` for OpenAI, Anthropic, Gemini, and Azure AI Foundry. Other providers invoke `core.WithWarningHandler` before omitting undeclared content parts, even when an upstream model advertises vision support.

## Rate Limits and Pricing

Rate limits and pricing vary by provider and subscription tier. Consult each provider's documentation for current information:

- **OpenAI**: https://platform.openai.com/docs/guides/rate-limits
- **Anthropic**: https://docs.anthropic.com/en/api/rate-limits
- **Google Gemini**: https://ai.google.dev/pricing
- **xAI**: https://docs.x.ai/docs
- **Perplexity**: https://docs.perplexity.ai/guides/rate-limits
- **Z.ai**: https://open.bigmodel.cn/pricing
- **Ollama**: No rate limits for local usage
- **HuggingFace**: https://huggingface.co/docs/api-inference/rate-limits
- **VoyageAI**: https://docs.voyageai.com/docs/rate-limits
