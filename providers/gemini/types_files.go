// Package gemini provides a Google Gemini API provider implementation for Iris.
package gemini

import "io"

// FileState represents the processing state of a file.
type FileState string

const (
	// FileStateUnspecified is the default/omitted state.
	FileStateUnspecified FileState = "STATE_UNSPECIFIED"
	// FileStateProcessing indicates file is being processed.
	FileStateProcessing FileState = "PROCESSING"
	// FileStateActive indicates file is ready for use.
	FileStateActive FileState = "ACTIVE"
	// FileStateFailed indicates processing failed.
	FileStateFailed FileState = "FAILED"
)

// FileSource indicates how the file was created.
type FileSource string

const (
	// FileSourceUnspecified is the default.
	FileSourceUnspecified FileSource = "SOURCE_UNSPECIFIED"
	// FileSourceUploaded indicates user upload.
	FileSourceUploaded FileSource = "UPLOADED"
	// FileSourceGenerated indicates Google generated.
	FileSourceGenerated FileSource = "GENERATED"
)

// File represents a file uploaded to the Gemini API.
type File struct {
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName,omitempty"`
	MimeType       string         `json:"mimeType"`
	SizeBytes      string         `json:"sizeBytes"`
	CreateTime     string         `json:"createTime"`
	UpdateTime     string         `json:"updateTime,omitempty"`
	ExpirationTime string         `json:"expirationTime,omitempty"`
	SHA256Hash     string         `json:"sha256Hash,omitempty"`
	URI            string         `json:"uri"`
	DownloadURI    string         `json:"downloadUri,omitempty"`
	State          FileState      `json:"state"`
	Source         FileSource     `json:"source,omitempty"`
	Error          *FileError     `json:"error,omitempty"`
	VideoMetadata  *VideoMetadata `json:"videoMetadata,omitempty"`
}

// VideoMetadata contains metadata for video files.
type VideoMetadata struct {
	VideoDuration string `json:"videoDuration,omitempty"`
}

// FileError contains error details if processing failed.
type FileError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// FileUploadRequest contains parameters for uploading a file.
type FileUploadRequest struct {
	// File is the reader containing file content.
	File io.Reader
	// DisplayName is the human-readable name (max 512 chars).
	DisplayName string
	// MimeType is the MIME type of the file.
	MimeType string
}

// FileListRequest contains parameters for listing files.
type FileListRequest struct {
	// PageSize is max files per page (default 10, max 100).
	PageSize int
	// PageToken for pagination.
	PageToken string
}

// FileListResponse contains paginated file results.
type FileListResponse struct {
	Files         []File `json:"files"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// fileUploadMetadata is the JSON body for upload initiation.
type fileUploadMetadata struct {
	File struct {
		DisplayName string `json:"display_name,omitempty"`
	} `json:"file"`
}

// fileUploadResponse wraps the upload response.
type fileUploadResponse struct {
	File File `json:"file"`
}
