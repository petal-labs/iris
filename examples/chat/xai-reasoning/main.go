// Example: xAI Grok Reasoning Model
//
// This example demonstrates using xAI Grok's reasoning capabilities
// with grok-4.5, which exposes the model's thinking process.
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

	// Use a longer timeout for reasoning tasks
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fmt.Println("Sending reasoning query to grok-4.5...")
	fmt.Println()

	// Use grok-4.5 with high reasoning effort for complex problems
	resp, err := client.Chat(xai.ModelGrok45).
		ReasoningEffort(core.ReasoningEffortHigh).
		User("If I have 3 apples and give away half, then receive 2 more, how many apples do I have? Show your reasoning.").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Print the reasoning if available
	if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
		fmt.Println("Model's Reasoning:")
		fmt.Println("---")
		for _, s := range resp.Reasoning.Summary {
			fmt.Println(s)
		}
		fmt.Println("---")
		fmt.Println()
	}

	// Print final response
	fmt.Println("Response:", resp.Output)
	fmt.Printf("Tokens: %d prompt + %d completion = %d total\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}
