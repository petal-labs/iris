package core

import "context"

// EncodingFormat specifies the embedding output format.
type EncodingFormat string

const (
	// EncodingFormatFloat returns embeddings as float arrays.
	EncodingFormatFloat EncodingFormat = "float"
	// EncodingFormatBase64 returns embeddings as base64-encoded strings.
	EncodingFormatBase64 EncodingFormat = "base64"
)

// InputType specifies the type of input for retrieval optimization.
type InputType string

const (
	// InputTypeNone uses default embedding without retrieval optimization.
	InputTypeNone InputType = ""
	// InputTypeQuery optimizes embeddings for search queries.
	InputTypeQuery InputType = "query"
	// InputTypeDocument optimizes embeddings for documents being searched.
	InputTypeDocument InputType = "document"
)

// OutputDType specifies the data type for embedding vectors.
type OutputDType string

const (
	// OutputDTypeFloat returns 32-bit floating point numbers (default).
	OutputDTypeFloat OutputDType = "float"
	// OutputDTypeInt8 returns 8-bit signed integers (-128 to 127).
	OutputDTypeInt8 OutputDType = "int8"
	// OutputDTypeUint8 returns 8-bit unsigned integers (0 to 255).
	OutputDTypeUint8 OutputDType = "uint8"
	// OutputDTypeBinary returns bit-packed signed integers.
	OutputDTypeBinary OutputDType = "binary"
	// OutputDTypeUbinary returns bit-packed unsigned integers.
	OutputDTypeUbinary OutputDType = "ubinary"
)

// EmbeddingInput represents a single text to embed with optional metadata.
type EmbeddingInput struct {
	Text     string            `json:"text"`
	ID       string            `json:"id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// EmbeddingRequest represents a request to generate embeddings.
type EmbeddingRequest struct {
	Model          ModelID          `json:"model"`
	Input          []EmbeddingInput `json:"input"`
	EncodingFormat EncodingFormat   `json:"encoding_format,omitempty"`
	Dimensions     *int             `json:"dimensions,omitempty"`
	User           string           `json:"user,omitempty"`
	InputType      InputType        `json:"input_type,omitempty"`
	OutputDType    OutputDType      `json:"output_dtype,omitempty"`
	Truncation     *bool            `json:"truncation,omitempty"`
}

// EmbeddingVector represents a single embedding result.
type EmbeddingVector struct {
	Index     int               `json:"index"`
	ID        string            `json:"id,omitempty"`
	Vector    []float32         `json:"vector,omitempty"`
	VectorB64 string            `json:"vector_b64,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// EmbeddingUsage tracks token consumption for embeddings.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// EmbeddingResponse contains the generated embeddings.
type EmbeddingResponse struct {
	Vectors []EmbeddingVector `json:"vectors"`
	Model   ModelID           `json:"model"`
	Usage   EmbeddingUsage    `json:"usage"`
}

// EmbeddingProvider is an optional interface for providers that support embeddings.
type EmbeddingProvider interface {
	// CreateEmbeddings generates embeddings for the given input texts.
	CreateEmbeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
}
