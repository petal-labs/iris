package openai

import "io"

// FilePurpose represents the intended use of an uploaded file.
type FilePurpose string

const (
	FilePurposeAssistants FilePurpose = "assistants"
	FilePurposeBatch      FilePurpose = "batch"
	FilePurposeFineTune   FilePurpose = "fine-tune"
	FilePurposeVision     FilePurpose = "vision"
	FilePurposeUserData   FilePurpose = "user_data"
	FilePurposeEvals      FilePurpose = "evals"
)

// File represents an uploaded file in OpenAI.
type File struct {
	ID        string      `json:"id"`
	Object    string      `json:"object"`
	Bytes     int64       `json:"bytes"`
	CreatedAt int64       `json:"created_at"`
	ExpiresAt *int64      `json:"expires_at,omitempty"`
	Filename  string      `json:"filename"`
	Purpose   FilePurpose `json:"purpose"`
}

// ExpiresAfter defines file expiration policy.
type ExpiresAfter struct {
	Anchor  string `json:"anchor"`
	Seconds int    `json:"seconds"`
}

// FileUploadRequest contains parameters for uploading a file.
type FileUploadRequest struct {
	File         io.Reader
	Filename     string
	Purpose      FilePurpose
	ExpiresAfter *ExpiresAfter
}

// FileListRequest contains parameters for listing files.
type FileListRequest struct {
	Purpose *FilePurpose
	Limit   *int
	After   *string
	Order   *string
}

// FileListResponse contains paginated file results.
type FileListResponse struct {
	Object  string `json:"object"`
	Data    []File `json:"data"`
	HasMore bool   `json:"has_more"`
	FirstID string `json:"first_id,omitempty"`
	LastID  string `json:"last_id,omitempty"`
}

// FileDeleteResponse contains the result of a file deletion.
type FileDeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// VectorStoreStatus represents the processing status.
type VectorStoreStatus string

const (
	VectorStoreStatusExpired    VectorStoreStatus = "expired"
	VectorStoreStatusInProgress VectorStoreStatus = "in_progress"
	VectorStoreStatusCompleted  VectorStoreStatus = "completed"
)

// VectorStore represents a vector store for file search.
type VectorStore struct {
	ID           string                `json:"id"`
	Object       string                `json:"object"`
	CreatedAt    int64                 `json:"created_at"`
	Name         string                `json:"name"`
	Description  *string               `json:"description,omitempty"`
	UsageBytes   int64                 `json:"usage_bytes"`
	FileCounts   VectorStoreFileCounts `json:"file_counts"`
	Status       VectorStoreStatus     `json:"status"`
	ExpiresAfter *VectorStoreExpiry    `json:"expires_after,omitempty"`
	ExpiresAt    *int64                `json:"expires_at,omitempty"`
	LastActiveAt *int64                `json:"last_active_at,omitempty"`
	Metadata     map[string]string     `json:"metadata,omitempty"`
}

// VectorStoreFileCounts tracks file processing status.
type VectorStoreFileCounts struct {
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	Cancelled  int `json:"cancelled"`
	Total      int `json:"total"`
}

// VectorStoreExpiry defines vector store expiration policy.
type VectorStoreExpiry struct {
	Anchor string `json:"anchor"`
	Days   int    `json:"days"`
}

// VectorStoreCreateRequest contains parameters for creating a vector store.
type VectorStoreCreateRequest struct {
	Name             string             `json:"name"`
	Description      *string            `json:"description,omitempty"`
	FileIDs          []string           `json:"file_ids,omitempty"`
	ExpiresAfter     *VectorStoreExpiry `json:"expires_after,omitempty"`
	ChunkingStrategy *ChunkingStrategy  `json:"chunking_strategy,omitempty"`
	Metadata         map[string]string  `json:"metadata,omitempty"`
}

// VectorStoreListRequest contains parameters for listing vector stores.
type VectorStoreListRequest struct {
	Limit  *int
	After  *string
	Before *string
	Order  *string
}

// VectorStoreListResponse contains paginated vector store results.
type VectorStoreListResponse struct {
	Object  string        `json:"object"`
	Data    []VectorStore `json:"data"`
	HasMore bool          `json:"has_more"`
	FirstID string        `json:"first_id,omitempty"`
	LastID  string        `json:"last_id,omitempty"`
}

// VectorStoreDeleteResponse contains the result of a vector store deletion.
type VectorStoreDeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// VectorStoreFileStatus represents file processing status within a store.
type VectorStoreFileStatus string

const (
	VectorStoreFileStatusInProgress VectorStoreFileStatus = "in_progress"
	VectorStoreFileStatusCompleted  VectorStoreFileStatus = "completed"
	VectorStoreFileStatusFailed     VectorStoreFileStatus = "failed"
	VectorStoreFileStatusCancelled  VectorStoreFileStatus = "cancelled"
)

// VectorStoreFile represents a file attached to a vector store.
type VectorStoreFile struct {
	ID               string                `json:"id"`
	Object           string                `json:"object"`
	CreatedAt        int64                 `json:"created_at"`
	VectorStoreID    string                `json:"vector_store_id"`
	Status           VectorStoreFileStatus `json:"status"`
	UsageBytes       int64                 `json:"usage_bytes"`
	LastError        *VectorStoreFileError `json:"last_error,omitempty"`
	ChunkingStrategy *ChunkingStrategy     `json:"chunking_strategy,omitempty"`
	Attributes       map[string]any        `json:"attributes,omitempty"`
}

// VectorStoreFileError contains error details for failed file processing.
type VectorStoreFileError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ChunkingStrategy defines how files are chunked for vector stores.
type ChunkingStrategy struct {
	Type   string              `json:"type"`
	Static *StaticChunkingOpts `json:"static,omitempty"`
}

// StaticChunkingOpts contains parameters for static chunking.
type StaticChunkingOpts struct {
	MaxChunkSizeTokens int `json:"max_chunk_size_tokens"`
	ChunkOverlapTokens int `json:"chunk_overlap_tokens"`
}

// VectorStoreFileAddRequest contains parameters for adding a file to a store.
type VectorStoreFileAddRequest struct {
	FileID           string            `json:"file_id"`
	ChunkingStrategy *ChunkingStrategy `json:"chunking_strategy,omitempty"`
	Attributes       map[string]any    `json:"attributes,omitempty"`
}

// VectorStoreFileListRequest contains parameters for listing files in a store.
type VectorStoreFileListRequest struct {
	Limit  *int
	After  *string
	Before *string
	Order  *string
	Filter *VectorStoreFileStatus
}

// VectorStoreFileListResponse contains paginated file results.
type VectorStoreFileListResponse struct {
	Object  string            `json:"object"`
	Data    []VectorStoreFile `json:"data"`
	HasMore bool              `json:"has_more"`
	FirstID string            `json:"first_id,omitempty"`
	LastID  string            `json:"last_id,omitempty"`
}

// VectorStoreFileDeleteResponse contains the result of a vector store file deletion.
type VectorStoreFileDeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}
