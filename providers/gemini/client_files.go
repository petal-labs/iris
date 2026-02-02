package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
)

const (
	filesPath       = "/v1beta/files"
	filesUploadPath = "/upload/v1beta/files"
)

// UploadFile uploads a file to Gemini using resumable upload protocol.
// Files are stored for 48 hours before automatic deletion.
func (p *Gemini) UploadFile(ctx context.Context, req *FileUploadRequest) (*File, error) {
	// Step 1: Initiate resumable upload
	uploadURL, err := p.initiateResumableUpload(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate upload: %w", err)
	}

	// Step 2: Upload file content
	return p.uploadFileContent(ctx, uploadURL, req)
}

func (p *Gemini) initiateResumableUpload(ctx context.Context, req *FileUploadRequest) (string, error) {
	url := p.config.BaseURL + filesUploadPath

	metadata := fileUploadMetadata{}
	if req.DisplayName != "" {
		metadata.File.DisplayName = req.DisplayName
	}

	body, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", p.config.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Goog-Upload-Protocol", "resumable")
	httpReq.Header.Set("X-Goog-Upload-Command", "start")
	if req.MimeType != "" {
		httpReq.Header.Set("X-Goog-Upload-Header-Content-Type", req.MimeType)
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return "", newNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", normalizeError(resp.StatusCode, respBody)
	}

	uploadURL := resp.Header.Get("X-Goog-Upload-URL")
	if uploadURL == "" {
		return "", fmt.Errorf("no upload URL in response headers")
	}

	return uploadURL, nil
}

func (p *Gemini) uploadFileContent(ctx context.Context, uploadURL string, req *FileUploadRequest) (*File, error) {
	// Read all content to determine size
	content, err := io.ReadAll(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}

	httpReq.Header.Set("Content-Length", strconv.Itoa(len(content)))
	httpReq.Header.Set("X-Goog-Upload-Offset", "0")
	httpReq.Header.Set("X-Goog-Upload-Command", "upload, finalize")

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
		return nil, normalizeError(resp.StatusCode, body)
	}

	var result fileUploadResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, newDecodeError(err)
	}

	return &result.File, nil
}

// GetFile retrieves metadata for a specific file.
func (p *Gemini) GetFile(ctx context.Context, name string) (*File, error) {
	url := p.config.BaseURL + "/v1beta/" + name

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", p.config.APIKey)

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
		return nil, normalizeError(resp.StatusCode, body)
	}

	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, newDecodeError(err)
	}

	return &file, nil
}

// ListFiles returns a paginated list of files.
func (p *Gemini) ListFiles(ctx context.Context, req *FileListRequest) (*FileListResponse, error) {
	url := p.config.BaseURL + filesPath

	if req != nil {
		params := make([]string, 0)
		if req.PageSize > 0 {
			params = append(params, "pageSize="+strconv.Itoa(req.PageSize))
		}
		if req.PageToken != "" {
			params = append(params, "pageToken="+req.PageToken)
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", p.config.APIKey)

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
		return nil, normalizeError(resp.StatusCode, body)
	}

	var result FileListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, newDecodeError(err)
	}

	return &result, nil
}

// ListAllFiles returns all files, handling pagination automatically.
func (p *Gemini) ListAllFiles(ctx context.Context) ([]File, error) {
	var allFiles []File
	var pageToken string

	for {
		req := &FileListRequest{
			PageSize:  100,
			PageToken: pageToken,
		}

		resp, err := p.ListFiles(ctx, req)
		if err != nil {
			return nil, err
		}

		allFiles = append(allFiles, resp.Files...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return allFiles, nil
}

// DeleteFile deletes a file.
func (p *Gemini) DeleteFile(ctx context.Context, name string) error {
	url := p.config.BaseURL + "/v1beta/" + name

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", p.config.APIKey)

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return newNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return normalizeError(resp.StatusCode, body)
	}

	return nil
}

// WaitForFileActive polls until file reaches ACTIVE state or fails.
// Returns ErrFileFailed if processing fails.
func (p *Gemini) WaitForFileActive(ctx context.Context, name string) (*File, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// First check immediately
	file, err := p.GetFile(ctx, name)
	if err != nil {
		return nil, err
	}

	for {
		switch file.State {
		case FileStateActive:
			return file, nil
		case FileStateFailed:
			msg := "file processing failed"
			if file.Error != nil {
				msg = file.Error.Message
			}
			return nil, &core.ProviderError{
				Provider: "gemini",
				Code:     "file_failed",
				Message:  msg,
				Err:      ErrFileFailed,
			}
		}

		// Wait for next poll or context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			file, err = p.GetFile(ctx, name)
			if err != nil {
				return nil, err
			}
		}
	}
}
