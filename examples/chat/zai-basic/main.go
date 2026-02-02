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

	resp, err := c.Chat(zai.ModelGLM47Flash).
		User("Hello, please introduce yourself.").
		GetResponse(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.Output)
}
