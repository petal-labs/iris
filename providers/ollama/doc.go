// Package ollama provides an Ollama provider implementation for the Iris SDK.
//
// Ollama is a local-first LLM platform that allows running models locally,
// with optional cloud support via ollama.com.
//
// # Local Usage (Default)
//
// For local Ollama instances, no API key is required:
//
//	provider := ollama.New()
//	client := core.NewClient(provider)
//
//	resp, err := client.Chat("llama3.2").
//		User("Hello!").
//		GetResponse(ctx)
//
// # Custom Base URL
//
// To connect to a remote Ollama instance:
//
//	provider := ollama.New(
//		ollama.WithBaseURL("http://remote-host:11434"),
//	)
//
// # Ollama Cloud
//
// For Ollama Cloud (ollama.com), an API key is required:
//
//	provider := ollama.New(
//		ollama.WithCloud(),
//		ollama.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
//	)
//
// # Features
//
// The Ollama provider supports:
//   - Chat completions with any locally available model
//   - Streaming responses
//   - Tool/function calling (for supported models)
//   - Thinking/reasoning mode (for supported models like qwen3)
//
// # Models
//
// Unlike other providers, Ollama models are dynamic - you can use any model
// that you have pulled locally. Common models include:
//   - llama3.2, llama3.2:70b
//   - mistral, mixtral
//   - qwen3
//   - gemma3
//   - deepseek-coder
//
// See https://ollama.com/library for available models.
package ollama
