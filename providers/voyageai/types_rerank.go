package voyageai

// voyageRerankRequest is the request body for POST /v1/rerank.
type voyageRerankRequest struct {
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	Model           string   `json:"model"`
	TopK            *int     `json:"top_k,omitempty"`
	ReturnDocuments bool     `json:"return_documents,omitempty"`
	Truncation      *bool    `json:"truncation,omitempty"`
}

// voyageRerankResponse is the response from POST /v1/rerank.
type voyageRerankResponse struct {
	Object string             `json:"object"`
	Data   []voyageRerankData `json:"data"`
	Model  string             `json:"model"`
	Usage  voyageRerankUsage  `json:"usage"`
}

// voyageRerankData represents a single reranking result.
type voyageRerankData struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       string  `json:"document,omitempty"`
}

// voyageRerankUsage contains token usage for the rerank request.
type voyageRerankUsage struct {
	TotalTokens int `json:"total_tokens"`
}
