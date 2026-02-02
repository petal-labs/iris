package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/petal-labs/iris/core"
)

// UploadFile uploads a file to OpenAI.
func (p *OpenAI) UploadFile(ctx context.Context, req *FileUploadRequest) (*File, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Add purpose field
	if err := w.WriteField("purpose", string(req.Purpose)); err != nil {
		return nil, fmt.Errorf("failed to write purpose field: %w", err)
	}

	// Add expires_after fields if provided
	if req.ExpiresAfter != nil {
		if err := w.WriteField("expires_after[anchor]", req.ExpiresAfter.Anchor); err != nil {
			return nil, fmt.Errorf("failed to write expires_after[anchor] field: %w", err)
		}
		if err := w.WriteField("expires_after[seconds]", strconv.Itoa(req.ExpiresAfter.Seconds)); err != nil {
			return nil, fmt.Errorf("failed to write expires_after[seconds] field: %w", err)
		}
	}

	// Add file field
	part, err := w.CreateFormFile("file", req.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, req.File); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := p.config.BaseURL + "/files"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers (but override Content-Type for multipart)
	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseFileError(resp)
	}

	var file File
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return &file, nil
}

// parseFileError parses an error response from the Files API.
func (p *OpenAI) parseFileError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return &core.ProviderError{
			Provider: "openai",
			Status:   resp.StatusCode,
			Code:     "unknown",
			Message:  string(body),
			Err:      p.mapStatusToSentinel(resp.StatusCode),
		}
	}

	return &core.ProviderError{
		Provider: "openai",
		Status:   resp.StatusCode,
		Code:     errResp.Error.Code,
		Message:  errResp.Error.Message,
		Err:      p.mapStatusToSentinel(resp.StatusCode),
	}
}

// mapStatusToSentinel maps HTTP status codes to sentinel errors.
func (p *OpenAI) mapStatusToSentinel(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return core.ErrUnauthorized
	case http.StatusNotFound:
		return core.ErrNotFound
	case http.StatusTooManyRequests:
		return core.ErrRateLimited
	case http.StatusBadRequest:
		return core.ErrBadRequest
	default:
		if status >= 500 {
			return core.ErrServer
		}
		return core.ErrBadRequest
	}
}

// ListFiles returns a list of files.
func (p *OpenAI) ListFiles(ctx context.Context, req *FileListRequest) (*FileListResponse, error) {
	url := p.config.BaseURL + "/files"

	// Build query parameters
	if req != nil {
		params := make([]string, 0)
		if req.Purpose != nil {
			params = append(params, "purpose="+string(*req.Purpose))
		}
		if req.Limit != nil {
			params = append(params, "limit="+strconv.Itoa(*req.Limit))
		}
		if req.After != nil {
			params = append(params, "after="+*req.After)
		}
		if req.Order != nil {
			params = append(params, "order="+*req.Order)
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseFileError(resp)
	}

	var result FileListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return &result, nil
}

// GetFile retrieves information about a specific file.
func (p *OpenAI) GetFile(ctx context.Context, fileID string) (*File, error) {
	url := p.config.BaseURL + "/files/" + fileID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseFileError(resp)
	}

	var file File
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return &file, nil
}

// DownloadFile retrieves the content of a file.
// The caller is responsible for closing the returned ReadCloser.
func (p *OpenAI) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	url := p.config.BaseURL + "/files/" + fileID + "/content"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, p.parseFileError(resp)
	}

	return resp.Body, nil
}

// DeleteFile deletes a file.
func (p *OpenAI) DeleteFile(ctx context.Context, fileID string) error {
	url := p.config.BaseURL + "/files/" + fileID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.parseFileError(resp)
	}

	var result FileDeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	if !result.Deleted {
		return &core.ProviderError{
			Provider: "openai",
			Code:     "delete_failed",
			Message:  "file was not deleted",
			Err:      core.ErrBadRequest,
		}
	}

	return nil
}
