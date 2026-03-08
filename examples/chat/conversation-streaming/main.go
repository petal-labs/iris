// Example: Conversation with Streaming
//
// This example demonstrates the built-in Conversation type with streaming
// support for real-time responses while maintaining conversation history.
//
// Run with:
//
//	export OPENAI_API_KEY=your-key
//	go run main.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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

	// Create a conversation with a system prompt
	conv := core.NewConversation(client, "gpt-4o-mini",
		core.WithSystemMessage("You are a helpful programming tutor. Keep responses concise. Remember our conversation context."),
	)

	fmt.Println("Chat with streaming responses (type 'quit' to exit)")
	fmt.Println("The assistant streams responses and remembers context.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if strings.ToLower(input) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		fmt.Print("Assistant: ")

		// Use streaming - response is automatically added to history when complete
		stream, err := conv.Stream(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			continue
		}

		// Print chunks as they arrive
		for chunk := range stream.Ch {
			fmt.Print(chunk.Delta)
		}
		fmt.Println()

		// Check for errors
		select {
		case err := <-stream.Err:
			if err != nil {
				fmt.Fprintf(os.Stderr, "Stream error: %v\n", err)
			}
		default:
		}

		// Get final response for metadata
		select {
		case finalResp := <-stream.Final:
			if finalResp != nil {
				fmt.Printf("  [tokens: %d]\n", finalResp.Usage.TotalTokens)
			}
		default:
		}

		fmt.Println()
	}

	// Show conversation history
	fmt.Printf("\nConversation had %d messages total.\n", conv.MessageCount())
}
