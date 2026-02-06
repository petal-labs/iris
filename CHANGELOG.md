# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/petal-labs/iris/compare/v0.11.0...HEAD
[0.11.0]: https://github.com/petal-labs/iris/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/petal-labs/iris/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/petal-labs/iris/releases/tag/v0.9.0
