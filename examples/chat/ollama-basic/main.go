package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/ollama"
)

func main() {
	// Create a local Ollama provider (no API key needed for local)
	p := ollama.New()

	// Or connect to a remote Ollama instance:
	// p := ollama.New(ollama.WithBaseURL("http://remote-host:11434"))

	// Or use Ollama Cloud:
	// p := ollama.New(
	//     ollama.WithCloud(),
	//     ollama.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
	// )

	c := core.NewClient(p)

	resp, err := c.Chat("llama3.2").
		System("You are a helpful assistant.").
		User("What is the capital of France?").
		GetResponse(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.Output)
	fmt.Printf("\nTokens: %d prompt + %d completion = %d total\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}
