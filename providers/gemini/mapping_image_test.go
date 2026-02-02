package gemini

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestMapImageGenerateRequest(t *testing.T) {
	req := &core.ImageGenerateRequest{
		Model:  "gemini-2.5-flash-image",
		Prompt: "A sunset over mountains",
		Size:   core.ImageSize1024x1024,
	}

	gemReq := mapImageGenerateRequest(req)

	if len(gemReq.Contents) != 1 {
		t.Fatalf("len(Contents) = %d, want 1", len(gemReq.Contents))
	}

	if gemReq.GenerationConfig == nil {
		t.Fatal("GenerationConfig is nil")
	}

	// Should have responseModalities for image generation
	modalities := gemReq.GenerationConfig.ResponseModalities
	if len(modalities) != 2 || modalities[0] != "TEXT" || modalities[1] != "IMAGE" {
		t.Errorf("ResponseModalities = %v, want [TEXT IMAGE]", modalities)
	}

	// Should have ImageConfig with aspect ratio when Size is provided
	if gemReq.GenerationConfig.ImageConfig == nil {
		t.Fatal("ImageConfig is nil")
	}

	if gemReq.GenerationConfig.ImageConfig.AspectRatio != "1:1" {
		t.Errorf("AspectRatio = %s, want 1:1", gemReq.GenerationConfig.ImageConfig.AspectRatio)
	}
}

func TestMapImageGenerateRequestNoSize(t *testing.T) {
	req := &core.ImageGenerateRequest{
		Model:  "gemini-2.5-flash-image",
		Prompt: "A sunset over mountains",
	}

	gemReq := mapImageGenerateRequest(req)

	if len(gemReq.Contents) != 1 {
		t.Fatalf("len(Contents) = %d, want 1", len(gemReq.Contents))
	}

	if gemReq.GenerationConfig == nil {
		t.Fatal("GenerationConfig is nil")
	}

	// Should still have responseModalities
	modalities := gemReq.GenerationConfig.ResponseModalities
	if len(modalities) != 2 || modalities[0] != "TEXT" || modalities[1] != "IMAGE" {
		t.Errorf("ResponseModalities = %v, want [TEXT IMAGE]", modalities)
	}

	// ImageConfig should be nil when no Size is provided
	if gemReq.GenerationConfig.ImageConfig != nil {
		t.Error("ImageConfig should be nil when no Size is provided")
	}
}
