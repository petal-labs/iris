// Example: Image Editing
//
// This example demonstrates how to edit images using the Iris SDK
// with OpenAI's image editing capabilities.
//
// Run with:
//
//	export OPENAI_API_KEY=your-key
//	go run main.go input.png
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

	// Check for input image argument
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run main.go <input-image.png>")
		os.Exit(1)
	}

	inputPath := os.Args[1]

	// Read input image
	imageData, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input image: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Read input image: %s (%d bytes)\n", inputPath, len(imageData))

	// Create OpenAI provider
	provider := openai.New(apiKey)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fmt.Println("Editing image...")

	// Edit the image
	resp, err := provider.EditImage(ctx, &core.ImageEditRequest{
		Model:  openai.ModelGPTImage1,
		Prompt: "Add a rainbow arching across the sky",
		Images: []core.ImageInput{
			{Data: imageData, Filename: inputPath},
		},
		InputFidelity: core.ImageInputFidelityHigh,
		Size:          core.ImageSize1024x1024,
		Quality:       core.ImageQualityHigh,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Save the result
	if len(resp.Data) > 0 {
		data, err := resp.Data[0].GetBytes()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error decoding image:", err)
			os.Exit(1)
		}

		filename := "output.png"
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintln(os.Stderr, "Error saving image:", err)
			os.Exit(1)
		}

		fmt.Printf("Edited image saved to %s (%d bytes)\n", filename, len(data))

		if resp.Data[0].RevisedPrompt != "" {
			fmt.Printf("Revised prompt: %s\n", resp.Data[0].RevisedPrompt)
		}
	}
}
