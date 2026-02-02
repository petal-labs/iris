package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/zai"
)

func main() {
	apiKey := os.Getenv("ZAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ZAI_API_KEY not set")
		os.Exit(1)
	}

	p := zai.New(apiKey)
	c := core.NewClient(p)

	// Use GLM-4.7 with thinking enabled
	resp, err := c.Chat(zai.ModelGLM47).
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
