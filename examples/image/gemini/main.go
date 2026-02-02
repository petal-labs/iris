// Example: Gemini Image Generation
//
// This example demonstrates how to generate images using the Iris SDK
// with Google Gemini's image generation models.
//
// Run with:
//
//	export GEMINI_API_KEY=your-key
//	go run main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "GEMINI_API_KEY environment variable not set")
		os.Exit(1)
	}

	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	fmt.Println("Generating image with Gemini...")

	// Type assert to ImageGenerator
	imageGen, ok := client.Provider().(core.ImageGenerator)
	if !ok {
		fmt.Fprintln(os.Stderr, "Provider does not support image generation")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := imageGen.GenerateImage(ctx, &core.ImageGenerateRequest{
		Model:  gemini.ModelGemini25FlashImage,
		Prompt: "A serene mountain landscape at sunset with a calm lake reflecting the colors of the sky",
		Size:   core.ImageSize1024x1024,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if len(resp.Data) == 0 {
		fmt.Fprintln(os.Stderr, "No images generated")
		os.Exit(1)
	}

	// Decode and save the image
	imageData, err := resp.Data[0].GetBytes()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error decoding image:", err)
		os.Exit(1)
	}

	if imageData == nil {
		fmt.Fprintln(os.Stderr, "No image data returned")
		os.Exit(1)
	}

	outputFile := "gemini_image.png"
	if err := os.WriteFile(outputFile, imageData, 0644); err != nil {
		fmt.Fprintln(os.Stderr, "Error saving image:", err)
		os.Exit(1)
	}

	fmt.Printf("Image saved to %s (%d bytes)\n", outputFile, len(imageData))

	if resp.Data[0].RevisedPrompt != "" {
		fmt.Printf("Revised prompt: %s\n", resp.Data[0].RevisedPrompt)
	}
}
