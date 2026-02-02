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

	// Use a model that supports thinking, like qwen3
	resp, err := c.Chat("qwen3").
		System("You are a helpful assistant. Think step by step.").
		User("What is 15% of 240?").
		ReasoningEffort(core.ReasoningEffortHigh).
		GetResponse(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Display reasoning if available
	if resp.Reasoning != nil && len(resp.Reasoning.Summary) > 0 {
		fmt.Println("Thinking:")
		for _, thought := range resp.Reasoning.Summary {
			fmt.Printf("  %s\n", thought)
		}
		fmt.Println()
	}

	fmt.Println("Answer:")
	fmt.Println(resp.Output)

	fmt.Printf("\nTokens: %d prompt + %d completion = %d total\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}
