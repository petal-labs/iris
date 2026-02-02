package core

import "encoding/base64"

// FeatureImageGeneration indicates support for image generation.
const FeatureImageGeneration Feature = "image_generation"

// ImageSize represents supported image dimensions.
type ImageSize string

const (
	ImageSize1024x1024 ImageSize = "1024x1024"
	ImageSize1536x1024 ImageSize = "1536x1024"
	ImageSize1024x1536 ImageSize = "1024x1536"
	ImageSizeAuto      ImageSize = "auto"
)

// IsValid reports whether the image size is a recognized value.
func (s ImageSize) IsValid() bool {
	switch s {
	case ImageSize1024x1024, ImageSize1536x1024, ImageSize1024x1536, ImageSizeAuto:
		return true
	default:
		return false
	}
}

// ImageQuality represents the rendering quality level.
type ImageQuality string

const (
	ImageQualityLow    ImageQuality = "low"
	ImageQualityMedium ImageQuality = "medium"
	ImageQualityHigh   ImageQuality = "high"
	ImageQualityAuto   ImageQuality = "auto"
)

// IsValid reports whether the image quality is a recognized value.
func (q ImageQuality) IsValid() bool {
	switch q {
	case ImageQualityLow, ImageQualityMedium, ImageQualityHigh, ImageQualityAuto:
		return true
	default:
		return false
	}
}

// ImageFormat represents the output file format.
type ImageFormat string

const (
	ImageFormatPNG  ImageFormat = "png"
	ImageFormatJPEG ImageFormat = "jpeg"
	ImageFormatWebP ImageFormat = "webp"
)

// IsValid reports whether the image format is a recognized value.
func (f ImageFormat) IsValid() bool {
	switch f {
	case ImageFormatPNG, ImageFormatJPEG, ImageFormatWebP:
		return true
	default:
		return false
	}
}

// ImageBackground represents the background style.
type ImageBackground string

const (
	ImageBackgroundOpaque      ImageBackground = "opaque"
	ImageBackgroundTransparent ImageBackground = "transparent"
	ImageBackgroundAuto        ImageBackground = "auto"
)

// ImageInputFidelity represents the input image preservation level.
type ImageInputFidelity string

const (
	ImageInputFidelityLow  ImageInputFidelity = "low"
	ImageInputFidelityHigh ImageInputFidelity = "high"
)

// ImageAction represents the action to take (for Responses API).
type ImageAction string

const (
	ImageActionAuto     ImageAction = "auto"
	ImageActionGenerate ImageAction = "generate"
	ImageActionEdit     ImageAction = "edit"
)

// ImageGenerateRequest represents a request to generate images.
type ImageGenerateRequest struct {
	Model  ModelID `json:"model"`
	Prompt string  `json:"prompt"`

	// Optional parameters
	N              int             `json:"n,omitempty"`                  // Number of images to generate (default 1)
	Size           ImageSize       `json:"size,omitempty"`               // Image dimensions
	Quality        ImageQuality    `json:"quality,omitempty"`            // Rendering quality
	Format         ImageFormat     `json:"output_format,omitempty"`      // Output format
	Compression    *int            `json:"output_compression,omitempty"` // 0-100 for jpeg/webp
	Background     ImageBackground `json:"background,omitempty"`         // Transparency setting
	Moderation     string          `json:"moderation,omitempty"`         // "auto" or "low"
	User           string          `json:"user,omitempty"`               // User identifier
	ResponseFormat string          `json:"response_format,omitempty"`    // "b64_json" or "url"
	PartialImages  int             `json:"partial_images,omitempty"`     // For streaming (0-3)
}

// ImageEditRequest represents a request to edit images.
type ImageEditRequest struct {
	Model  ModelID `json:"model"`
	Prompt string  `json:"prompt"`

	// Image inputs - at least one required
	Images []ImageInput `json:"-"` // Handled separately for multipart

	// Optional mask for inpainting
	Mask *ImageInput `json:"-"`

	// Optional parameters
	N             int                `json:"n,omitempty"`
	Size          ImageSize          `json:"size,omitempty"`
	Quality       ImageQuality       `json:"quality,omitempty"`
	Format        ImageFormat        `json:"output_format,omitempty"`
	Compression   *int               `json:"output_compression,omitempty"`
	Background    ImageBackground    `json:"background,omitempty"`
	InputFidelity ImageInputFidelity `json:"input_fidelity,omitempty"`
	User          string             `json:"user,omitempty"`
}

// ImageInput represents an input image for editing.
type ImageInput struct {
	// One of these must be set
	Data     []byte // Raw image bytes
	Base64   string // Base64-encoded image
	URL      string // URL to fetch image from (Responses API only)
	FileID   string // File ID from Files API (Responses API only)
	Filename string // Optional filename hint
}

// GetBytes returns the image data as bytes.
func (i ImageInput) GetBytes() ([]byte, error) {
	if len(i.Data) > 0 {
		return i.Data, nil
	}
	if i.Base64 != "" {
		return base64.StdEncoding.DecodeString(i.Base64)
	}
	return nil, nil // URL/FileID handled by API
}

// ImageResponse represents a response containing generated images.
type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
	Usage   *ImageUsage `json:"usage,omitempty"`
}

// ImageData represents a single generated image.
type ImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// GetBytes decodes and returns the image data.
func (d ImageData) GetBytes() ([]byte, error) {
	if d.B64JSON != "" {
		return base64.StdEncoding.DecodeString(d.B64JSON)
	}
	return nil, nil // URL must be fetched separately
}

// ImageUsage tracks token usage for image generation.
type ImageUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ImageChunk represents an incremental streaming response for images.
type ImageChunk struct {
	PartialImageIndex int    `json:"partial_image_index"`
	B64JSON           string `json:"b64_json"`
}

// ImageStream represents a streaming image generation response.
type ImageStream struct {
	Ch    <-chan ImageChunk     // Partial images
	Err   <-chan error          // At most one error
	Final <-chan *ImageResponse // Complete response
}
