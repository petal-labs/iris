# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-02-01

### Added

- `StepController` interface for step-through debugging of PetalFlow workflows
- `CallbackStepController` for simple function-based step control
- `ChannelStepController` for interactive debugging via Go channels
- `BreakpointStepController` for pausing only at specific nodes
- `AutoStepController` for auto-stepping with configurable delay
- Step actions: continue, skip, abort, run-to-breakpoint
- `EnvelopeSnapshot` and `GraphSnapshot` for state inspection at step points
- Step events: `step_paused`, `step_resumed`, `step_skipped`, `step_aborted`
- `FilterNode` for pruning artifacts/messages with top-N, threshold, dedupe, type, match/exclude operations
- `TransformNode` for data reshaping with pick, omit, rename, flatten, merge, template, stringify, parse operations
- `GuardianNode` for validation with required fields, length limits, regex patterns, enum, PII detection, and schema checks
- `CacheNode` for memoization with TTL support, stable hashing, and pluggable `CacheStore` interface
- `SinkNode` for output to external systems (file, webhook, log, metric, var, custom) with error policies
- `HumanNode` for human-in-the-loop workflows with approval, choice, edit, input, and review request types
- `MemoryCacheStore` in-memory cache implementation with TTL and pruning support
- `HumanHandler` interface with `ChannelHumanHandler`, `CallbackHumanHandler`, `AutoApproveHandler`, and `QueuedHumanHandler` implementations
- `MockHTTPClient` and `MockMetricRecorder` test helpers for sink testing

## [0.7.0] - 2026-01-31

### Added

- PetalFlow workflow orchestration framework for LLM-powered agent pipelines
- Core types: `Node`, `Graph`, `Envelope`, `Runtime` for workflow execution
- `LLMNode` for LLM completions with provider adapters
- `ToolNode` for tool/function execution
- `RouterNode` for conditional branching with LLM and function-based strategies
- `MergeNode` for combining parallel branch results with 5 merge strategies
- `MapNode` for collection iteration with concurrent processing
- `GateNode` for conditional guards (block, skip, redirect actions)
- `GraphBuilder` fluent API for workflow construction
- Concurrent execution with configurable worker pools
- Provider and Tool adapters for Iris integration
- Event system for workflow observability
- PetalFlow documentation with use case guides (LLM workflows, RAG pipelines, multi-provider)
- Architecture Decision Records (ADRs) for PetalFlow design decisions

### Removed

- Legacy agent graph framework (replaced by PetalFlow)

## [0.6.0] - 2026-01-30

### Added

- VoyageAI provider with embedding, contextualized embedding, and reranking support
- Core `EmbeddingProvider`, `ContextualizedEmbeddingProvider`, and `RerankerProvider` interfaces
- OpenAI embeddings support with multiple encoding formats (float, base64)
- Gemini Files API with resumable uploads, pagination, and file polling
- Anthropic Files API with upload, download, list, and delete operations
- `InputType` and `OutputDType` options for embedding requests

### Fixed

- Gemini: use tagged switch to satisfy staticcheck
- Integration: improve test stability in CI

## [0.4.0] - 2026-01-29

### Added

- Hugging Face Inference Providers support
- OpenAI Files API with upload, download, list, and delete operations
- OpenAI Vector Stores API for file search with polling support
- Multimodal content support in core with `MessageBuilder` and content parts
- `ToolResources` for file search vector store IDs in core
- xAI Grok provider integration tests
- Z.ai provider integration tests
- Gemini provider integration tests
- Anthropic provider integration tests
- HuggingFace provider integration tests
- `ErrNotFound` sentinel error in core

### Changed

- CLI architecture improved with modular design

### Fixed

- CLI: remove duplicate provider imports
- Core: validate multimodal messages have content
- Core: remove redundant type declaration in test
- HuggingFace: use direct type conversion for HubModelInfo

## [0.3.0] - 2026-01-28

### Added

- xAI Grok provider support with chat, streaming, tools, and reasoning
- Z.ai GLM provider support with chat, streaming, tools, and thinking
- Ollama provider support for local and cloud deployments
- Google Gemini provider with full chat, streaming, and tools support
- Anthropic Claude provider support
- OpenAI image generation with DALL-E and GPT-Image models
- Gemini image generation with Nano Banana models
- Z.ai async image generation support
- `ImageGenerator` interface and image generation types in core
- `Provider()` accessor method to Client
- Makefile and git hooks setup for development workflow

### Fixed

- Gemini: fix response parsing when text precedes image data
- Gemini: use correct Nano Banana model names
- Z.ai: update default base URL to coding endpoint
- OpenAI: handle image_generation.completed event for final image
- OpenAI: set correct MIME type for image uploads
- OpenAI: increase scanner buffer for streaming images
- OpenAI: only set response_format for DALL-E models
- OpenAI: remove duplicate /v1 from image API paths
- Gemini: remove unused errNotImplemented variable
- gofmt formatting in xai models.go
- golangci-lint errors

### Removed

- Z.ai: image generation support (reverted)

## [0.2.0] - 2026-01-27

### Added

- OpenAI Responses API support for GPT-5+ models
- Support for all OpenAI chat and reasoning models
- GitHub Actions CI/CD workflows

### Fixed

- golangci-lint-action updated to v7
- Windows line endings handling
- File permissions test skip on Windows

## [0.1.0] - 2026-01-27

### Added

- Core SDK with `Client` and `ChatBuilder` fluent API
- Streaming abstraction with `DrainStream` helper
- Retry policy with exponential backoff
- Telemetry interfaces for observability
- Full `Provider` interface and `ModelInfo` type
- Tool interface, Registry, and `ParseArgs` for function calling
- OpenAI provider with streaming and non-streaming chat
- Minimal agent graph framework for workflow execution
- CLI skeleton with Cobra and config loading
- `keys` command with AES-256-GCM encrypted keystore
- `chat` command for LLM interactions
- `init` command for project scaffolding
- `graph export` command with Mermaid and JSON output
- Integration tests for OpenAI and CLI
- Comprehensive SDK documentation and examples

[Unreleased]: https://github.com/erikhoward/iris/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/erikhoward/iris/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/erikhoward/iris/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/erikhoward/iris/compare/v0.4.0...v0.6.0
[0.4.0]: https://github.com/erikhoward/iris/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/erikhoward/iris/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/erikhoward/iris/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/erikhoward/iris/releases/tag/v0.1.0
