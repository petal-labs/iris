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

const embeddingsPath = "/embeddings"

// CreateEmbeddings generates embeddings for the given input texts.
func (p *VoyageAI) CreateEmbeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	voyageReq := buildEmbeddingRequest(req)

	body, err := json.Marshal(voyageReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + embeddingsPath
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

	var voyageResp voyageEmbeddingResponse
	if err := json.Unmarshal(respBody, &voyageResp); err != nil {
		return nil, newDecodeError(err)
	}

	return mapEmbeddingResponse(&voyageResp, req), nil
}

// buildEmbeddingRequest converts core request to Voyage AI format.
func buildEmbeddingRequest(req *core.EmbeddingRequest) *voyageEmbeddingRequest {
	inputs := make([]string, len(req.Input))
	for i, input := range req.Input {
		inputs[i] = input.Text
	}

	voyageReq := &voyageEmbeddingRequest{
		Input: inputs,
		Model: string(req.Model),
	}

	if req.InputType != "" {
		voyageReq.InputType = string(req.InputType)
	}
	if req.Dimensions != nil {
		voyageReq.OutputDimension = req.Dimensions
	}
	if req.OutputDType != "" {
		voyageReq.OutputDType = string(req.OutputDType)
	}
	if req.EncodingFormat != "" {
		voyageReq.EncodingFormat = string(req.EncodingFormat)
	}
	if req.Truncation != nil {
		voyageReq.Truncation = req.Truncation
	}

	return voyageReq
}

// mapEmbeddingResponse converts Voyage AI response to core format.
func mapEmbeddingResponse(resp *voyageEmbeddingResponse, req *core.EmbeddingRequest) *core.EmbeddingResponse {
	vectors := make([]core.EmbeddingVector, len(resp.Data))

	for i, data := range resp.Data {
		vec := core.EmbeddingVector{
			Index: data.Index,
		}

		// Copy ID and Metadata from input if index is valid
		if data.Index >= 0 && data.Index < len(req.Input) {
			vec.ID = req.Input[data.Index].ID
			vec.Metadata = req.Input[data.Index].Metadata
		}

		// Handle embedding based on type (float array or base64 string)
		switch emb := data.Embedding.(type) {
		case []interface{}:
			vec.Vector = make([]float32, len(emb))
			for j, v := range emb {
				if f, ok := v.(float64); ok {
					vec.Vector[j] = float32(f)
				}
			}
		case string:
			vec.VectorB64 = emb
		}

		vectors[i] = vec
	}

	return &core.EmbeddingResponse{
		Vectors: vectors,
		Model:   core.ModelID(resp.Model),
		Usage: core.EmbeddingUsage{
			TotalTokens: resp.Usage.TotalTokens,
		},
	}
}

// Compile-time check that VoyageAI implements EmbeddingProvider.
var _ core.EmbeddingProvider = (*VoyageAI)(nil)
