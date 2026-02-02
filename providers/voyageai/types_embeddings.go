package voyageai

// voyageEmbeddingRequest is the request body for POST /v1/embeddings.
type voyageEmbeddingRequest struct {
	Input           []string `json:"input"`
	Model           string   `json:"model"`
	InputType       string   `json:"input_type,omitempty"`
	Truncation      *bool    `json:"truncation,omitempty"`
	OutputDimension *int     `json:"output_dimension,omitempty"`
	OutputDType     string   `json:"output_dtype,omitempty"`
	EncodingFormat  string   `json:"encoding_format,omitempty"`
}

// voyageEmbeddingResponse is the response from POST /v1/embeddings.
type voyageEmbeddingResponse struct {
	Object string                `json:"object"`
	Data   []voyageEmbeddingData `json:"data"`
	Model  string                `json:"model"`
	Usage  voyageEmbeddingUsage  `json:"usage"`
}

// voyageEmbeddingData represents a single embedding in the response.
type voyageEmbeddingData struct {
	Object    string `json:"object"`
	Index     int    `json:"index"`
	Embedding any    `json:"embedding"`
}

// voyageEmbeddingUsage contains token usage for the embedding request.
type voyageEmbeddingUsage struct {
	TotalTokens int `json:"total_tokens"`
}
