package anthropic

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

// filesPath is the API endpoint for files.
const filesPath = "/v1/files"

// buildFilesHeaders constructs headers for Files API requests.
func (p *Anthropic) buildFilesHeaders() http.Header {
	headers := p.buildHeaders()

	beta := p.config.FilesAPIBeta
	if beta == "" {
		beta = DefaultFilesAPIBeta
	}
	headers.Set("anthropic-beta", beta)

	return headers
}

// UploadFile uploads a file to Anthropic.
func (p *Anthropic) UploadFile(ctx context.Context, req *FileUploadRequest) (*File, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

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

	url := p.config.BaseURL + filesPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildFilesHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newNetworkError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, normalizeError(resp.StatusCode, body, resp.Header.Get("request-id"))
	}

	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, newDecodeError(err)
	}

	return &file, nil
}

// GetFile retrieves metadata for a specific file.
func (p *Anthropic) GetFile(ctx context.Context, fileID string) (*File, error) {
	url := p.config.BaseURL + filesPath + "/" + fileID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildFilesHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newNetworkError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, normalizeError(resp.StatusCode, body, resp.Header.Get("request-id"))
	}

	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, newDecodeError(err)
	}

	return &file, nil
}

// ListFiles returns a paginated list of files.
func (p *Anthropic) ListFiles(ctx context.Context, req *FileListRequest) (*FileListResponse, error) {
	url := p.config.BaseURL + filesPath

	if req != nil {
		params := make([]string, 0)
		if req.Limit != nil {
			params = append(params, "limit="+strconv.Itoa(*req.Limit))
		}
		if req.BeforeID != nil {
			params = append(params, "before_id="+*req.BeforeID)
		}
		if req.AfterID != nil {
			params = append(params, "after_id="+*req.AfterID)
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildFilesHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newNetworkError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, normalizeError(resp.StatusCode, body, resp.Header.Get("request-id"))
	}

	var result FileListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, newDecodeError(err)
	}

	return &result, nil
}

// ListAllFiles returns all files, handling pagination automatically.
func (p *Anthropic) ListAllFiles(ctx context.Context) ([]File, error) {
	var allFiles []File
	var afterID *string
	limit := 1000

	for {
		req := &FileListRequest{
			Limit:   &limit,
			AfterID: afterID,
		}

		resp, err := p.ListFiles(ctx, req)
		if err != nil {
			return nil, err
		}

		allFiles = append(allFiles, resp.Data...)

		if !resp.HasMore {
			break
		}
		afterID = &resp.LastID
	}

	return allFiles, nil
}

// DownloadFile retrieves the content of a file.
// Pre-checks downloadability; returns ErrFileNotDownloadable for user-uploaded files.
// Caller is responsible for closing the returned ReadCloser.
func (p *Anthropic) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	// Pre-check: Get file metadata to verify downloadability
	file, err := p.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if !file.Downloadable {
		return nil, &core.ProviderError{
			Provider: "anthropic",
			Code:     "file_not_downloadable",
			Message:  fmt.Sprintf("file %s is not downloadable (only tool-generated files can be downloaded)", fileID),
			Err:      ErrFileNotDownloadable,
		}
	}

	url := p.config.BaseURL + filesPath + "/" + fileID + "/content"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildFilesHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, normalizeError(resp.StatusCode, body, resp.Header.Get("request-id"))
	}

	return resp.Body, nil
}

// DeleteFile deletes a file.
func (p *Anthropic) DeleteFile(ctx context.Context, fileID string) error {
	url := p.config.BaseURL + filesPath + "/" + fileID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildFilesHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return newNetworkError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return newNetworkError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return normalizeError(resp.StatusCode, body, resp.Header.Get("request-id"))
	}

	return nil
}
