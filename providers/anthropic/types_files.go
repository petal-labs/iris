package anthropic

import "io"

// File represents an uploaded file in Anthropic.
type File struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	CreatedAt    string `json:"created_at"`
	Downloadable bool   `json:"downloadable"`
}

// FileUploadRequest contains parameters for uploading a file.
type FileUploadRequest struct {
	File     io.Reader
	Filename string
	MimeType string
	Metadata map[string]string
}

// FileListRequest contains parameters for listing files.
type FileListRequest struct {
	Limit    *int
	BeforeID *string
	AfterID  *string
}

// FileListResponse contains paginated file results.
type FileListResponse struct {
	Data    []File `json:"data"`
	FirstID string `json:"first_id"`
	LastID  string `json:"last_id"`
	HasMore bool   `json:"has_more"`
}

// FileDeleteResponse contains the result of a file deletion.
type FileDeleteResponse struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
