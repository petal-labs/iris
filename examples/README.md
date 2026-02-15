# Iris SDK Examples

This directory contains runnable examples for the Iris SDK across chat, streaming, tools, and image workflows.

## Module Structure

Examples live in a separate module (`github.com/petal-labs/iris/examples`) and are wired into the root `go.work` for local development.

You can run examples from:
- Project root (recommended)
- `examples/` directory

## Prerequisites

- Go 1.24+
- Provider credentials for the examples you run:

```bash
export OPENAI_API_KEY=your-key      # OpenAI chat + tools + image + Responses API examples
export GEMINI_API_KEY=your-key      # Gemini image example
export XAI_API_KEY=your-key         # xAI examples
export ZAI_API_KEY=your-key         # Z.ai examples
export HF_TOKEN=your-key            # Hugging Face examples
```

## Running Examples

### From Project Root

```bash
# Build all examples
go build ./examples/...

# OpenAI examples
go run ./examples/chat/basic
go run ./examples/chat/streaming
go run ./examples/chat/system-message
go run ./examples/chat/conversation
go run ./examples/chat/responses-api

# Provider-specific chat examples
go run ./examples/chat/ollama-basic
go run ./examples/chat/ollama-streaming
go run ./examples/chat/ollama-reasoning
go run ./examples/chat/huggingface-basic
go run ./examples/chat/huggingface-streaming
go run ./examples/chat/huggingface-discovery
go run ./examples/chat/xai-basic
go run ./examples/chat/xai-streaming
go run ./examples/chat/xai-reasoning
go run ./examples/chat/zai-basic
go run ./examples/chat/zai-streaming
go run ./examples/chat/zai-reasoning

# Tools + middleware example
go run ./examples/tools/weather

# Image examples
go run ./examples/image/basic
go run ./examples/image/streaming
go run ./examples/image/edit
go run ./examples/image/gemini
```

### From `examples/` Directory

```bash
cd examples
go run ./chat/basic
go run ./chat/responses-api
go run ./tools/weather
go run ./image/basic
```

## Highlights

- `chat/responses-api`: GPT-5 Responses API usage (reasoning, web search, response chaining).
- `tools/weather`: tool calling with middleware (`WithBasicValidation`, `WithTimeout`, `WithLogging`) and warning routing (`WithWarningHandler`).
- `chat/huggingface-discovery`: model discovery helpers (`GetModelStatus`, `GetModelProviders`, `ListModels`).
- `image/gemini`: Gemini image generation path.
