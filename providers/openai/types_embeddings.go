package openai

// openAIEmbeddingRequest is the request body for POST /v1/embeddings.
type openAIEmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	Dimensions     *int     `json:"dimensions,omitempty"`
	User           string   `json:"user,omitempty"`
}

// openAIEmbeddingResponse is the response from POST /v1/embeddings.
type openAIEmbeddingResponse struct {
	Object string                `json:"object"`
	Data   []openAIEmbeddingData `json:"data"`
	Model  string                `json:"model"`
	Usage  openAIEmbeddingUsage  `json:"usage"`
}

// openAIEmbeddingData represents a single embedding in the response.
type openAIEmbeddingData struct {
	Object    string `json:"object"`
	Index     int    `json:"index"`
	Embedding any    `json:"embedding"`
}

// openAIEmbeddingUsage contains token usage for the embedding request.
type openAIEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
