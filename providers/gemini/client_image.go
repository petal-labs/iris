package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// GenerateImage generates images from a text prompt.
func (p *Gemini) GenerateImage(ctx context.Context, req *core.ImageGenerateRequest) (*core.ImageResponse, error) {
	gemReq := mapImageGenerateRequest(req)

	body, err := json.Marshal(gemReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", p.config.BaseURL, req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, newNetworkError(err)
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

	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody)
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return nil, newDecodeError(err)
	}

	return mapImageResponse(&gemResp), nil
}

// EditImage edits images using a prompt and input images.
func (p *Gemini) EditImage(ctx context.Context, req *core.ImageEditRequest) (*core.ImageResponse, error) {
	gemReq := mapImageEditRequest(req)

	body, err := json.Marshal(gemReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", p.config.BaseURL, req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, newNetworkError(err)
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

	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody)
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return nil, newDecodeError(err)
	}

	return mapImageResponse(&gemResp), nil
}

// StreamImage is not supported by Gemini - returns an error.
func (p *Gemini) StreamImage(ctx context.Context, req *core.ImageGenerateRequest) (*core.ImageStream, error) {
	return nil, &core.ProviderError{
		Provider: "gemini",
		Code:     "not_supported",
		Message:  "Gemini does not support streaming image generation",
		Err:      core.ErrBadRequest,
	}
}
