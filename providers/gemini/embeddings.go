package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

const (
	geminiBatchEmbeddingPathFormat = "%s/v1beta/models/%s:batchEmbedContents"
	geminiEmbeddingTaskQuery       = "RETRIEVAL_QUERY"
	geminiEmbeddingTaskDocument    = "RETRIEVAL_DOCUMENT"
)

type geminiBatchEmbeddingRequest struct {
	Requests []geminiEmbeddingRequest `json:"requests"`
}

type geminiEmbeddingRequest struct {
	Model                string        `json:"model"`
	Content              geminiContent `json:"content"`
	TaskType             string        `json:"taskType,omitempty"`
	OutputDimensionality *int          `json:"outputDimensionality,omitempty"`
}

type geminiBatchEmbeddingResponse struct {
	Embeddings    []geminiContentEmbedding `json:"embeddings"`
	UsageMetadata geminiEmbeddingUsage     `json:"usageMetadata"`
}

type geminiContentEmbedding struct {
	Values []float32 `json:"values"`
}

type geminiEmbeddingUsage struct {
	PromptTokenCount int `json:"promptTokenCount"`
}

var geminiEmbeddingModelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// CreateEmbeddings generates embeddings for one or more text inputs through
// Gemini's synchronous batchEmbedContents endpoint.
func (p *Gemini) CreateEmbeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if err := normalize.RequireAPIKey("gemini", p.config.APIKey); err != nil {
		return nil, err
	}
	if err := validateGeminiEmbeddingRequest(ctx, req); err != nil {
		return nil, err
	}

	ctx, cancel := timeoutx.Apply(ctx, p.config.Timeout)
	defer cancel()

	body, err := json.Marshal(buildGeminiEmbeddingRequest(req))
	if err != nil {
		return nil, newDecodeError(err)
	}

	response, err := p.sendEmbeddingRequest(ctx, req.Model, body)
	if err != nil {
		return nil, err
	}
	return mapGeminiEmbeddingResponse(response, req)
}

func (p *Gemini) sendEmbeddingRequest(ctx context.Context, model core.ModelID, body []byte) (*geminiBatchEmbeddingResponse, error) {
	endpoint := fmt.Sprintf(geminiBatchEmbeddingPathFormat, strings.TrimRight(p.config.BaseURL, "/"), model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, newNetworkError(err)
	}
	for key, values := range p.buildHeaders() {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()
	return decodeGeminiEmbeddingResponse(resp)
}

func decodeGeminiEmbeddingResponse(resp *http.Response) (*geminiBatchEmbeddingResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newNetworkError(err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, normalizeError(resp.StatusCode, body)
	}

	var response geminiBatchEmbeddingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, newDecodeError(err)
	}
	return &response, nil
}

func validateGeminiEmbeddingRequest(ctx context.Context, req *core.EmbeddingRequest) error {
	if ctx == nil {
		return fmt.Errorf("gemini: embedding context cannot be nil")
	}
	if req == nil {
		return fmt.Errorf("gemini: embedding request cannot be nil")
	}
	if !geminiEmbeddingModelPattern.MatchString(string(req.Model)) {
		return fmt.Errorf("gemini: invalid embedding model %q", req.Model)
	}
	if len(req.Input) == 0 {
		return fmt.Errorf("gemini: embedding input cannot be empty")
	}
	for i, input := range req.Input {
		if strings.TrimSpace(input.Text) == "" {
			return fmt.Errorf("gemini: embedding input %d cannot be blank", i)
		}
	}
	if req.Dimensions != nil && *req.Dimensions <= 0 {
		return fmt.Errorf("gemini: embedding dimensions must be positive")
	}
	if req.User != "" {
		return fmt.Errorf("gemini: embedding user identifier: %w", core.ErrNotSupported)
	}
	if req.Truncation != nil {
		return fmt.Errorf("gemini: embedding truncation: %w", core.ErrNotSupported)
	}
	if req.InputType != core.InputTypeNone && req.InputType != core.InputTypeQuery && req.InputType != core.InputTypeDocument {
		return fmt.Errorf("gemini: unsupported embedding input type %q", req.InputType)
	}
	if req.EncodingFormat != "" && req.EncodingFormat != core.EncodingFormatFloat {
		return fmt.Errorf("gemini: embedding encoding format %q: %w", req.EncodingFormat, core.ErrNotSupported)
	}
	if req.OutputDType != "" && req.OutputDType != core.OutputDTypeFloat {
		return fmt.Errorf("gemini: embedding output type %q: %w", req.OutputDType, core.ErrNotSupported)
	}
	return nil
}

func buildGeminiEmbeddingRequest(req *core.EmbeddingRequest) *geminiBatchEmbeddingRequest {
	requests := make([]geminiEmbeddingRequest, len(req.Input))
	taskType := mapGeminiEmbeddingTaskType(req.InputType)
	for i, input := range req.Input {
		requests[i] = geminiEmbeddingRequest{
			Model:                "models/" + string(req.Model),
			Content:              geminiContent{Parts: []geminiPart{{Text: input.Text}}},
			TaskType:             taskType,
			OutputDimensionality: req.Dimensions,
		}
	}
	return &geminiBatchEmbeddingRequest{Requests: requests}
}

func mapGeminiEmbeddingTaskType(inputType core.InputType) string {
	switch inputType {
	case core.InputTypeQuery:
		return geminiEmbeddingTaskQuery
	case core.InputTypeDocument:
		return geminiEmbeddingTaskDocument
	default:
		return ""
	}
}

func mapGeminiEmbeddingResponse(resp *geminiBatchEmbeddingResponse, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if len(resp.Embeddings) != len(req.Input) {
		return nil, fmt.Errorf("gemini: embedding response count %d does not match input count %d", len(resp.Embeddings), len(req.Input))
	}

	vectors := make([]core.EmbeddingVector, len(resp.Embeddings))
	for i, embedding := range resp.Embeddings {
		vectors[i] = core.EmbeddingVector{
			Index:    i,
			ID:       req.Input[i].ID,
			Metadata: maps.Clone(req.Input[i].Metadata),
			Vector:   embedding.Values,
		}
	}
	tokens := resp.UsageMetadata.PromptTokenCount
	return &core.EmbeddingResponse{
		Vectors: vectors,
		Model:   req.Model,
		Usage: core.EmbeddingUsage{
			PromptTokens: tokens,
			TotalTokens:  tokens,
		},
	}, nil
}
