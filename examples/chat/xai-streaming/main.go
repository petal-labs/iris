// Example: Streaming xAI Grok Chat Completion
//
// This example demonstrates how to use streaming responses
// with xAI Grok to display text as it's generated.
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
	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "XAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	provider := xai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Streaming response from Grok:")
	fmt.Println("---")

	// Start streaming request with grok-4
	stream, err := client.Chat(xai.ModelGrok4).
		User("Write a short poem about space exploration. Make it 4 lines.").
		Stream(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error starting stream:", err)
		os.Exit(1)
	}

	// Print chunks as they arrive
	for chunk := range stream.Ch {
		fmt.Print(chunk.Delta)
	}
	fmt.Println()
	fmt.Println("---")

	// Check for any errors during streaming
	select {
	case err := <-stream.Err:
		if err != nil {
			fmt.Fprintln(os.Stderr, "Stream error:", err)
			os.Exit(1)
		}
	default:
	}

	// Get final response with usage stats
	select {
	case resp := <-stream.Final:
		if resp != nil {
			fmt.Printf("Tokens: %d prompt + %d completion = %d total\n",
				resp.Usage.PromptTokens,
				resp.Usage.CompletionTokens,
				resp.Usage.TotalTokens)
		}
	default:
		fmt.Println("(Usage stats not available)")
	}
}
