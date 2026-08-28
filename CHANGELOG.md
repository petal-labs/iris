# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-08-28

Iris 1.0.0 is the first stable release. The API surface is now locked, and release
binaries are keyless-signed with Sigstore Cosign. This release consolidates the
unified tool contract, provider API consistency, client resilience, native token
counting, Gemini embeddings, Perplexity citations, CLI parity, and secure keystore
wiring that landed across the 0.18.0 development cycle.

### Added

- **Unified tool interface:** `core.Tool` now requires `Schema() ToolSchema`, with `core.ToolSchema` owned by `core` and aliased by the `tools` package. A tool whose `Schema()` has the wrong signature is now a compile error instead of silently transmitting `{}` as the parameters schema (#58)
- **Capability discovery helpers:** symmetric `As*` helpers for every optional provider interface — `AsEmbeddingProvider`, `AsContextualizedEmbeddingProvider`, `AsReranker`, `AsImageGenerator`, `AsContentPartSupporter`, `AsTokenCounter` — removing the need for raw type assertions (#45, #72)
- **Client-managed embeddings:** `(*core.Client).Embed` delegates embedding requests with client-level timeout, retry, and telemetry behavior (#45)
- **Token counting:** provider-neutral `core.TokenCounter` / `core.TokenCountResponse` / `AsTokenCounter` / `FeatureTokenCounting`, with native Anthropic (`/v1/messages/count_tokens`) and Gemini (`/v1beta/models/{model}:countTokens`) implementations (#70)
- **Ollama dynamic model discovery:** `ollama.ListModels(ctx)` reads the configured instance's `/api/tags` endpoint; `Models()` performs live discovery with a bounded fallback to the illustrative catalog only when discovery fails (#70)
- **Client resilience:** `core.RateLimiter` / `RateLimiterFunc` / `IntervalRateLimiter` / `WithRateLimiter` for client-side request pacing; `ProviderError.RetryAfter` preserves server-advised delays; stream setup failures now use the configured retry policy; `Retry-After` headers are honored as a minimum delay (#69)
- **Perplexity citations and search options:** `core.ChatResponse.Citations` populated for unary and streaming calls; provider-agnostic `core.SearchOptions` (domain filter, recency, search mode) via `ChatBuilder.SearchOptions`; `core.ErrSearchUnsupported` sentinel and `core.FeatureWebSearch` capability; requests with search options against unsupported providers fail fast (#55)
- **Gemini embeddings:** `(*gemini.Gemini).CreateEmbeddings` through the synchronous `batchEmbedContents` API and `gemini.ModelGeminiEmbedding001` in the model catalog; OpenAI provider now reports provider-level `FeatureReasoning` (#67)
- **Provider API consistency:** every provider package exposes `NewFromEnv`, `WithAPIKey`, `WithHeader`, and `DefaultAPIKeyEnvVar`; registry gains `ProviderConfig`, `RegisterConfigured`, and `CreateWithConfig` so endpoint-dependent providers (Azure AI Foundry) participate without encoding resource endpoints into API keys (#71)
- **Conversations:** `Conversation.AddToolResults(ctx, results)` for recording tool execution results before the next conversational turn (#60)
- **Multimodal:** provider content-part capability declarations and `WithWarningHandler` warnings before unsupported multimodal parts are omitted (#61)
- **CLI parity:** `iris chat` gains REPL mode, timeout, JSON-schema structured output, and `iris models` for listing provider catalogs (#62)
- **CLI provider coverage:** the CLI can now construct all ten built-in providers, including Perplexity, Voyage AI, and Azure AI Foundry, without losing provider-specific configuration; `iris init` emits a standalone Go module, selects a real model for all ten providers, and generates provider-appropriate starter code that compiles without hand-editing; Azure config fields (`endpoint`, `deployment_id`, `api_version`, `use_openai_endpoint`) (#66)
- **Secure keystore:** the CLI keystore now honors the `IRIS_KEYSTORE_KEY` environment variable (V2: Argon2id + AES-256-GCM); `iris keys migrate` re-encrypts legacy stores under the master key with a `.bak` backup; legacy stores remain readable during transition via a decryption fallback (#54)
- **Testing utilities:** `core.ProviderUnwrapper` convention so `RecordingProvider` preserves discovery of every optional interface through `As*` helpers; a bare `MockProvider` advertises a chat-focused capability baseline that passes structured-output and web-search builder gates (#72)
- **Signed releases:** every release binary, the SBOM, and `checksums.txt` are keyless-signed with Sigstore Cosign using GitHub Actions OIDC; each artifact ships with a `.sig` bundle (signature, Fulcio certificate, and Rekor transparency-log entry) (#118)
- **Community health files:** `CODE_OF_CONDUCT.md`, expanded `CONTRIBUTING.md`, issue/PR templates, `dependabot.yml`, and `CODEOWNERS` (#75)

### Changed

- **Breaking:** `NewConversation`, primary `Conversation` operations, and every `Memory` method now require `context.Context`; `SendWithContext` and `StreamWithContext` remain as deprecated aliases (#60)
- **Breaking:** the SDK test-utility package moved from `github.com/petal-labs/iris/testing` to `github.com/petal-labs/iris/iristest` (package `iristest`); the old import path is removed. Update imports and drop any alias used to avoid the standard-library `testing` collision (#102)
- **Breaking:** the CLI entrypoint moved from `cli/cmd/iris` to `cmd/iris`. Install with `go install github.com/petal-labs/iris/cmd/iris@latest`; `cli/commands`, `cli/config`, and `cli/keystore` are unchanged and `cmd/gen-models` keeps its path (#102)
- **Breaking:** the minimum Go version is now 1.25.0, raised from 1.24.0 across all four workspace modules, every CI workflow, and the `iris init` project scaffold. Dependency updates pulled in modules requiring Go >= 1.25.0, which the 1.24.0 workspace could not build
- The partial `providers` re-export layer (aliases of `core` types) is now deprecated and frozen in favor of direct `core` imports; existing aliases remain source-compatible (#71)
- `iris init` no longer generates the unused `iris.yaml` file; it emits a `go.mod` instead (#66)
- `iris init` now derives its scaffolded SDK dependency from validated build metadata (release linker flags, then `debug.ReadBuildInfo`, then a changelog-guarded fallback) instead of a hardcoded stale version (#110)
- `iris chat --provider ollama` no longer requires a keystore entry for local inference; other providers keep the actionable `iris keys set X` hint (#56)
- Local lint toolchain aligned with CI: golangci-lint pinned to v2.5.0 (#76)
- All ten provider model tables in `docs/PROVIDERS.md` reconciled against each provider's `Models()` implementation; a docs test now enforces the feature matrix against `Supports()` so it cannot silently drift (#57)
- README trimmed from 1,241 to 297 lines; reference material restructured into ten topic guides under `docs/guides/`, a `docs/README.md` index, and `docs/DEVELOPMENT.md` (#63)

### Fixed

- Conversation replay now preserves complete messages, including assistant tool calls, tool results, and multimodal parts; unary and streaming tool-call responses are retained in history (#60)
- Anthropic image blocks, OpenAI Chat Completions image parts, and Azure AI Foundry vision messages are now transmitted instead of silently dropping `Message.Parts`; Gemini now accepts the pointer parts produced by `UserMultimodal` (#61)
- `GetModelInfo` returns copies, not shared pointers into the provider registry, preventing accidental mutation of catalog entries (#64)
- `ChatBuilder` no longer aliases caller input slices, preventing mutation of request data after submission (#59)
- Provider HTTP errors are normalized through a shared `providers/internal/normalize` path, preserving `Retry-After` and response bodies consistently across all ten providers (#69, #82)
- Keystore V1 decryption key-schedule bug (a double hash that could not decrypt files written by the original v1 implementation) corrected (#54)
- `iris init` no longer pins generated projects to an SDK release older than the CLI that created them (#110)

### Security

- The CLI keystore now genuinely encrypts API keys with Argon2id key derivation and AES-256-GCM under `IRIS_KEYSTORE_KEY`; previously the secure path was implemented but unreachable, and keys were encrypted with a predictable machine-derived key (#54)
- GitHub Actions workflows hardened with SHA-pinned third-party actions and least-privilege `permissions` blocks (#74)
- Release artifacts are keyless-signed with Sigstore Cosign and recorded in the Rekor transparency log, binding each binary to the `petal-labs/iris` workflow identity (#118)

## [0.17.0] - 2026-08-02

### Added

- Per-provider empty-key preflight: chat providers now return `core.ErrUnauthorized` before any HTTP request when the API key is empty (openai, anthropic, gemini, xai, perplexity, huggingface, zai on `Chat`/`StreamChat`; voyageai on its embedding methods; azurefoundry's no-auth error now also satisfies `errors.Is(err, core.ErrUnauthorized)`) (#47)
- `WithAPIKey(key string)` option on all providers, overriding the constructor argument (last-wins) (#47)
- `core.WithStreamIdleTimeout(d)` client option: terminates a stream that produces no data for the idle window with an `ErrTimeout`-satisfying error (#44)
- `providers/internal/timeoutx` helper and a per-provider operation timeout (default 600s) bounding non-chat unary calls — embeddings, files, images, batch, and vector stores (#44)

### Changed

- Per-provider `WithTimeout`/`Config.Timeout` **un-deprecated** and now functional: it bounds each provider's non-chat unary operations (default 600s). Chat and streaming continue to honor `core.WithTimeout` and context deadlines (#44)
- `core.ErrTimeout` messages now name the provider and model in flight (#44)
- `tools.WithTimeout` timeout error now satisfies `errors.Is` for both `core.ErrTimeout` and `context.DeadlineExceeded` (#44)
- API keys are now trimmed of surrounding whitespace globally via `core.NewSecret` (already shipped groundwork); empty/whitespace keys are treated as empty (#47)

### Fixed

- `core.BatchWaiter.Wait` now keys `ErrBatchTimeout` on the wall-clock `maxWait` deadline instead of any `context.DeadlineExceeded`, preserving a provider's own transport error and its detail (#44)
- azurefoundry chat/stream honor `core.WithTimeout` only and are no longer capped by the per-provider timeout default (#44)

### Removed

- Unused `ProviderConfig.APIKeyRef` field from the CLI config (it was parsed but never resolved) (#47)

## [0.16.0] - 2026-08-01

### Added

- `core.ProviderError.Body` field carrying the raw (truncated) response body, preserved when the parsed error message lacks detail
- `core.ErrInvalidSchema` and `core.ErrStructuredOutputUnsupported` sentinel errors
- `ChatBuilder.ResponseJSONSchemaNonStrict` to opt out of strict schema validation
- Structured output support on the OpenAI Responses API (GPT-5.x models), previously silently dropped

### Changed

- `ResponseJSONSchema` is now strict by default: it forces `Strict = true` and validates the schema up front, returning `core.ErrInvalidSchema` when a schema is not strict-compatible (every object must set `additionalProperties: false` and list all properties in `required`). Opt out with `ResponseJSONSchemaNonStrict`.
- Requesting schema-based structured output from a provider or model without support now returns `core.ErrStructuredOutputUnsupported` before the request, instead of silently returning unconstrained output. Plain `ResponseJSON()` (JSON mode) is not gated.
- Provider error messages now include the real response body instead of generic HTTP status text
- `core.NewSecret` trims surrounding whitespace (fixing trailing newlines from secret managers); `Secret.IsEmpty()` is whitespace-aware; an empty OpenAI API key returns a descriptive `core.ErrUnauthorized` before any request

### Fixed

- Preserve the wrapped error chain in provider normalization so `errors.Is` reaches `context.DeadlineExceeded` and `core.ErrTimeout` (restoring execution-timeout surfacing for real providers on both unary and streaming paths) and correctly classify timed-out requests as non-retryable
- `core.DrainStream` no longer swallows a late streaming error and returns a false success
- `BatchWaiter.Wait` bounds each poll by the remaining `maxWait` budget so a stuck poll cannot hang indefinitely

## [0.15.0] - 2026-07-31

### Added

- Core-level LLM execution timeout applied uniformly across all providers
  - `core.WithTimeout(d)` client option and `DefaultTimeout` (120s)
  - `core.ErrTimeout` typed error that wraps `context.DeadlineExceeded`
  - Applies to both unary (`GetResponse`) and streaming (`Stream`) calls
  - Precedence: caller context deadline > `ChatBuilder.Timeout()` > client default
- Test coverage confirming `gpt-5.4-mini` routes to the OpenAI Responses API

### Changed

- A default 120s execution timeout now applies when neither a context deadline nor a per-call timeout is set; opt out with `core.WithTimeout(0)`. Streaming helpers that use a background context (for example `Conversation.Stream`) inherit this default, so generations longer than 120s are cut off unless the value is raised or disabled.

### Deprecated

- Per-provider `WithTimeout` option and `Config.Timeout` field (inert on every provider except Azure AI Foundry); use `core.WithTimeout` on the client instead

## [0.14.0] - 2026-07-29

### Added

- Azure AI Foundry provider
- OpenTelemetry instrumentation support
- Ollama embeddings via `core.EmbeddingProvider` (`/api/embed`)
- Latest model definitions synced from models.dev across providers

### Fixed

- Perplexity tool format updated to flat structure

## [0.13.0] - 2026-03-08

### Added

- Batch API support for async request processing at 50% cost savings
  - `BatchProvider` interface with `CreateBatch`, `GetBatchStatus`, `GetBatchResults`, `CancelBatch`, `ListBatches`
  - `BatchWaiter` utility for automatic polling with configurable intervals
  - OpenAI batch implementation with JSONL file handling
- Structured output for constrained model responses
  - `ResponseJSON()` for freeform JSON mode
  - `ResponseJSONSchema()` for strict JSON Schema enforcement
  - `JSONSchemaDefinition` type for schema configuration
- Conversation streaming support via `Conversation.Stream()` method
- Testing utilities package (`testing/`)
  - `MockProvider` with response queuing, error injection, and streaming mocks
  - `RecordingProvider` for capturing provider interactions
- Model constants code generator (`cmd/gen-models/`)
- Convenience re-exports at package root for streamlined imports

### Changed

- Updated README with comprehensive feature documentation
- Added examples for structured output, conversation streaming, batch API, and testing utilities

## [0.12.0] - 2026-02-15

### Added

- Shared provider error normalization package (`providers/internal/normalize`) and migration across providers
- Internal tool-call assembler to unify streaming tool-call reconstruction (`providers/internal/toolcalls/assembler`)
- Provider chat conformance integration suite (`tests/integration/chat_conformance_test.go`)
- Integration smoke test workflow for a fast OpenAI chat check in CI

### Changed

- Refactored CLI command wiring and provider setup (`cli/commands/app.go`, `cli/commands/provider_factory.go`)
- Restructured tool middleware implementation into focused middleware modules under `tools/`
- Updated README, examples, and contributor documentation to match hardening and refactor changes

### Removed

- Minimal agent execution implementation from `core/agent.go` and related tests

### Fixed

- Perplexity integration test model selection
- Outdated CLI init test expectation that referenced removed agents behavior
- CI coverage upload configuration to pass Codecov token correctly

## [0.11.0] - 2026-02-06

### Added

- Agent loop with parallel tool execution (`AgentRunner`)
  - Configurable iteration limits and execution hooks
  - Concurrent tool execution for improved performance
  - Support for streaming and non-streaming modes
- Tool middleware system (`tools/middleware.go`)
  - Composable middleware chain for tool execution
  - Built-in middleware: logging, timeout, rate limiting, caching, validation, metrics, retry, circuit breaker
  - Conditional middleware with `ForTools()` and `ExceptTools()`
  - Registry-level middleware support
- Memory management with auto-summarization (`core/memory.go`)
  - `Memory` interface for pluggable storage backends
  - `InMemoryStore` as thread-safe in-memory implementation
  - `Conversation` type for high-level multi-turn chat API
  - Automatic summarization when token threshold exceeded
  - Configurable preservation of recent messages
- Tool result injection for multi-turn tool use
- Gosec security scanning in CI workflow
- Codecov integration for test coverage reporting

### Fixed

- Missing PERPLEXITY_API_KEY in integration test CI

## [0.10.0] - 2026-02-03

### Added

- Secret type (`core.Secret`) to prevent accidental API key logging
- Keystore v2 with Argon2id encryption (OWASP recommended parameters)
- `IRIS_KEYSTORE_KEY` environment variable for production keystore encryption
- MasterKeySource interface for flexible key sourcing (env, prompt, fallback)
- Comprehensive documentation suite:
  - `docs/PROVIDERS.md` - Provider comparison with feature matrix
  - `docs/ARCHITECTURE.md` - Key design decisions and rationale
  - `docs/SECURITY.md` - Keystore encryption and security guide
- Expanded `core/doc.go` with comprehensive package documentation
- Ease-of-use improvements for SDK including streamlined client initialization
- Comprehensive test coverage for HuggingFace and Perplexity providers
- Documentation tests to verify doc completeness

### Changed

- Enhanced linter configuration and cleaned up dependencies
- Updated repository location references
- Removed agent graph references from codebase
- Moved documentation tests to `tests/` directory

### Fixed

- Removed unused functions in keystore module
- Fixed gofmt formatting in Ollama integration tests

## [0.9.0] - 2026-02-02

### Added

- Initial release of Iris SDK
- Provider-agnostic client for LLM interactions
- Support for OpenAI, Anthropic, and Ollama providers
- Streaming-first API design with ChatStream support
- Tool registry and schema definitions
- Vector store interfaces for Qdrant and PgVector
- Layered configuration system
- Secrets management with OS keychain support
- Telemetry hooks and retry policies
- Typed error handling

[Unreleased]: https://github.com/petal-labs/iris/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/petal-labs/iris/compare/v0.17.0...v1.0.0
[0.17.0]: https://github.com/petal-labs/iris/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/petal-labs/iris/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/petal-labs/iris/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/petal-labs/iris/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/petal-labs/iris/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/petal-labs/iris/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/petal-labs/iris/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/petal-labs/iris/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/petal-labs/iris/releases/tag/v0.9.0
