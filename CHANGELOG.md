# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Secret type and keystore v2 with Argon2id encryption for secure credential storage
- Ease-of-use improvements for SDK including streamlined client initialization
- Comprehensive test coverage for HuggingFace and Perplexity providers

### Changed

- Enhanced linter configuration and cleaned up dependencies
- Updated repository location references
- Removed agent graph references from codebase

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

[Unreleased]: https://github.com/petal-labs/iris/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/petal-labs/iris/releases/tag/v0.9.0
