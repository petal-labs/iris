// Example: Basic xAI Grok Chat Completion
//
// This example demonstrates using the Iris SDK with xAI Grok
// to send a chat message and receive a response.
//
// Run with:
//
//	export XAI_API_KEY=your-key
//	go run main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/xai"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "XAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Create xAI provider
	provider := xai.New(apiKey)

	// Create client
	client := core.NewClient(provider)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send chat request using fluent builder
	// Using grok-4-1-fast-non-reasoning for quick responses
	resp, err := client.Chat(xai.ModelGrok41FastNonReasoning).
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
