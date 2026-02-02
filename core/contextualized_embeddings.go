package core

import "context"

// ContextualizedEmbeddingProvider is an optional interface for providers that support
// context-aware chunk embeddings. These embeddings encode not just the local content
// of each chunk, but also context from surrounding chunks in the same document.
type ContextualizedEmbeddingProvider interface {
	// CreateContextualizedEmbeddings generates context-aware embeddings for document chunks.
	// Each inner slice in Inputs represents chunks from a single document.
	CreateContextualizedEmbeddings(ctx context.Context, req *ContextualizedEmbeddingRequest) (*ContextualizedEmbeddingResponse, error)
}

// ContextualizedEmbeddingRequest represents a request to generate contextualized embeddings.
type ContextualizedEmbeddingRequest struct {
	// Model specifies the embedding model to use.
	Model ModelID `json:"model"`

	// Inputs is a list of documents, where each document is a list of chunks.
	// Chunks within the same document are encoded with awareness of each other.
	Inputs [][]string `json:"inputs"`

	// InputType specifies whether inputs are queries or documents.
	InputType InputType `json:"input_type,omitempty"`

	// OutputDimension specifies the number of dimensions for output embeddings.
	OutputDimension *int `json:"output_dimension,omitempty"`

	// OutputDType specifies the data type for embeddings (float, int8, etc.).
	OutputDType OutputDType `json:"output_dtype,omitempty"`

	// EncodingFormat specifies the encoding format (float array or base64).
	EncodingFormat EncodingFormat `json:"encoding_format,omitempty"`
}

// ContextualizedEmbeddingResponse contains the generated contextualized embeddings.
type ContextualizedEmbeddingResponse struct {
	// Embeddings contains the vectors grouped by document, then by chunk.
	// embeddings[i][j] is the embedding for chunk j of document i.
	Embeddings [][]EmbeddingVector `json:"embeddings"`

	// Model is the model that generated the embeddings.
	Model ModelID `json:"model"`

	// Usage contains token consumption information.
	Usage EmbeddingUsage `json:"usage"`
}
