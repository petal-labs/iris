package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// embedPath is the Ollama native embeddings endpoint.
const embedPath = "/api/embed"

// ollamaEmbedRequest is the request body for the Ollama /api/embed endpoint.
type ollamaEmbedRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Truncate   *bool    `json:"truncate,omitempty"`
	Dimensions *int     `json:"dimensions,omitempty"`
}

// ollamaEmbedResponse is the response body from the Ollama /api/embed endpoint.
type ollamaEmbedResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float32 `json:"embeddings"`
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
	Error           string      `json:"error,omitempty"`
}

// CreateEmbeddings generates embeddings for the given input texts using the
// Ollama /api/embed endpoint. All inputs are sent as a single batch and the
// returned vectors preserve input order, IDs, and metadata.
func (p *Ollama) CreateEmbeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	ollamaReq := buildEmbedRequest(req)

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + embedPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &core.ProviderError{
			Provider: "ollama",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	var embedResp ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if embedResp.Error != "" {
		return nil, mapOllamaError(resp.StatusCode, embedResp.Error)
	}

	return mapEmbedResponse(&embedResp, req), nil
}

// buildEmbedRequest converts a core embedding request into the Ollama format.
func buildEmbedRequest(req *core.EmbeddingRequest) *ollamaEmbedRequest {
	inputs := make([]string, len(req.Input))
	for i, input := range req.Input {
		inputs[i] = input.Text
	}

	return &ollamaEmbedRequest{
		Model:      string(req.Model),
		Input:      inputs,
		Truncate:   req.Truncation,
		Dimensions: req.Dimensions,
	}
}

// mapEmbedResponse converts an Ollama embed response into the core format,
// preserving input order and carrying through each input's ID and metadata.
func mapEmbedResponse(resp *ollamaEmbedResponse, req *core.EmbeddingRequest) *core.EmbeddingResponse {
	vectors := make([]core.EmbeddingVector, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		vec := core.EmbeddingVector{
			Index:  i,
			Vector: emb,
		}
		if i < len(req.Input) {
			vec.ID = req.Input[i].ID
			vec.Metadata = req.Input[i].Metadata
		}
		vectors[i] = vec
	}

	model := core.ModelID(resp.Model)
	if model == "" {
		model = req.Model
	}

	return &core.EmbeddingResponse{
		Vectors: vectors,
		Model:   model,
		Usage: core.EmbeddingUsage{
			PromptTokens: resp.PromptEvalCount,
			TotalTokens:  resp.PromptEvalCount,
		},
	}
}

// Compile-time check that Ollama implements EmbeddingProvider.
var _ core.EmbeddingProvider = (*Ollama)(nil)
