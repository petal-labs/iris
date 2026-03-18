// Package azurefoundry provides an Iris provider implementation for Azure AI Foundry.
//
// Azure AI Foundry is Microsoft's unified AI platform providing access to multiple
// model families (OpenAI, Meta Llama, Mistral, Cohere, and more) through a
// standardized inference API.
//
// # Authentication
//
// The provider supports two authentication methods:
//
// API Key Authentication:
//
//	provider := azurefoundry.New(
//	    "https://my-resource.services.ai.azure.com",
//	    "my-api-key",
//	)
//
// Microsoft Entra ID (Azure AD) Authentication:
//
//	cred, _ := azidentity.NewDefaultAzureCredential(nil)
//	provider := azurefoundry.NewWithCredential(
//	    "https://my-resource.services.ai.azure.com",
//	    cred,
//	)
//
// Environment Variables:
//
//	provider, err := azurefoundry.NewFromEnv()
//	// Uses AZURE_AI_ENDPOINT and AZURE_AI_API_KEY
//
// # Endpoint Formats
//
// Azure AI Foundry supports two endpoint formats:
//
// Model Inference API (default):
//
//	POST https://{resource}.services.ai.azure.com/models/chat/completions
//
// Azure OpenAI Service:
//
//	POST https://{resource}.openai.azure.com/openai/deployments/{deployment}/chat/completions
//
// Use WithOpenAIEndpoint() to switch to the Azure OpenAI format:
//
//	provider := azurefoundry.New(endpoint, apiKey,
//	    azurefoundry.WithOpenAIEndpoint(),
//	    azurefoundry.WithDeploymentID("gpt-4o"),
//	)
//
// # Basic Usage
//
//	provider := azurefoundry.New(
//	    "https://my-resource.services.ai.azure.com",
//	    "my-api-key",
//	)
//
//	client := core.NewClient(provider)
//
//	resp, err := client.Chat().
//	    Model("gpt-4o").
//	    System("You are a helpful assistant.").
//	    User("Hello!").
//	    GetResponse(context.Background())
//
// # Streaming
//
//	stream, err := client.Chat().
//	    Model("gpt-4o").
//	    User("Write a poem.").
//	    Stream(context.Background())
//
//	for chunk := range stream.Ch {
//	    fmt.Print(chunk.Delta)
//	}
//
// # Supported Features
//
//   - Chat completions
//   - Streaming responses
//   - Tool calling (function calling)
//   - Structured output (JSON mode, JSON Schema)
//   - Embeddings
//
// # Models
//
// Azure AI Foundry provides access to various models depending on your deployment:
//   - OpenAI: GPT-4o, GPT-4 Turbo, GPT-3.5 Turbo
//   - Meta: Llama 3.1 (8B, 70B, 405B)
//   - Mistral: Mistral Large, Mistral Small
//   - Cohere: Command R, Command R+
//   - And more via the Model Inference API
//
// Use the model deployment name from your Azure configuration as the model ID.
package azurefoundry
