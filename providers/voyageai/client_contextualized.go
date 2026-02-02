package voyageai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
)

const contextualizedEmbeddingsPath = "/contextualizedembeddings"

// CreateContextualizedEmbeddings generates context-aware embeddings for document chunks.
func (p *VoyageAI) CreateContextualizedEmbeddings(ctx context.Context, req *core.ContextualizedEmbeddingRequest) (*core.ContextualizedEmbeddingResponse, error) {
	voyageReq := buildContextualizedRequest(req)

	body, err := json.Marshal(voyageReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + contextualizedEmbeddingsPath
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
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newNetworkError(err)
	}

	requestID := resp.Header.Get("x-request-id")

	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	var voyageResp voyageContextualizedResponse
	if err := json.Unmarshal(respBody, &voyageResp); err != nil {
		return nil, newDecodeError(err)
	}

	return mapContextualizedResponse(&voyageResp), nil
}

// buildContextualizedRequest converts core request to Voyage AI format.
func buildContextualizedRequest(req *core.ContextualizedEmbeddingRequest) *voyageContextualizedRequest {
	voyageReq := &voyageContextualizedRequest{
		Inputs: req.Inputs,
		Model:  string(req.Model),
	}

	if req.InputType != "" {
		voyageReq.InputType = string(req.InputType)
	}
	if req.OutputDimension != nil {
		voyageReq.OutputDimension = req.OutputDimension
	}
	if req.OutputDType != "" {
		voyageReq.OutputDType = string(req.OutputDType)
	}
	if req.EncodingFormat != "" {
		voyageReq.EncodingFormat = string(req.EncodingFormat)
	}

	return voyageReq
}

// mapContextualizedResponse converts Voyage AI response to core format.
func mapContextualizedResponse(resp *voyageContextualizedResponse) *core.ContextualizedEmbeddingResponse {
	embeddings := make([][]core.EmbeddingVector, len(resp.Data))

	for i, docData := range resp.Data {
		embeddings[i] = make([]core.EmbeddingVector, len(docData.Data))

		for j, chunkData := range docData.Data {
			vec := core.EmbeddingVector{
				Index: chunkData.Index,
			}

			// Handle embedding based on type (float array or base64 string)
			switch emb := chunkData.Embedding.(type) {
			case []interface{}:
				vec.Vector = make([]float32, len(emb))
				for k, v := range emb {
					if f, ok := v.(float64); ok {
						vec.Vector[k] = float32(f)
					}
				}
			case string:
				vec.VectorB64 = emb
			}

			embeddings[i][j] = vec
		}
	}

	return &core.ContextualizedEmbeddingResponse{
		Embeddings: embeddings,
		Model:      core.ModelID(resp.Model),
		Usage: core.EmbeddingUsage{
			TotalTokens: resp.Usage.TotalTokens,
		},
	}
}

// Compile-time check that VoyageAI implements ContextualizedEmbeddingProvider.
var _ core.ContextualizedEmbeddingProvider = (*VoyageAI)(nil)
