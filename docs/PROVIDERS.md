# Provider Comparison

This document provides a comprehensive comparison of the AI providers supported by Iris.

## Feature Support Matrix

| Provider | Chat | Streaming | Tool Calling | Reasoning | Built-in Tools | Response Chain | Embeddings | Reranking |
|----------|------|-----------|--------------|-----------|----------------|----------------|------------|-----------|
| OpenAI | Yes | Yes | Yes | Yes* | Yes* | Yes* | No | No |
| Anthropic | Yes | Yes | Yes | No | No | No | No | No |
| Gemini | Yes | Yes | Yes | Yes | No | No | No | No |
| xAI (Grok) | Yes | Yes | Yes | Yes* | No | No | No | No |
| Perplexity | Yes | Yes | Yes* | Yes* | No | No | No | No |
| Z.ai (GLM) | Yes | Yes | Yes | Yes* | No | No | No | No |
| Ollama | Yes | Yes | Yes* | Yes* | No | No | No | No |
| HuggingFace | Yes | Yes | Yes | No | No | No | No | No |
| VoyageAI | No | No | No | No | No | No | Yes | Yes |

*Feature availability varies by model. See model-specific tables below.

## Provider Details

### OpenAI

**API Endpoint**: `https://api.openai.com/v1`

**Authentication**: API key via `OPENAI_API_KEY` environment variable

**API Types**:
- Chat Completions API (GPT-4o, GPT-4, GPT-3.5 series)
- Responses API (GPT-5.x, GPT-4.1, o-series models)

**Models**:

| Model | Display Name | Reasoning | Built-in Tools | Notes |
|-------|--------------|-----------|----------------|-------|
| gpt-5.2 | GPT-5.2 | Yes | Yes | Latest flagship |
| gpt-5.2-pro | GPT-5.2 Pro | Yes | Yes | Enhanced capabilities |
| gpt-5.2-codex | GPT-5.2 Codex | Yes | Yes | Code specialized |
| gpt-5.1 | GPT-5.1 | Yes | Yes | |
| gpt-5.1-codex | GPT-5.1 Codex | Yes | Yes | Code specialized |
| gpt-5 | GPT-5 | Yes | Yes | |
| gpt-5-mini | GPT-5 Mini | Yes | Yes | Smaller, faster |
| gpt-5-nano | GPT-5 Nano | No | Yes | Lightweight |
| gpt-4.1 | GPT-4.1 | No | Yes | |
| gpt-4o | GPT-4o | No | No | Multimodal |
| gpt-4o-mini | GPT-4o Mini | No | No | Cost-effective |
| o4-mini | o4-mini | Yes | Yes | Reasoning focused |
| o3 | o3 | Yes | Yes | Reasoning focused |
| o1 | o1 | Yes | No | Reasoning focused |

**Image Generation Models**: gpt-image-1.5, gpt-image-1, dall-e-3, dall-e-2

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

| Model | Display Name | Notes |
|-------|--------------|-------|
| claude-sonnet-4-5 | Claude Sonnet 4.5 | Balanced performance |
| claude-haiku-4-5 | Claude Haiku 4.5 | Fast, cost-effective |
| claude-opus-4-5 | Claude Opus 4.5 | Most capable |

**Special Features**:
- Extended context windows
- Strong instruction following
- Built-in safety guardrails

**Usage Example**:
```go
provider := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(anthropic.ModelClaudeSonnet45).
    System("You are a helpful assistant.").
    User("Explain quantum computing.").
    GetResponse(ctx)
```

---

### Google Gemini

**API Endpoint**: `https://generativelanguage.googleapis.com/v1beta`

**Authentication**: API key via `GEMINI_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Notes |
|-------|--------------|-----------|-------|
| gemini-3-pro-preview | Gemini 3 Pro Preview | Yes | Latest preview |
| gemini-3-flash-preview | Gemini 3 Flash Preview | Yes | Fast preview |
| gemini-2.5-pro | Gemini 2.5 Pro | Yes | Production ready |
| gemini-2.5-flash | Gemini 2.5 Flash | Yes | Fast, efficient |
| gemini-2.5-flash-lite | Gemini 2.5 Flash Lite | Yes | Lightweight |

**Image Generation Models**: gemini-2.5-flash-image, gemini-3-pro-image-preview (Nano Banana)

**Special Features**:
- Native multimodal support
- Long context windows
- Grounding with Google Search

**Usage Example**:
```go
provider := gemini.New(os.Getenv("GEMINI_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(gemini.ModelGemini25Flash).
    User("Summarize this document.").
    GetResponse(ctx)
```

---

### xAI (Grok)

**API Endpoint**: `https://api.x.ai/v1`

**Authentication**: API key via `XAI_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Notes |
|-------|--------------|-----------|-------|
| grok-4 | Grok 4 | Yes | Latest flagship |
| grok-4-fast-reasoning | Grok 4 Fast (Reasoning) | Yes | Fast with reasoning |
| grok-4-fast-non-reasoning | Grok 4 Fast (Non-Reasoning) | No | Fast without reasoning |
| grok-4-1-fast-reasoning | Grok 4.1 Fast (Reasoning) | Yes | Newest fast model |
| grok-3 | Grok 3 | Yes | Previous generation |
| grok-3-mini | Grok 3 Mini | Yes | Smaller model |
| grok-code-fast | Grok Code Fast | No | Code specialized |

**Special Features**:
- Real-time information access
- Distinct reasoning modes

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
- Citation support
- Research report generation

**Usage Example**:
```go
provider := perplexity.New(os.Getenv("PERPLEXITY_API_KEY"))
client := core.NewClient(provider)

resp, err := client.Chat(perplexity.ModelSonarPro).
    User("What are the latest developments in AI?").
    GetResponse(ctx)
```

---

### Z.ai (GLM)

**API Endpoint**: `https://open.bigmodel.cn/api/paas/v4`

**Authentication**: API key via `ZAI_API_KEY` environment variable

**Models**:

| Model | Display Name | Reasoning | Vision | Notes |
|-------|--------------|-----------|--------|-------|
| glm-4.7 | GLM-4.7 | Yes | No | Latest flagship |
| glm-4.7-flash | GLM-4.7 Flash | No | No | Fast |
| glm-4.6 | GLM-4.6 | Yes | No | |
| glm-4.6v | GLM-4.6V | Yes | Yes | Vision capable |
| glm-4.5 | GLM-4.5 | Yes | No | |
| glm-4.5v | GLM-4.5V | Yes | Yes | Vision capable |
| glm-4-32b-0414-128k | GLM-4 32B | No | No | Large context |

**Special Features**:
- Vision models for image understanding
- Chinese language optimization
- Large context windows

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

**Models**: Dynamic - any model pulled locally. Common models:
- llama3.2, llama3.2:70b
- mistral, mixtral
- qwen3
- gemma3
- deepseek-coder

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

## Choosing a Provider

| Use Case | Recommended Provider(s) |
|----------|------------------------|
| General chat and coding | OpenAI (GPT-4o, GPT-5), Anthropic (Claude) |
| Complex reasoning | OpenAI (o-series), Gemini 3, xAI Grok 4 |
| Web search integration | Perplexity |
| Local/private deployment | Ollama |
| Cost-sensitive applications | HuggingFace (routing), Ollama (local) |
| Embeddings and RAG | VoyageAI |
| Code generation | OpenAI (Codex models), Anthropic |
| Multimodal (vision) | OpenAI (GPT-4o), Gemini, Z.ai (GLM-V) |
| Image generation | OpenAI (DALL-E, GPT-Image), Gemini (Nano Banana) |

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
