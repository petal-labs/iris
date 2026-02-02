package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/providers/huggingface"
)

func main() {
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: HF_TOKEN environment variable not set")
		os.Exit(1)
	}

	p := huggingface.New(token)
	ctx := context.Background()

	// Example 1: Check if a model has inference providers available
	fmt.Println("=== Model Status ===")
	modelID := "meta-llama/Llama-3.1-8B-Instruct"
	status, err := p.GetModelStatus(ctx, modelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetModelStatus error: %v\n", err)
	} else {
		fmt.Printf("Model %s status: %s\n", modelID, status)
	}
	fmt.Println()

	// Example 2: List providers serving a specific model
	fmt.Println("=== Model Providers ===")
	providers, err := p.GetModelProviders(ctx, modelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetModelProviders error: %v\n", err)
	} else {
		fmt.Printf("Providers serving %s:\n", modelID)
		for _, prov := range providers {
			liveMarker := ""
			if prov.IsLive() {
				liveMarker = " [LIVE]"
			}
			fmt.Printf("  - %s (task: %s)%s\n", prov.Name, prov.Task, liveMarker)
		}
	}
	fmt.Println()

	// Example 3: List available models with filters
	fmt.Println("=== Available Models (text-generation, limit 10) ===")
	models, err := p.ListModels(ctx, huggingface.ListModelsOptions{
		Provider:    "all",
		PipelineTag: "text-generation",
		Limit:       10,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ListModels error: %v\n", err)
	} else {
		for _, m := range models {
			fmt.Printf("  - %s\n", m.ID)
		}
	}
	fmt.Println()

	// Example 4: List models from a specific provider
	fmt.Println("=== Models from Cerebras ===")
	cerebrasModels, err := p.ListModels(ctx, huggingface.ListModelsOptions{
		Provider: "cerebras",
		Limit:    5,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ListModels error: %v\n", err)
	} else {
		for _, m := range cerebrasModels {
			fmt.Printf("  - %s (pipeline: %s)\n", m.ID, m.PipelineTag)
		}
	}
}
