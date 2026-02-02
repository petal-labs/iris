package openai

// openAIImageRequest represents a request to the OpenAI image generations endpoint.
type openAIImageRequest struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	N                 int    `json:"n,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	ResponseFormat    string `json:"response_format,omitempty"` // "b64_json" or "url"
	OutputFormat      string `json:"output_format,omitempty"`   // "png", "jpeg", "webp"
	OutputCompression *int   `json:"output_compression,omitempty"`
	Background        string `json:"background,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	User              string `json:"user,omitempty"`
	Stream            bool   `json:"stream,omitempty"`
	PartialImages     int    `json:"partial_images,omitempty"`
}

// openAIImageResponse represents a response from the OpenAI image API.
type openAIImageResponse struct {
	Created int64             `json:"created"`
	Data    []openAIImageData `json:"data"`
	Usage   *openAIImageUsage `json:"usage,omitempty"`
	Error   *openAIImageError `json:"error,omitempty"`
}

// openAIImageData represents a single image in the response.
type openAIImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// openAIImageUsage tracks token usage for image generation.
type openAIImageUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

// openAIImageError represents an error from the image API.
type openAIImageError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

// openAIImageStreamEvent represents a streaming event from image generation.
type openAIImageStreamEvent struct {
	Type              string `json:"type"` // "image_generation.partial_image", "image_generation.completed"
	PartialImageIndex int    `json:"partial_image_index,omitempty"`
	B64JSON           string `json:"b64_json,omitempty"`
}

// openAIImageCompletedEvent represents the final completed image event.
type openAIImageCompletedEvent struct {
	Type         string                     `json:"type"` // "image_generation.completed"
	B64JSON      string                     `json:"b64_json"`
	CreatedAt    int64                      `json:"created_at,omitempty"`
	Size         string                     `json:"size,omitempty"`
	Quality      string                     `json:"quality,omitempty"`
	Background   string                     `json:"background,omitempty"`
	OutputFormat string                     `json:"output_format,omitempty"`
	Usage        *openAIImageCompletedUsage `json:"usage,omitempty"`
}

// openAIImageCompletedUsage contains token usage for completed image generation.
type openAIImageCompletedUsage struct {
	TotalTokens        int                           `json:"total_tokens"`
	InputTokens        int                           `json:"input_tokens"`
	OutputTokens       int                           `json:"output_tokens"`
	InputTokensDetails *openAIImageInputTokenDetails `json:"input_tokens_details,omitempty"`
}

// openAIImageInputTokenDetails breaks down input token usage.
type openAIImageInputTokenDetails struct {
	TextTokens  int `json:"text_tokens"`
	ImageTokens int `json:"image_tokens"`
}
