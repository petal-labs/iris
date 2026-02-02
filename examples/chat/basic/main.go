// Example: Basic Chat Completion
//
// This example demonstrates the simplest use of the Iris SDK
// to send a chat message and receive a response.
//
// Run with:
//
//	export OPENAI_API_KEY=your-key
//	go run main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Create OpenAI provider
	provider := openai.New(apiKey)

	// Create client
	client := core.NewClient(provider)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send chat request using fluent builder
	resp, err := client.Chat("gpt-4o-mini").
		User("What is the capital of France? Please respond in one sentence.").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Print response
	fmt.Println("Response:", resp.Output)
	fmt.Printf("Tokens: %d prompt + %d completion = %d total\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}
