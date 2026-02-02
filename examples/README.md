# Iris SDK Examples

This directory contains runnable examples demonstrating the Iris SDK features.

## Module Structure

Examples are in a separate Go module (`github.com/erikhoward/iris/examples`) that depends on the main Iris SDK. The project uses a Go workspace (`go.work`) for seamless local development.

You can run examples from either:
- **Project root**: Uses the workspace to resolve the local SDK
- **Examples directory**: Uses the `replace` directive in `go.mod`

## Prerequisites

- Go 1.24 or later
- Most examples require an OpenAI API key:

```bash
export OPENAI_API_KEY=your-api-key-here
```

## Chat Examples

### Basic Chat (`chat/basic`)

The simplest example showing how to send a chat message and receive a response.

```bash
cd chat/basic
go run main.go
```

### Streaming (`chat/streaming`)

Demonstrates streaming responses, printing text as it's generated.

```bash
cd chat/streaming
go run main.go
```

### System Messages (`chat/system-message`)

Shows how to use system messages to control the assistant's behavior and personality.

```bash
cd chat/system-message
go run main.go
```

### Multi-turn Conversation (`chat/conversation`)

Interactive example showing how to maintain context across multiple turns.

```bash
cd chat/conversation
go run main.go
```

## Tool Examples

### Weather Tool (`tools/weather`)

Demonstrates tool/function calling with a simulated weather API.

```bash
cd tools/weather
go run main.go
```

## Agent Examples

### Simple Agent Graph (`agents/simple`)

Shows how to build and execute a simple workflow using the graph framework.
This example doesn't require an API key.

```bash
cd agents/simple
go run main.go
```

## Running All Examples

### From Project Root (Recommended)

The Go workspace (`go.work`) allows running examples directly:

```bash
# Build all examples to check for compilation errors
go build ./examples/...

# Run a specific example
go run ./examples/chat/basic
go run ./examples/chat/streaming
go run ./examples/chat/system-message
go run ./examples/chat/conversation
go run ./examples/tools/weather
go run ./examples/agents/simple
```

### From Examples Directory

You can also run from within the examples directory:

```bash
cd examples

# Run examples
go run ./chat/basic
go run ./chat/streaming
go run ./tools/weather
```

## Example Output

### Basic Chat
```
Response: The capital of France is Paris.
Tokens: 25 prompt + 8 completion = 33 total
```

### Streaming
```
Streaming response:
---
In the realm of code so bright,
Go routines dance through the night,
Channels flow with data's stream,
Concurrency fulfills the dream.
---
Tokens: 30 prompt + 32 completion = 62 total
```

### Simple Agent Graph
```
=== Starting Graph Execution ===

[input] Processing input...
[input] Original: "  Iris Agent Framework  "
[transform] Transforming text...
[transform] Uppercase: IRIS AGENT FRAMEWORK
[output] Final results:
  Original:    Iris Agent Framework
  Uppercase: IRIS AGENT FRAMEWORK
  Lowercase: iris agent framework
  Length:    20

=== Execution Complete ===
```
