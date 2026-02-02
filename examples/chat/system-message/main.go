// Example: System Messages
//
// This example demonstrates how to use system messages
// to control the assistant's behavior and personality.
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
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	provider := openai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Example 1: Technical assistant
	fmt.Println("=== Technical Assistant ===")
	resp, err := client.Chat("gpt-4o-mini").
		System("You are a technical assistant. Provide concise, accurate answers focused on programming and technology. Use code examples when helpful.").
		User("How do I read a file in Go?").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println(resp.Output)
	fmt.Println()

	// Example 2: Friendly tutor
	fmt.Println("=== Friendly Tutor ===")
	resp, err = client.Chat("gpt-4o-mini").
		System("You are a friendly programming tutor. Explain concepts simply, as if teaching a beginner. Use analogies and encouraging language.").
		User("How do I read a file in Go?").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println(resp.Output)
	fmt.Println()

	// Example 3: Pirate personality
	fmt.Println("=== Pirate Personality ===")
	resp, err = client.Chat("gpt-4o-mini").
		System("You are a pirate programmer. Answer questions about code but speak like a pirate. Use nautical terms and pirate expressions.").
		User("How do I read a file in Go?").
		Temperature(0.9). // Higher temperature for more creative responses
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println(resp.Output)
}
