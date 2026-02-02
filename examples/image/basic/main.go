// Example: Basic Image Generation
//
// This example demonstrates how to generate images using the Iris SDK
// with OpenAI's image generation models.
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
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Create OpenAI provider
	provider := openai.New(apiKey)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fmt.Println("Generating image...")

	// Generate an image
	resp, err := provider.GenerateImage(ctx, &core.ImageGenerateRequest{
		Model:   openai.ModelGPTImage1,
		Prompt:  "A serene mountain landscape at sunset with a calm lake in the foreground reflecting the orange and purple sky",
		Size:    core.ImageSize1024x1024,
		Quality: core.ImageQualityHigh,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Save the image
	if len(resp.Data) > 0 {
		data, err := resp.Data[0].GetBytes()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error decoding image:", err)
			os.Exit(1)
		}

		filename := "landscape.png"
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintln(os.Stderr, "Error saving image:", err)
			os.Exit(1)
		}

		fmt.Printf("Image saved to %s (%d bytes)\n", filename, len(data))

		if resp.Data[0].RevisedPrompt != "" {
			fmt.Printf("Revised prompt: %s\n", resp.Data[0].RevisedPrompt)
		}
	}
}
