package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/huggingface"
)

func main() {
	// Get HF token from environment
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: HF_TOKEN environment variable not set")
		os.Exit(1)
	}

	// Create a Hugging Face provider
	p := huggingface.New(token)

	// Or use a specific provider routing policy:
	// p := huggingface.New(token, huggingface.WithProviderPolicy(huggingface.PolicyFastest))
	// p := huggingface.New(token, huggingface.WithProviderPolicy(huggingface.PolicyCheapest))
	// p := huggingface.New(token, huggingface.WithProviderPolicy("cerebras"))

	c := core.NewClient(p)

	// Use any model available on HF Inference Providers
	// You can also append :fastest, :cheapest, or :provider-name to the model
	// Run the huggingface-discovery example to see available models
	resp, err := c.Chat("meta-llama/Llama-3.1-8B-Instruct").
		System("You are a helpful assistant.").
		User("What is the capital of France?").
		GetResponse(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.Output)
	fmt.Printf("\nTokens: %d prompt + %d completion = %d total\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}
