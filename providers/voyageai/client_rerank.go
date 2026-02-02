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

const rerankPath = "/rerank"

// Rerank scores and sorts documents by relevance to the query.
func (p *VoyageAI) Rerank(ctx context.Context, req *core.RerankRequest) (*core.RerankResponse, error) {
	voyageReq := buildRerankRequest(req)

	body, err := json.Marshal(voyageReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + rerankPath
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

	var voyageResp voyageRerankResponse
	if err := json.Unmarshal(respBody, &voyageResp); err != nil {
		return nil, newDecodeError(err)
	}

	return mapRerankResponse(&voyageResp), nil
}

// buildRerankRequest converts core request to Voyage AI format.
func buildRerankRequest(req *core.RerankRequest) *voyageRerankRequest {
	voyageReq := &voyageRerankRequest{
		Query:           req.Query,
		Documents:       req.Documents,
		Model:           string(req.Model),
		ReturnDocuments: req.ReturnDocuments,
	}

	if req.TopK != nil {
		voyageReq.TopK = req.TopK
	}
	if req.Truncation != nil {
		voyageReq.Truncation = req.Truncation
	}

	return voyageReq
}

// mapRerankResponse converts Voyage AI response to core format.
func mapRerankResponse(resp *voyageRerankResponse) *core.RerankResponse {
	results := make([]core.RerankResult, len(resp.Data))

	for i, data := range resp.Data {
		results[i] = core.RerankResult{
			Index:          data.Index,
			RelevanceScore: data.RelevanceScore,
			Document:       data.Document,
		}
	}

	return &core.RerankResponse{
		Results: results,
		Model:   core.ModelID(resp.Model),
		Usage: core.RerankUsage{
			TotalTokens: resp.Usage.TotalTokens,
		},
	}
}

// Compile-time check that VoyageAI implements RerankerProvider.
var _ core.RerankerProvider = (*VoyageAI)(nil)
