# Iris SDK Examples

This directory contains runnable examples for the Iris SDK across chat, streaming, tools, batch, testing, image, speech, and real-time voice-agent workflows.

## Module Structure

Examples live in a separate module (`github.com/petal-labs/iris/examples`) and are wired into the root `go.work` for local development.

You can run examples from:
- Project root (recommended)
- `examples/` directory

## Prerequisites

- Go 1.25+
- Provider credentials for the examples you run:

```bash
export OPENAI_API_KEY=your-key      # OpenAI chat + tools + image + batch + Responses API examples
export GEMINI_API_KEY=your-key      # Gemini image example
export XAI_API_KEY=your-key         # xAI examples
export ZAI_API_KEY=your-key         # Z.ai examples
export HF_TOKEN=your-key            # Hugging Face examples
export ELEVENLABS_API_KEY=your-key  # ElevenLabs speech and voice examples
export ELEVENLABS_AGENT_ID=agent_... # Optional: hosted ElevenAgents example
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
go run ./examples/chat/conversation-streaming
go run ./examples/chat/structured-output
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

# Batch API example
go run ./examples/batch/basic

# Testing utilities example
go run ./examples/testing/mock-provider

# Image examples
go run ./examples/image/basic
go run ./examples/image/streaming
go run ./examples/image/edit
go run ./examples/image/gemini

# Speech examples (ElevenLabs)
go run ./examples/speech/elevenlabs-synthesize -text "Hello from Iris." -out speech.mp3
go run ./examples/speech/elevenlabs-stream -text "Streaming audio from Iris." -out stream.mp3
go run ./examples/speech/elevenlabs-transcribe -file speech.mp3 -diarize -timestamps word
go run ./examples/speech/elevenlabs-voices -category premade -limit 10

# Real-time voice-agent examples (ElevenLabs + Iris LLM)
go run ./examples/voice/pipeline -prompt "Give me a one-sentence fun fact about Go." -out reply.pcm
go run ./examples/voice/speech-engine -addr :8080 -path /speech-engine
go run ./examples/voice/elevenagents -prompt "Hello! What can you help with?" -out agent.pcm
go run ./examples/voice/elevenagents -signed-url-only
```

### From `examples/` Directory

```bash
cd examples
go run ./chat/basic
go run ./chat/structured-output
go run ./chat/responses-api
go run ./batch/basic
go run ./testing/mock-provider
go run ./tools/weather
go run ./image/basic
go run ./speech/elevenlabs-synthesize -text "Hello from Iris." -out speech.mp3
go run ./speech/elevenlabs-voices -category premade -limit 10
go run ./voice/pipeline -prompt "Hello!" -out reply.pcm
```

## Highlights

- `chat/structured-output`: JSON mode and JSON Schema enforcement for type-safe responses.
- `chat/conversation-streaming`: Built-in `Conversation` type with streaming support.
- `batch/basic`: Batch API for async processing at 50% cost savings.
- `testing/mock-provider`: `MockProvider` and `RecordingProvider` for deterministic tests.
- `chat/responses-api`: GPT-5 Responses API usage (reasoning, web search, response chaining).
- `tools/weather`: tool calling with middleware (`WithBasicValidation`, `WithTimeout`, `WithLogging`) and warning routing (`WithWarningHandler`).
- `chat/huggingface-discovery`: model discovery helpers (`GetModelStatus`, `GetModelProviders`, `ListModels`).
- `image/gemini`: Gemini image generation path.
- `speech/elevenlabs-synthesize`: Synchronous ElevenLabs TTS saving audio to disk.
- `speech/elevenlabs-stream`: HTTP streaming TTS measuring first-chunk latency and writing incremental chunks.
- `speech/elevenlabs-transcribe`: File/reader Scribe transcription with optional diarization, timestamps, and audio events.
- `speech/elevenlabs-voices`: Read-only voice library discovery (`ListVoices`) and voice settings lookup (`GetVoiceSettings`).
- `voice/pipeline`: Iris-managed full-duplex voice pipeline (`voice.Pipeline`) streaming OpenAI tokens into ElevenLabs Flash v2.5 WebSocket TTS (with optional 16kHz PCM input via Scribe v2 Realtime STT), `core.VoiceTurnTracker`, and latency telemetry.
- `voice/speech-engine`: ElevenLabs Speech Engine upstream WebSocket server (`elevenlabs.NewSpeechEngineHandler` `http.Handler` with HS256 JWT verification) backed by any Iris LLM.
- `voice/elevenagents`: Hosted ElevenAgents `core.VoiceSession` consumption and backend signed-URL minting (`GetAgentSignedURL`).
