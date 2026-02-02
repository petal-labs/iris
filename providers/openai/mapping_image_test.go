// providers/openai/mapping_image_test.go
package openai

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestMapImageGenerateRequest(t *testing.T) {
	req := &core.ImageGenerateRequest{
		Model:   "gpt-image-1",
		Prompt:  "A sunset over mountains",
		Size:    core.ImageSize1024x1024,
		Quality: core.ImageQualityHigh,
		N:       2,
	}

	mapped := mapImageGenerateRequest(req)

	if mapped.Model != "gpt-image-1" {
		t.Errorf("Model = %s, want gpt-image-1", mapped.Model)
	}
	if mapped.Prompt != "A sunset over mountains" {
		t.Errorf("Prompt = %s, want 'A sunset over mountains'", mapped.Prompt)
	}
	if mapped.Size != "1024x1024" {
		t.Errorf("Size = %s, want 1024x1024", mapped.Size)
	}
	if mapped.Quality != "high" {
		t.Errorf("Quality = %s, want high", mapped.Quality)
	}
	if mapped.N != 2 {
		t.Errorf("N = %d, want 2", mapped.N)
	}
	// gpt-image models don't use response_format (they always return base64)
	if mapped.ResponseFormat != "" {
		t.Errorf("ResponseFormat = %s, want empty for gpt-image models", mapped.ResponseFormat)
	}
}

func TestMapImageGenerateRequestDALLE(t *testing.T) {
	req := &core.ImageGenerateRequest{
		Model:  "dall-e-3",
		Prompt: "A sunset",
	}

	mapped := mapImageGenerateRequest(req)

	// DALL-E models should have response_format set
	if mapped.ResponseFormat != "b64_json" {
		t.Errorf("ResponseFormat = %s, want b64_json for DALL-E", mapped.ResponseFormat)
	}
}

func TestMapImageGenerateRequestDefaults(t *testing.T) {
	req := &core.ImageGenerateRequest{
		Model:  "gpt-image-1",
		Prompt: "A cat",
	}

	mapped := mapImageGenerateRequest(req)

	// Should default to 1 image
	if mapped.N != 1 {
		t.Errorf("N = %d, want 1 (default)", mapped.N)
	}
}

func TestMapImageResponse(t *testing.T) {
	resp := &openAIImageResponse{
		Created: 1234567890,
		Data: []openAIImageData{
			{
				B64JSON:       "aW1hZ2VkYXRh",
				RevisedPrompt: "A beautiful sunset over mountains",
			},
		},
	}

	mapped := mapImageResponse(resp)

	if mapped.Created != 1234567890 {
		t.Errorf("Created = %d, want 1234567890", mapped.Created)
	}
	if len(mapped.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(mapped.Data))
	}
	if mapped.Data[0].B64JSON != "aW1hZ2VkYXRh" {
		t.Errorf("B64JSON = %s, want aW1hZ2VkYXRh", mapped.Data[0].B64JSON)
	}
	if mapped.Data[0].RevisedPrompt != "A beautiful sunset over mountains" {
		t.Errorf("RevisedPrompt = %s, want 'A beautiful sunset over mountains'", mapped.Data[0].RevisedPrompt)
	}
}

func TestMapImageResponseWithUsage(t *testing.T) {
	resp := &openAIImageResponse{
		Created: 1234567890,
		Data:    []openAIImageData{{B64JSON: "dGVzdA=="}},
		Usage: &openAIImageUsage{
			InputTokens:  100,
			OutputTokens: 200,
			TotalTokens:  300,
		},
	}

	mapped := mapImageResponse(resp)

	if mapped.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if mapped.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", mapped.Usage.InputTokens)
	}
	if mapped.Usage.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", mapped.Usage.OutputTokens)
	}
	if mapped.Usage.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", mapped.Usage.TotalTokens)
	}
}
