package azurefoundry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// ----------------------------------------------------------------------------
// Embeddings Request/Response Types
// ----------------------------------------------------------------------------

// azureEmbeddingRequest is the request body for embeddings.
type azureEmbeddingRequest struct {
	Model          string   `json:"model,omitempty"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	Dimensions     *int     `json:"dimensions,omitempty"`
	User           string   `json:"user,omitempty"`
	InputType      string   `json:"input_type,omitempty"` // Azure/Cohere: "query" or "document"
}

// azureEmbeddingResponse is the response from embeddings endpoint.
type azureEmbeddingResponse struct {
	Object string               `json:"object"`
	Data   []azureEmbeddingData `json:"data"`
	Model  string               `json:"model"`
	Usage  azureEmbeddingUsage  `json:"usage"`
}

// azureEmbeddingData represents a single embedding in the response.
type azureEmbeddingData struct {
	Object    string `json:"object"`
	Index     int    `json:"index"`
	Embedding any    `json:"embedding"` // []float64 or base64 string
}

// azureEmbeddingUsage contains token usage for the embedding request.
type azureEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ----------------------------------------------------------------------------
// EmbeddingProvider Implementation
// ----------------------------------------------------------------------------

// CreateEmbeddings generates embeddings for the given input texts.
// Implements core.EmbeddingProvider.
func (p *AzureFoundry) CreateEmbeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	// Apply timeout if configured
	if p.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.config.Timeout)
		defer cancel()
	}

	// Build Azure request
	azReq := buildEmbeddingRequest(req)

	// Marshal request body
	body, err := json.Marshal(azReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Build URL
	url, err := p.buildEmbeddingsURL(req.Model)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Set headers
	headers, err := p.buildHeaders(ctx)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	// Execute request
	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Extract request ID
	requestID := extractRequestID(resp.Header)

	// Check for error status
	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	// Parse response
	var azResp azureEmbeddingResponse
	if err := json.Unmarshal(respBody, &azResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapEmbeddingResponse(&azResp, req), nil
}

// buildEmbeddingsURL constructs the URL for embeddings.
func (p *AzureFoundry) buildEmbeddingsURL(model core.ModelID) (string, error) {
	if p.config.UseOpenAIEndpoint {
		// Azure OpenAI format: /openai/deployments/{deployment-id}/embeddings
		deploymentID := p.config.DeploymentID
		if deploymentID == "" {
			deploymentID = string(model)
		}
		if deploymentID == "" {
			return "", ErrDeploymentRequired
		}
		return p.config.Endpoint + "/openai/deployments/" + deploymentID +
			"/embeddings?api-version=" + p.config.APIVersion, nil
	}

	// Model Inference API format: /models/embeddings
	return p.config.Endpoint + "/models/embeddings?api-version=" + p.config.APIVersion, nil
}

// buildEmbeddingRequest converts core request to Azure format.
func buildEmbeddingRequest(req *core.EmbeddingRequest) *azureEmbeddingRequest {
	inputs := make([]string, len(req.Input))
	for i, input := range req.Input {
		inputs[i] = input.Text
	}

	azReq := &azureEmbeddingRequest{
		Model: string(req.Model),
		Input: inputs,
		User:  req.User,
	}

	if req.EncodingFormat != "" {
		azReq.EncodingFormat = string(req.EncodingFormat)
	}
	if req.Dimensions != nil {
		azReq.Dimensions = req.Dimensions
	}
	if req.InputType != "" {
		azReq.InputType = string(req.InputType)
	}

	return azReq
}

// mapEmbeddingResponse converts Azure response to core format.
func mapEmbeddingResponse(resp *azureEmbeddingResponse, req *core.EmbeddingRequest) *core.EmbeddingResponse {
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
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}
}

// Compile-time check that AzureFoundry implements EmbeddingProvider.
var _ core.EmbeddingProvider = (*AzureFoundry)(nil)
