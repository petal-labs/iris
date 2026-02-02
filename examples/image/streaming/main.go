// Example: Streaming Image Generation
//
// This example demonstrates how to generate images with streaming
// partial results using the Iris SDK.
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
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	fmt.Println("Generating image with streaming partial results...")

	// Stream an image with partial results
	stream, err := provider.StreamImage(ctx, &core.ImageGenerateRequest{
		Model:         openai.ModelGPTImage1,
		Prompt:        "A futuristic cityscape with flying cars and neon lights at night",
		Size:          core.ImageSize1024x1024,
		Quality:       core.ImageQualityHigh,
		PartialImages: 3, // Request 3 partial image updates
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Process partial images as they arrive
	partialCount := 0
	for chunk := range stream.Ch {
		partialCount++
		fmt.Printf("Received partial image %d (index %d)\n", partialCount, chunk.PartialImageIndex)

		// Optionally save partial images
		data, err := core.ImageData{B64JSON: chunk.B64JSON}.GetBytes()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding partial image: %v\n", err)
			continue
		}

		filename := fmt.Sprintf("partial_%d.png", chunk.PartialImageIndex)
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving partial image: %v\n", err)
			continue
		}
		fmt.Printf("  Saved to %s (%d bytes)\n", filename, len(data))
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			fmt.Fprintln(os.Stderr, "Stream error:", err)
			os.Exit(1)
		}
	default:
	}

	// Save final image
	final := <-stream.Final
	if final != nil && len(final.Data) > 0 {
		data, err := final.Data[0].GetBytes()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error decoding final image:", err)
			os.Exit(1)
		}

		filename := "final.png"
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintln(os.Stderr, "Error saving final image:", err)
			os.Exit(1)
		}

		fmt.Printf("\nFinal image saved to %s (%d bytes)\n", filename, len(data))

		if final.Data[0].RevisedPrompt != "" {
			fmt.Printf("Revised prompt: %s\n", final.Data[0].RevisedPrompt)
		}
	} else {
		fmt.Println("\nNo final image received")
	}

	fmt.Printf("\nTotal partial images received: %d\n", partialCount)
}
