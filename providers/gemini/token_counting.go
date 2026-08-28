package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

const (
	geminiTokenCountPathFormat = "%s/v1beta/models/%s:countTokens"
	maxTokenCountResponseBytes = 1 << 20
)

var geminiTokenCountModelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type geminiTokenCountRequest struct {
	GenerateContentRequest *geminiRequest `json:"generateContentRequest"`
}

type geminiTokenCountResponse struct {
	TotalTokens *int `json:"totalTokens"`
}

// CountTokens returns Gemini's input-token count for a chat request without
// generating content.
func (p *Gemini) CountTokens(ctx context.Context, req *core.ChatRequest) (*core.TokenCountResponse, error) {
	if err := normalize.RequireAPIKey("gemini", p.config.APIKey); err != nil {
		return nil, err
	}
	if err := validateGeminiTokenCountRequest(ctx, req); err != nil {
		return nil, err
	}

	ctx, cancel := timeoutx.Apply(ctx, p.config.Timeout)
	defer cancel()

	body, err := json.Marshal(buildGeminiTokenCountRequest(req))
	if err != nil {
		return nil, newDecodeError(err)
	}
	return p.sendTokenCountRequest(ctx, req.Model, body)
}

func (p *Gemini) sendTokenCountRequest(ctx context.Context, model core.ModelID, body []byte) (*core.TokenCountResponse, error) {
	endpoint := fmt.Sprintf(geminiTokenCountPathFormat, strings.TrimRight(p.config.BaseURL, "/"), model)
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

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenCountResponseBytes+1))
	if err != nil {
		return nil, newNetworkError(err)
	}
	if len(respBody) > maxTokenCountResponseBytes {
		return nil, newDecodeError(fmt.Errorf("token count response exceeds %d bytes", maxTokenCountResponseBytes))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, normalizeError(resp.StatusCode, respBody, resp.Header)
	}
	return decodeTokenCountResponse(respBody)
}

func decodeTokenCountResponse(body []byte) (*core.TokenCountResponse, error) {
	var gemResp geminiTokenCountResponse
	if err := json.Unmarshal(body, &gemResp); err != nil {
		return nil, newDecodeError(err)
	}
	if gemResp.TotalTokens == nil || *gemResp.TotalTokens < 0 {
		return nil, newDecodeError(fmt.Errorf("invalid totalTokens in response"))
	}
	return &core.TokenCountResponse{InputTokens: *gemResp.TotalTokens}, nil
}

func validateGeminiTokenCountRequest(ctx context.Context, req *core.ChatRequest) error {
	if ctx == nil {
		return fmt.Errorf("gemini: token counting context cannot be nil")
	}
	if req == nil {
		return fmt.Errorf("gemini: token counting request cannot be nil")
	}
	if !geminiTokenCountModelPattern.MatchString(string(req.Model)) {
		return fmt.Errorf("gemini: invalid token counting model %q", req.Model)
	}
	if len(req.Messages) == 0 {
		return fmt.Errorf("gemini: token counting messages cannot be empty")
	}
	return nil
}

func buildGeminiTokenCountRequest(req *core.ChatRequest) *geminiTokenCountRequest {
	generateRequest := buildRequest(req)
	generateRequest.Model = "models/" + string(req.Model)
	generateRequest.GenerationConfig = nil
	return &geminiTokenCountRequest{GenerateContentRequest: generateRequest}
}

var _ core.TokenCounter = (*Gemini)(nil)
