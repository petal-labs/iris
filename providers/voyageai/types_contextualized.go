package voyageai

// voyageContextualizedRequest is the request body for POST /v1/contextualizedembeddings.
type voyageContextualizedRequest struct {
	Inputs          [][]string `json:"inputs"`
	Model           string     `json:"model"`
	InputType       string     `json:"input_type,omitempty"`
	OutputDimension *int       `json:"output_dimension,omitempty"`
	OutputDType     string     `json:"output_dtype,omitempty"`
	EncodingFormat  string     `json:"encoding_format,omitempty"`
}

// voyageContextualizedResponse is the response from POST /v1/contextualizedembeddings.
type voyageContextualizedResponse struct {
	Object string                        `json:"object"`
	Data   []voyageContextualizedDocData `json:"data"`
	Model  string                        `json:"model"`
	Usage  voyageContextualizedUsage     `json:"usage"`
}

// voyageContextualizedDocData represents embeddings for a single document.
type voyageContextualizedDocData struct {
	Object string                          `json:"object"`
	Index  int                             `json:"index"`
	Data   []voyageContextualizedChunkData `json:"data"`
}

// voyageContextualizedChunkData represents a single chunk embedding.
type voyageContextualizedChunkData struct {
	Object    string `json:"object"`
	Index     int    `json:"index"`
	Embedding any    `json:"embedding"`
}

// voyageContextualizedUsage contains token usage for the contextualized embedding request.
type voyageContextualizedUsage struct {
	TotalTokens int `json:"total_tokens"`
}
