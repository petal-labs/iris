// Example: Batch API
//
// This example demonstrates the Batch API for submitting multiple requests
// for asynchronous processing at 50% cost savings.
//
// Note: Batch processing can take up to 24 hours. This example creates
// a batch and shows how to check status, but does not wait for completion.
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
	ctx := context.Background()

	// Check if provider supports batch API
	bp, ok := core.AsBatchProvider(provider)
	if !ok {
		fmt.Fprintln(os.Stderr, "Provider does not support Batch API")
		os.Exit(1)
	}

	fmt.Println("=== Batch API Example ===")
	fmt.Println()

	// Create batch requests
	requests := []core.BatchRequest{
		{
			CustomID: "translate-1",
			Request: core.ChatRequest{
				Model: "gpt-4o-mini",
				Messages: []core.Message{
					{Role: core.RoleSystem, Content: "You are a translator. Translate to French."},
					{Role: core.RoleUser, Content: "Hello, how are you?"},
				},
			},
		},
		{
			CustomID: "translate-2",
			Request: core.ChatRequest{
				Model: "gpt-4o-mini",
				Messages: []core.Message{
					{Role: core.RoleSystem, Content: "You are a translator. Translate to Spanish."},
					{Role: core.RoleUser, Content: "Good morning!"},
				},
			},
		},
		{
			CustomID: "translate-3",
			Request: core.ChatRequest{
				Model: "gpt-4o-mini",
				Messages: []core.Message{
					{Role: core.RoleSystem, Content: "You are a translator. Translate to German."},
					{Role: core.RoleUser, Content: "Thank you very much."},
				},
			},
		},
	}

	fmt.Printf("Submitting %d requests to batch API...\n", len(requests))

	// Submit the batch
	batchID, err := bp.CreateBatch(ctx, requests)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create batch: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Batch created: %s\n\n", batchID)

	// Check initial status
	info, err := bp.GetBatchStatus(ctx, batchID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get status: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Initial Status:")
	printBatchInfo(info)

	// For demonstration, poll a few times
	fmt.Println("\nPolling for status updates (3 attempts)...")
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Second)

		info, err = bp.GetBatchStatus(ctx, batchID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get status: %v\n", err)
			continue
		}

		fmt.Printf("\nAttempt %d:\n", i+1)
		printBatchInfo(info)

		if info.IsComplete() {
			fmt.Println("\nBatch completed!")
			break
		}
	}

	// If completed, get results
	if info.Status == core.BatchStatusCompleted {
		fmt.Println("\n=== Results ===")
		results, err := bp.GetBatchResults(ctx, batchID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get results: %v\n", err)
			os.Exit(1)
		}

		for _, result := range results {
			if result.IsSuccess() {
				fmt.Printf("%s: %s\n", result.CustomID, result.Response.Output)
			} else {
				fmt.Printf("%s: ERROR - %s\n", result.CustomID, result.Error.Message)
			}
		}
	} else {
		fmt.Println("\nBatch still processing. To check status later:")
		fmt.Printf("  Batch ID: %s\n", batchID)
		fmt.Println("\nUse BatchWaiter for automatic polling:")
		fmt.Println("  waiter := core.NewBatchWaiter(bp).WithPollInterval(30 * time.Second)")
		fmt.Println("  results, err := waiter.WaitAndCollect(ctx, batchID)")
	}

	// List recent batches
	fmt.Println("\n=== Recent Batches ===")
	batches, err := bp.ListBatches(ctx, 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list batches: %v\n", err)
	} else {
		for _, b := range batches {
			fmt.Printf("  %s: %s (%d/%d completed)\n",
				b.ID, b.Status, b.Completed, b.Total)
		}
	}
}

func printBatchInfo(info *core.BatchInfo) {
	fmt.Printf("  ID:        %s\n", info.ID)
	fmt.Printf("  Status:    %s\n", info.Status)
	fmt.Printf("  Progress:  %d/%d completed, %d failed\n",
		info.Completed, info.Total, info.Failed)
}
