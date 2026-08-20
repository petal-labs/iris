# 📖 Iris Documentation

Everything beyond the [root README](../README.md) quick start lives here. The
root README covers installation, a first SDK call, Ollama, the CLI,
configuration, and security basics — these pages cover the rest.

## 🚀 Guides

Task-focused pages, each one thing you might want to do with the SDK.

| Guide | What it covers |
|---|---|
| [Provider Quick Starts](guides/provider-quickstarts.md) | Runnable examples for Anthropic, Gemini, xAI Grok, Z.ai GLM, and Perplexity (incl. search options and citations) |
| [Streaming Responses](guides/streaming.md) | The `ChatStream` three-channel design and `core.DrainStream` |
| [Timeouts and Warning Hooks](guides/timeouts-and-warnings.md) | The 120s default execution timeout, precedence rules, and routing non-fatal warnings to your logger |
| [Tools and Function Calling](guides/tools.md) | Implementing `tools.Tool` and composing the middleware stack (validation, timeout, logging, retry) |
| [Structured Output](guides/structured-output.md) | `ResponseJSON()` for JSON mode and strict-by-default `ResponseJSONSchema()` |
| [Conversation Management](guides/conversations.md) | `core.Conversation` history handling, streaming turns, and tool results |
| [Multimodal and Image Generation](guides/multimodal.md) | Sending images to models, plus generating, streaming, and editing images |
| [Responses API (GPT-5+)](guides/responses-api.md) | Reasoning effort, built-in tools like web search, and response chaining |
| [Batch API](guides/batch-api.md) | Async submission and `core.BatchWaiter` polling for reduced cost |
| [Testing Utilities](guides/testing.md) | `MockProvider` and `RecordingProvider` for deterministic, offline tests |

## 🧭 Reference

Longer-form material you read once and come back to.

| Document | What it covers |
|---|---|
| [Provider Comparison](PROVIDERS.md) | Feature support matrix, per-provider model catalogs, endpoints, auth, and how to choose |
| [Architecture Design Decisions](ARCHITECTURE.md) | Why streaming is first-class, why `Provider` is an interface, why `ChatBuilder` is not thread-safe, error and retry design |
| [Security Guide](SECURITY.md) | Keystore V1/V2 formats, Argon2id and AES-256-GCM details, the `Secret` type, CI/CD and Docker practices |
| [Development Guide](DEVELOPMENT.md) | Repository layout, Makefile targets, building the CLI, unit and integration tests, provider registry |

## 🧰 Project internals

Not user documentation — these directories back the project's tooling and
history. Listed here so they are discoverable rather than hidden.

| Directory | Purpose |
|---|---|
| [`changes/`](changes/) | Per-feature change records consumed by the automated documentation pipeline. One file per logical feature, named `YYYY-MM-DD_v{version}_{slug}.md`. See the repository `CLAUDE.md` for the required structure. |
| [`superpowers/`](superpowers/) | Design specs (`specs/`) and implementation plans (`plans/`) produced while building features. Kept in-tree because they feed the documentation pipeline. |

## 🔗 Elsewhere in the repository

- [Examples](../examples/README.md) — runnable programs for each provider and feature
- [CONTRIBUTING.md](../CONTRIBUTING.md) — setup, test tiers, and PR expectations
- [GoDoc](https://godoc.org/github.com/petal-labs/iris) — generated API reference
