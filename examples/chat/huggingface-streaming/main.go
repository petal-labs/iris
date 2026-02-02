package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/huggingface"
)

func main() {
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: HF_TOKEN environment variable not set")
		os.Exit(1)
	}

	// Use fastest provider for streaming
	p := huggingface.New(token, huggingface.WithProviderPolicy(huggingface.PolicyFastest))
	c := core.NewClient(p)

	stream, err := c.Chat("meta-llama/Llama-3.1-8B-Instruct").
		User("Write a short poem about artificial intelligence.").
		Stream(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
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
			fmt.Fprintln(os.Stderr, "Stream error:", err)
			os.Exit(1)
		}
	default:
	}

	// Get final response with usage info
	select {
	case resp := <-stream.Final:
		if resp != nil {
			fmt.Printf("\nTokens: %d prompt + %d completion = %d total\n",
				resp.Usage.PromptTokens,
				resp.Usage.CompletionTokens,
				resp.Usage.TotalTokens)
		}
	default:
	}
}
