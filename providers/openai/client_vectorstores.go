package openai

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

// buildHeadersWithBeta returns headers with the OpenAI-Beta header set.
func (p *OpenAI) buildHeadersWithBeta() http.Header {
	headers := p.buildHeaders()
	headers.Set("OpenAI-Beta", "assistants=v2")
	return headers
}

// CreateVectorStore creates a new vector store.
func (p *OpenAI) CreateVectorStore(ctx context.Context, req *VectorStoreCreateRequest) (*VectorStore, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + "/vector_stores"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeadersWithBeta() {
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
		return nil, p.parseVectorStoreError(resp)
	}

	var vs VectorStore
	if err := json.NewDecoder(resp.Body).Decode(&vs); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return &vs, nil
}

// ListVectorStores returns a list of vector stores.
func (p *OpenAI) ListVectorStores(ctx context.Context, req *VectorStoreListRequest) (*VectorStoreListResponse, error) {
	url := p.config.BaseURL + "/vector_stores"

	if req != nil {
		params := make([]string, 0)
		if req.Limit != nil {
			params = append(params, "limit="+strconv.Itoa(*req.Limit))
		}
		if req.After != nil {
			params = append(params, "after="+*req.After)
		}
		if req.Before != nil {
			params = append(params, "before="+*req.Before)
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

	for key, values := range p.buildHeadersWithBeta() {
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
		return nil, p.parseVectorStoreError(resp)
	}

	var result VectorStoreListResponse
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

// GetVectorStore retrieves information about a specific vector store.
func (p *OpenAI) GetVectorStore(ctx context.Context, vectorStoreID string) (*VectorStore, error) {
	url := p.config.BaseURL + "/vector_stores/" + vectorStoreID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeadersWithBeta() {
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
		return nil, p.parseVectorStoreError(resp)
	}

	var vs VectorStore
	if err := json.NewDecoder(resp.Body).Decode(&vs); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return &vs, nil
}

// DeleteVectorStore deletes a vector store.
func (p *OpenAI) DeleteVectorStore(ctx context.Context, vectorStoreID string) error {
	url := p.config.BaseURL + "/vector_stores/" + vectorStoreID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeadersWithBeta() {
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
		return p.parseVectorStoreError(resp)
	}

	var result VectorStoreDeleteResponse
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
			Message:  "vector store was not deleted",
			Err:      core.ErrBadRequest,
		}
	}

	return nil
}

// AddFileToVectorStore adds a file to a vector store.
func (p *OpenAI) AddFileToVectorStore(ctx context.Context, vectorStoreID string, req *VectorStoreFileAddRequest) (*VectorStoreFile, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + "/vector_stores/" + vectorStoreID + "/files"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeadersWithBeta() {
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
		return nil, p.parseVectorStoreError(resp)
	}

	var vsf VectorStoreFile
	if err := json.NewDecoder(resp.Body).Decode(&vsf); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return &vsf, nil
}

// ListVectorStoreFiles returns a list of files in a vector store.
func (p *OpenAI) ListVectorStoreFiles(ctx context.Context, vectorStoreID string, req *VectorStoreFileListRequest) (*VectorStoreFileListResponse, error) {
	url := p.config.BaseURL + "/vector_stores/" + vectorStoreID + "/files"

	if req != nil {
		params := make([]string, 0)
		if req.Limit != nil {
			params = append(params, "limit="+strconv.Itoa(*req.Limit))
		}
		if req.After != nil {
			params = append(params, "after="+*req.After)
		}
		if req.Before != nil {
			params = append(params, "before="+*req.Before)
		}
		if req.Order != nil {
			params = append(params, "order="+*req.Order)
		}
		if req.Filter != nil {
			params = append(params, "filter="+string(*req.Filter))
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeadersWithBeta() {
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
		return nil, p.parseVectorStoreError(resp)
	}

	var result VectorStoreFileListResponse
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

// GetVectorStoreFile retrieves information about a specific file in a vector store.
func (p *OpenAI) GetVectorStoreFile(ctx context.Context, vectorStoreID, fileID string) (*VectorStoreFile, error) {
	url := p.config.BaseURL + "/vector_stores/" + vectorStoreID + "/files/" + fileID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeadersWithBeta() {
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
		return nil, p.parseVectorStoreError(resp)
	}

	var vsf VectorStoreFile
	if err := json.NewDecoder(resp.Body).Decode(&vsf); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return &vsf, nil
}

// DeleteVectorStoreFile deletes a file from a vector store.
func (p *OpenAI) DeleteVectorStoreFile(ctx context.Context, vectorStoreID, fileID string) error {
	url := p.config.BaseURL + "/vector_stores/" + vectorStoreID + "/files/" + fileID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeadersWithBeta() {
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
		return p.parseVectorStoreError(resp)
	}

	var result VectorStoreFileDeleteResponse
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
			Message:  "vector store file was not deleted",
			Err:      core.ErrBadRequest,
		}
	}

	return nil
}

// PollVectorStoreUntilReady polls a vector store until it reaches completed status.
// It returns an error if the vector store expires or if the context is canceled.
func (p *OpenAI) PollVectorStoreUntilReady(ctx context.Context, vectorStoreID string, interval time.Duration) (*VectorStore, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// First check immediately
	vs, err := p.GetVectorStore(ctx, vectorStoreID)
	if err != nil {
		return nil, err
	}

	for vs.Status != VectorStoreStatusCompleted {
		// Check for terminal failure states
		if vs.Status == VectorStoreStatusExpired {
			return nil, &core.ProviderError{
				Provider: "openai",
				Code:     "vector_store_expired",
				Message:  "vector store has expired",
				Err:      core.ErrBadRequest,
			}
		}

		// Wait for next poll or context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			vs, err = p.GetVectorStore(ctx, vectorStoreID)
			if err != nil {
				return nil, err
			}
		}
	}

	return vs, nil
}

// parseVectorStoreError parses an error response from the Vector Stores API.
func (p *OpenAI) parseVectorStoreError(resp *http.Response) error {
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
