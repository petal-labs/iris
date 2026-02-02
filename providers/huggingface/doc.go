// Package huggingface provides an LLM provider implementation for Hugging Face
// Inference Providers. It gives access to thousands of models across multiple
// inference providers (Cerebras, Groq, Together, etc.) through HF's unified
// OpenAI-compatible API.
//
// # Basic Usage
//
//	provider := huggingface.New("hf_xxxx")
//	resp, err := provider.Chat(ctx, &core.ChatRequest{
//	    Model:    "meta-llama/Llama-3-8B-Instruct",
//	    Messages: []core.Message{{Role: "user", Content: "Hello!"}},
//	})
//
// # Provider Routing
//
// Hugging Face routes requests to different inference providers. You can control
// this routing by appending a suffix to the model name:
//
//   - ":fastest" - Routes to the provider with highest throughput
//   - ":cheapest" - Routes to the provider with lowest cost
//   - ":provider-name" - Routes to a specific provider (e.g., ":cerebras", ":together")
//
// Alternatively, set a default policy using WithProviderPolicy:
//
//	provider := huggingface.New("hf_xxxx", huggingface.WithProviderPolicy("fastest"))
//
// # Discovery API
//
// The provider includes methods to query the Hugging Face Hub API:
//
//   - GetModelStatus: Check if a model has available inference providers
//   - GetModelProviders: List providers serving a specific model
//   - ListModels: Query available models with filters
//
// # Authentication
//
// Requires a Hugging Face token with "Make calls to Inference Providers" permission.
// Generate one at https://huggingface.co/settings/tokens
package huggingface
