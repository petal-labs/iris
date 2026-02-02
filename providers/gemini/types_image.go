// Package gemini provides a Google Gemini API provider implementation for Iris.
package gemini

// geminiImageGenConfig holds image-specific generation config.
type geminiImageGenConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

// geminiInlineData represents inline image data in request/response.
type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64 encoded
}
