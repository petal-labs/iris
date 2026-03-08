// Example: Testing Utilities
//
// This example demonstrates the testing package utilities:
// - MockProvider for deterministic test responses
// - RecordingProvider for capturing provider interactions
//
// Run with:
//
//	go run main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/testing"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== MockProvider Example ===")
	fmt.Println()
	demonstrateMockProvider(ctx)

	fmt.Println()
	fmt.Println("=== RecordingProvider Example ===")
	fmt.Println()
	demonstrateRecordingProvider(ctx)
}

func demonstrateMockProvider(ctx context.Context) {
	// Create a mock provider with queued responses
	mock := testing.NewMockProvider().
		WithResponse(core.ChatResponse{
			ID:     "resp-1",
			Model:  "mock-model",
			Output: "Hello! I'm a mock response.",
			Usage:  core.TokenUsage{TotalTokens: 10},
		}).
		WithResponse(core.ChatResponse{
			ID:     "resp-2",
			Model:  "mock-model",
			Output: "This is the second response.",
			Usage:  core.TokenUsage{TotalTokens: 15},
		}).
		WithDefaultResponse(core.ChatResponse{
			ID:     "default",
			Model:  "mock-model",
			Output: "Default response when queue is exhausted.",
		})

	// Create a client with the mock provider
	client := core.NewClient(mock)

	// First request - gets first queued response
	resp, err := client.Chat("any-model").
		User("Hello!").
		GetResponse(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Response 1: %s\n", resp.Output)

	// Second request - gets second queued response
	resp, err = client.Chat("any-model").
		User("Tell me more").
		GetResponse(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Response 2: %s\n", resp.Output)

	// Third request - queue exhausted, uses default
	resp, err = client.Chat("any-model").
		User("And more?").
		GetResponse(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Response 3: %s\n", resp.Output)

	// Inspect recorded calls
	fmt.Println("\nRecorded calls:")
	for i, call := range mock.Calls() {
		userMsg := ""
		for _, msg := range call.Request.Messages {
			if msg.Role == core.RoleUser {
				userMsg = msg.Content
				break
			}
		}
		fmt.Printf("  %d. %s - User said: %q\n", i+1, call.Method, userMsg)
	}

	// Demonstrate error injection
	fmt.Println("\nError injection:")
	errorMock := testing.NewMockProvider().
		WithError(core.ErrRateLimited)

	errorClient := core.NewClient(errorMock)
	_, err = errorClient.Chat("any-model").
		User("This will fail").
		GetResponse(ctx)
	fmt.Printf("  Got expected error: %v\n", err)

	// Demonstrate streaming mock
	fmt.Println("\nStreaming mock:")
	streamMock := testing.NewMockProvider().
		WithStreamingResponse(
			[]string{"Hello", " ", "world", "!"},
			&core.ChatResponse{
				ID:     "stream-resp",
				Model:  "mock-model",
				Output: "Hello world!",
			},
		)

	streamClient := core.NewClient(streamMock)
	stream, err := streamClient.Chat("any-model").
		User("Stream something").
		Stream(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Print("  Chunks: ")
	for chunk := range stream.Ch {
		fmt.Printf("[%s]", chunk.Delta)
	}
	fmt.Println()
}

func demonstrateRecordingProvider(ctx context.Context) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("OPENAI_API_KEY not set - using mock for demonstration")
		// Fall back to mock
		mock := testing.NewMockProvider().
			WithResponse(core.ChatResponse{
				ID:     "recorded",
				Model:  "gpt-4o-mini",
				Output: "The capital of France is Paris.",
				Usage:  core.TokenUsage{TotalTokens: 25},
			})
		demonstrateRecordingWithProvider(ctx, mock)
		return
	}

	// Wrap real provider with recorder
	realProvider := openai.New(apiKey)
	demonstrateRecordingWithProvider(ctx, realProvider)
}

func demonstrateRecordingWithProvider(ctx context.Context, provider core.Provider) {
	// Wrap with recording
	recorder := testing.NewRecordingProvider(provider)
	client := core.NewClient(recorder)

	// Make some calls
	fmt.Println("Making API calls...")

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.Chat("gpt-4o-mini").
		User("What is the capital of France?").
		GetResponse(ctxWithTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Response: %s\n", resp.Output)

	// Inspect recordings
	fmt.Println("\nRecorded interactions:")
	for i, rec := range recorder.Recordings() {
		fmt.Printf("  Call %d:\n", i+1)
		fmt.Printf("    Method:   %s\n", rec.Method)
		fmt.Printf("    Duration: %v\n", rec.Duration)

		// Show request details
		if rec.Request != nil {
			fmt.Printf("    Model:    %s\n", rec.Request.Model)
			for _, msg := range rec.Request.Messages {
				fmt.Printf("    %s: %s\n", msg.Role, truncate(msg.Content, 50))
			}
		}

		// Show response details
		if rec.Response != nil {
			fmt.Printf("    Output:   %s\n", truncate(rec.Response.Output, 60))
			fmt.Printf("    Tokens:   %d\n", rec.Response.Usage.TotalTokens)
		}

		if rec.Error != nil {
			fmt.Printf("    Error:    %v\n", rec.Error)
		}
	}

	// Clear recordings
	recorder.Clear()
	fmt.Printf("\nCleared recordings. Count: %d\n", len(recorder.Recordings()))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
