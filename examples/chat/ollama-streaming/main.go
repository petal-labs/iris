package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/ollama"
)

func main() {
	p := ollama.New()
	c := core.NewClient(p)

	stream, err := c.Chat("llama3.2").
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
