//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
)

func TestOpenAI_ImageGeneration(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	provider := openai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := provider.GenerateImage(ctx, &core.ImageGenerateRequest{
		Model:   openai.ModelGPTImage1Mini, // Use mini for faster/cheaper tests
		Prompt:  "A simple red circle on a white background",
		Size:    core.ImageSize1024x1024,
		Quality: core.ImageQualityLow,
	})
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}

	if len(resp.Data) == 0 {
		t.Fatal("Expected at least one image in response")
	}

	data, err := resp.Data[0].GetBytes()
	if err != nil {
		t.Fatalf("GetBytes failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Image data is empty")
	}

	t.Logf("Generated image: %d bytes", len(data))
	if resp.Data[0].RevisedPrompt != "" {
		t.Logf("Revised prompt: %s", resp.Data[0].RevisedPrompt)
	}
}

func TestOpenAI_ImageGeneration_WithOptions(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	provider := openai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := provider.GenerateImage(ctx, &core.ImageGenerateRequest{
		Model:      openai.ModelGPTImage1Mini,
		Prompt:     "A blue square with rounded corners",
		Size:       core.ImageSize1024x1024,
		Quality:    core.ImageQualityLow,
		Format:     core.ImageFormatPNG,
		Background: core.ImageBackgroundOpaque,
	})
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}

	if len(resp.Data) == 0 {
		t.Fatal("Expected at least one image in response")
	}

	data, err := resp.Data[0].GetBytes()
	if err != nil {
		t.Fatalf("GetBytes failed: %v", err)
	}

	// PNG files start with specific magic bytes
	if len(data) < 8 {
		t.Fatal("Image data too short to be a valid PNG")
	}

	// Check PNG magic number: 0x89 0x50 0x4E 0x47 0x0D 0x0A 0x1A 0x0A
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngMagic {
		if data[i] != b {
			t.Fatalf("Image doesn't appear to be a valid PNG (magic byte %d mismatch)", i)
		}
	}

	t.Logf("Generated PNG image: %d bytes", len(data))
}

func TestOpenAI_ImageStreaming(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	provider := openai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	stream, err := provider.StreamImage(ctx, &core.ImageGenerateRequest{
		Model:         openai.ModelGPTImage1Mini,
		Prompt:        "A simple green triangle",
		Size:          core.ImageSize1024x1024,
		Quality:       core.ImageQualityLow,
		PartialImages: 2,
	})
	if err != nil {
		t.Fatalf("StreamImage failed: %v", err)
	}

	var partialCount int
	for range stream.Ch {
		partialCount++
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	// Wait for final response
	final := <-stream.Final
	if final == nil {
		t.Log("No final response received (may be expected for some models)")
	} else if len(final.Data) > 0 {
		data, err := final.Data[0].GetBytes()
		if err != nil {
			t.Fatalf("GetBytes failed: %v", err)
		}
		t.Logf("Final image: %d bytes", len(data))
	}

	t.Logf("Received %d partial images", partialCount)
}

func TestOpenAI_ImageGeneration_MultipleImages(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	provider := openai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	resp, err := provider.GenerateImage(ctx, &core.ImageGenerateRequest{
		Model:   openai.ModelGPTImage1Mini,
		Prompt:  "A yellow star",
		Size:    core.ImageSize1024x1024,
		Quality: core.ImageQualityLow,
		N:       2, // Request 2 images
	})
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("Expected 2 images, got %d", len(resp.Data))
	}

	for i, img := range resp.Data {
		data, err := img.GetBytes()
		if err != nil {
			t.Fatalf("GetBytes for image %d failed: %v", i, err)
		}
		if len(data) == 0 {
			t.Fatalf("Image %d data is empty", i)
		}
		t.Logf("Image %d: %d bytes", i, len(data))
	}
}
