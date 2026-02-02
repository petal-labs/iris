package openai

import (
	"strconv"

	"github.com/petal-labs/iris/core"
)

// mapImageGenerateRequest converts a core request to OpenAI format.
func mapImageGenerateRequest(req *core.ImageGenerateRequest) *openAIImageRequest {
	r := &openAIImageRequest{
		Model:  string(req.Model),
		Prompt: req.Prompt,
		N:      req.N,
	}

	// response_format is only for DALL-E models, not gpt-image models
	model := string(req.Model)
	if model == "dall-e-2" || model == "dall-e-3" {
		r.ResponseFormat = "b64_json"
	}

	if req.Size != "" {
		r.Size = string(req.Size)
	}
	if req.Quality != "" {
		r.Quality = string(req.Quality)
	}
	if req.Format != "" {
		r.OutputFormat = string(req.Format)
	}
	if req.Compression != nil {
		r.OutputCompression = req.Compression
	}
	if req.Background != "" {
		r.Background = string(req.Background)
	}
	if req.Moderation != "" {
		r.Moderation = req.Moderation
	}
	if req.User != "" {
		r.User = req.User
	}
	if req.PartialImages > 0 {
		r.Stream = true
		r.PartialImages = req.PartialImages
	}

	// Default to 1 image if not specified
	if r.N == 0 {
		r.N = 1
	}

	return r
}

// mapImageResponse converts an OpenAI response to core format.
func mapImageResponse(resp *openAIImageResponse) *core.ImageResponse {
	r := &core.ImageResponse{
		Created: resp.Created,
		Data:    make([]core.ImageData, len(resp.Data)),
	}

	for i, d := range resp.Data {
		r.Data[i] = core.ImageData{
			B64JSON:       d.B64JSON,
			URL:           d.URL,
			RevisedPrompt: d.RevisedPrompt,
		}
	}

	if resp.Usage != nil {
		r.Usage = &core.ImageUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	return r
}

// mapImageEditRequestFields converts a core edit request to multipart form fields.
// Returns a map of field names to values for non-file fields.
func mapImageEditRequestFields(req *core.ImageEditRequest) map[string]string {
	fields := map[string]string{
		"model":  string(req.Model),
		"prompt": req.Prompt,
	}

	if req.N > 0 {
		fields["n"] = strconv.Itoa(req.N)
	}
	if req.Size != "" {
		fields["size"] = string(req.Size)
	}
	if req.Quality != "" {
		fields["quality"] = string(req.Quality)
	}
	if req.Format != "" {
		fields["output_format"] = string(req.Format)
	}
	if req.Background != "" {
		fields["background"] = string(req.Background)
	}
	if req.InputFidelity != "" {
		fields["input_fidelity"] = string(req.InputFidelity)
	}
	if req.User != "" {
		fields["user"] = req.User
	}

	return fields
}

// mapImageChunk converts an OpenAI stream event to core format.
func mapImageChunk(event *openAIImageStreamEvent) core.ImageChunk {
	return core.ImageChunk{
		PartialImageIndex: event.PartialImageIndex,
		B64JSON:           event.B64JSON,
	}
}
