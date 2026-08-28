package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

const (
	messagesTokenCountPath     = "/v1/messages/count_tokens"
	maxTokenCountResponseBytes = 1 << 20
)

type anthropicTokenCountRequest struct {
	Model    string             `json:"model"`
	Messages []anthropicMessage `json:"messages"`
	System   string             `json:"system,omitempty"`
	Tools    []anthropicTool    `json:"tools,omitempty"`
}

type anthropicTokenCountResponse struct {
	InputTokens *int `json:"input_tokens"`
}

// CountTokens returns Anthropic's input-token count for a chat request without
// creating a message.
func (p *Anthropic) CountTokens(ctx context.Context, req *core.ChatRequest) (*core.TokenCountResponse, error) {
	if err := normalize.RequireAPIKey("anthropic", p.config.APIKey); err != nil {
		return nil, err
	}
	antReq, err := buildTokenCountRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeoutx.Apply(ctx, p.config.Timeout)
	defer cancel()

	body, err := json.Marshal(antReq)
	if err != nil {
		return nil, newDecodeError(err)
	}
	return p.sendTokenCountRequest(ctx, body)
}

func (p *Anthropic) sendTokenCountRequest(ctx context.Context, body []byte) (*core.TokenCountResponse, error) {
	endpoint := strings.TrimRight(p.config.BaseURL, "/") + messagesTokenCountPath
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
		return nil, normalizeError(resp.StatusCode, respBody, resp.Header.Get("request-id"), resp.Header)
	}
	return decodeTokenCountResponse(respBody)
}

func decodeTokenCountResponse(body []byte) (*core.TokenCountResponse, error) {
	var antResp anthropicTokenCountResponse
	if err := json.Unmarshal(body, &antResp); err != nil {
		return nil, newDecodeError(err)
	}
	if antResp.InputTokens == nil || *antResp.InputTokens < 0 {
		return nil, newDecodeError(fmt.Errorf("invalid input_tokens in response"))
	}
	return &core.TokenCountResponse{InputTokens: *antResp.InputTokens}, nil
}

func buildTokenCountRequest(ctx context.Context, req *core.ChatRequest) (*anthropicTokenCountRequest, error) {
	if ctx == nil {
		return nil, fmt.Errorf("anthropic: token counting context cannot be nil")
	}
	if req == nil {
		return nil, fmt.Errorf("anthropic: token counting request cannot be nil")
	}
	if strings.TrimSpace(string(req.Model)) == "" {
		return nil, fmt.Errorf("anthropic: token counting model cannot be empty")
	}

	system, messages := mapMessages(req.Messages)
	if len(messages) == 0 {
		return nil, fmt.Errorf("anthropic: token counting messages cannot be empty")
	}

	request := &anthropicTokenCountRequest{
		Model:    string(req.Model),
		Messages: messages,
		System:   system,
	}
	if len(req.Tools) > 0 {
		request.Tools = mapTools(req.Tools)
	}
	return request, nil
}

var _ core.TokenCounter = (*Anthropic)(nil)
