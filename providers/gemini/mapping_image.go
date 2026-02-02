package gemini

import (
	"encoding/base64"
	"path/filepath"
	"strings"

	"github.com/petal-labs/iris/core"
)

// mapImageGenerateRequest converts a core image request to Gemini format.
func mapImageGenerateRequest(req *core.ImageGenerateRequest) *geminiImageRequest {
	r := &geminiImageRequest{
		Contents: []geminiContent{{
			Parts: []geminiPart{{
				Text: req.Prompt,
			}},
		}},
		GenerationConfig: &geminiImageGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}

	// Add aspect ratio if size is specified
	if req.Size != "" {
		r.GenerationConfig.ImageConfig = mapSizeToImageConfig(req.Size)
	}

	return r
}

// mapSizeToImageConfig maps core ImageSize to Gemini image config.
// Note: imageSize is only valid for gemini-3-pro-image-preview, not gemini-2.5-flash-image.
func mapSizeToImageConfig(size core.ImageSize) *geminiImageGenConfig {
	// Default to square aspect ratio
	aspectRatio := "1:1"
	switch size {
	case core.ImageSize1536x1024:
		aspectRatio = "3:2"
	case core.ImageSize1024x1536:
		aspectRatio = "2:3"
	}

	return &geminiImageGenConfig{
		AspectRatio: aspectRatio,
		// Don't set imageSize - it's only valid for gemini-3-pro-image-preview
	}
}

// geminiImageRequest extends geminiRequest with image-specific config.
type geminiImageRequest struct {
	Contents         []geminiContent              `json:"contents"`
	GenerationConfig *geminiImageGenerationConfig `json:"generationConfig,omitempty"`
}

// geminiImageGenerationConfig holds image generation config.
type geminiImageGenerationConfig struct {
	ResponseModalities []string              `json:"responseModalities,omitempty"`
	ImageConfig        *geminiImageGenConfig `json:"imageConfig,omitempty"`
}

// mapImageEditRequest converts a core image edit request to Gemini format.
func mapImageEditRequest(req *core.ImageEditRequest) *geminiImageRequest {
	parts := []geminiPart{{
		Text: req.Prompt,
	}}

	// Add input images
	for _, img := range req.Images {
		data, err := img.GetBytes()
		if err != nil || data == nil {
			continue
		}

		mimeType := "image/png"
		if img.Filename != "" {
			mimeType = detectMIMEType(img.Filename, data)
		}

		parts = append(parts, geminiPart{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		})
	}

	r := &geminiImageRequest{
		Contents: []geminiContent{{
			Parts: parts,
		}},
		GenerationConfig: &geminiImageGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}

	// Add aspect ratio if size is specified
	if req.Size != "" {
		r.GenerationConfig.ImageConfig = mapSizeToImageConfig(req.Size)
	}

	return r
}

// detectMIMEType detects MIME type from filename extension or magic bytes.
func detectMIMEType(filename string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}

	// Magic byte detection
	if len(data) >= 8 {
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "image/png"
		}
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image/jpeg"
		}
	}

	return "image/png"
}

// mapImageResponse converts a Gemini response to core format.
func mapImageResponse(resp *geminiResponse) *core.ImageResponse {
	r := &core.ImageResponse{
		Data: []core.ImageData{},
	}

	if len(resp.Candidates) == 0 {
		return r
	}

	// Collect all text and images from parts
	var textParts []string
	var images []core.ImageData

	for _, part := range resp.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			images = append(images, core.ImageData{
				B64JSON: part.InlineData.Data,
			})
		}
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}

	// If we have images, add them with any text as revised prompt
	if len(images) > 0 {
		revisedPrompt := ""
		if len(textParts) > 0 {
			revisedPrompt = strings.Join(textParts, " ")
		}
		for i := range images {
			if i == 0 && revisedPrompt != "" {
				images[i].RevisedPrompt = revisedPrompt
			}
			r.Data = append(r.Data, images[i])
		}
	} else if len(textParts) > 0 {
		// No images, just text - add as placeholder with revised prompt
		r.Data = append(r.Data, core.ImageData{
			RevisedPrompt: strings.Join(textParts, " "),
		})
	}

	return r
}
