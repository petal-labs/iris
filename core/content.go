// Package core provides the Iris SDK client and types.
package core

// ContentPart represents a part of multimodal content in a message.
// Types implementing this interface can be used in multimodal messages
// with the OpenAI Responses API.
type ContentPart interface {
	// ContentType returns the type identifier for this content part.
	ContentType() string
}

// ImageDetail specifies the level of detail for image processing.
type ImageDetail string

const (
	// ImageDetailAuto lets the model decide the appropriate detail level.
	ImageDetailAuto ImageDetail = "auto"
	// ImageDetailLow uses fewer tokens for faster processing.
	ImageDetailLow ImageDetail = "low"
	// ImageDetailHigh uses more tokens for detailed analysis.
	ImageDetailHigh ImageDetail = "high"
)

// InputText represents text content in a multimodal message.
type InputText struct {
	// Text is the text content.
	Text string
}

// ContentType returns the type identifier for InputText.
func (t InputText) ContentType() string {
	return "input_text"
}

// InputImage represents image content in a multimodal message.
type InputImage struct {
	// ImageURL is an HTTPS URL or data URL (data:image/jpeg;base64,...).
	ImageURL string
	// FileID is a file ID from the Files API.
	FileID string
	// Detail specifies the level of detail for image processing.
	Detail ImageDetail
}

// ContentType returns the type identifier for InputImage.
func (i InputImage) ContentType() string {
	return "input_image"
}

// InputFile represents file content in a multimodal message.
type InputFile struct {
	// FileID is a file ID from the Files API.
	FileID string
	// FileURL is an HTTPS URL to the file.
	FileURL string
	// FileData contains base64-encoded file bytes.
	FileData string
	// Filename is the recommended filename when using FileData.
	Filename string
}

// ContentType returns the type identifier for InputFile.
func (f InputFile) ContentType() string {
	return "input_file"
}
