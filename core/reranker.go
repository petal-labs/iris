package core

import "context"

// RerankerProvider is an optional interface for providers that support
// semantic reranking of documents based on query relevance.
type RerankerProvider interface {
	// Rerank scores and sorts documents by relevance to the query.
	// Results are returned in descending order of relevance.
	Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error)
}

// RerankRequest represents a request to rerank documents.
type RerankRequest struct {
	// Model specifies the reranker model to use.
	Model ModelID `json:"model"`

	// Query is the search query to rank documents against.
	Query string `json:"query"`

	// Documents is the list of documents to rerank.
	Documents []string `json:"documents"`

	// TopK limits the number of results to return. If nil, returns all.
	TopK *int `json:"top_k,omitempty"`

	// ReturnDocuments includes document text in the response if true.
	ReturnDocuments bool `json:"return_documents,omitempty"`

	// Truncation controls whether to truncate inputs exceeding context length.
	// Defaults to true.
	Truncation *bool `json:"truncation,omitempty"`
}

// RerankResponse contains the reranking results.
type RerankResponse struct {
	// Results contains the reranked documents sorted by descending relevance.
	Results []RerankResult `json:"results"`

	// Model is the model that performed the reranking.
	Model ModelID `json:"model"`

	// Usage contains token consumption information.
	Usage RerankUsage `json:"usage"`
}

// RerankResult represents a single document's reranking result.
type RerankResult struct {
	// Index is the original position in the input documents slice.
	Index int `json:"index"`

	// RelevanceScore is the relevance score (higher is more relevant).
	RelevanceScore float64 `json:"relevance_score"`

	// Document contains the document text if ReturnDocuments was true.
	Document string `json:"document,omitempty"`
}

// RerankUsage tracks token consumption for a rerank request.
type RerankUsage struct {
	// TotalTokens is the total number of tokens used.
	TotalTokens int `json:"total_tokens"`
}
