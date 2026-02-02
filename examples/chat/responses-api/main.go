// Example: OpenAI Responses API
//
// This example demonstrates the new Responses API features available
// with newer OpenAI models like GPT-5.2, including:
// - Reasoning with configurable effort levels
// - Built-in tools (web search, code interpreter)
// - Response chaining (continuing from previous responses)
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Example 1: Basic Responses API usage
	// GPT-5.2 automatically uses the Responses API
	fmt.Println("=== Example 1: Basic Responses API ===")
	resp, err := client.Chat(openai.ModelGPT52).
		Instructions("You are a helpful assistant.").
		User("What is the capital of France?").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("Response:", resp.Output)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Println()

	// Example 2: Using reasoning with high effort
	fmt.Println("=== Example 2: Reasoning with High Effort ===")
	resp, err = client.Chat(openai.ModelGPT52).
		Instructions("You are a math tutor. Show your reasoning.").
		User("What is 15% of 240?").
		ReasoningEffort(core.ReasoningEffortHigh).
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("Response:", resp.Output)
	if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
		fmt.Println("Reasoning Summary:")
		for _, summary := range resp.Reasoning.Summary {
			fmt.Printf("  - %s\n", summary)
		}
	}
	fmt.Println()

	// Example 3: Using built-in web search
	fmt.Println("=== Example 3: Built-in Web Search ===")
	resp, err = client.Chat(openai.ModelGPT52).
		Instructions("Use web search to find current information.").
		User("What are the top news stories today?").
		WebSearch().
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("Response:", resp.Output)
	fmt.Println()

	// Example 4: Response chaining
	fmt.Println("=== Example 4: Response Chaining ===")
	firstResp, err := client.Chat(openai.ModelGPT52).
		Instructions("You are a storyteller.").
		User("Start a short story about a robot.").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("First part:", firstResp.Output)
	fmt.Println()

	// Continue from the previous response
	secondResp, err := client.Chat(openai.ModelGPT52).
		ContinueFrom(firstResp.ID).
		User("Continue the story with an unexpected twist.").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("Continuation:", secondResp.Output)
	fmt.Println()

	// Example 5: Streaming with Responses API
	fmt.Println("=== Example 5: Streaming with Responses API ===")
	stream, err := client.Chat(openai.ModelGPT52).
		Instructions("You are a poet.").
		User("Write a haiku about programming.").
		Stream(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Print("Streaming: ")
	for chunk := range stream.Ch {
		fmt.Print(chunk.Delta)
	}
	fmt.Println()

	// Check for streaming errors
	select {
	case err := <-stream.Err:
		if err != nil {
			fmt.Fprintln(os.Stderr, "Stream error:", err)
		}
	default:
	}

	// Get final response with usage
	select {
	case finalResp := <-stream.Final:
		if finalResp != nil {
			fmt.Printf("Total tokens: %d\n", finalResp.Usage.TotalTokens)
		}
	default:
	}

	fmt.Println()
	fmt.Println("=== Backward Compatibility ===")
	// Example 6: Older models still use Chat Completions API
	resp, err = client.Chat(openai.ModelGPT4o).
		System("You are a helpful assistant.").
		User("What is 2+2?").
		GetResponse(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("GPT-4o (Chat Completions API):", resp.Output)
}
