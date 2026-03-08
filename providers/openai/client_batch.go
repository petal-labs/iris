package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/petal-labs/iris/core"
)

// batchesPath is the API path for batch operations.
const batchesPath = "/batches"

// CreateBatch submits requests for asynchronous batch processing.
// The requests are serialized to JSONL, uploaded as a file, and submitted as a batch.
func (p *OpenAI) CreateBatch(ctx context.Context, requests []core.BatchRequest) (core.BatchID, error) {
	if len(requests) == 0 {
		return "", &core.ProviderError{
			Provider: "openai",
			Code:     "invalid_request",
			Message:  "at least one request is required",
			Err:      core.ErrBadRequest,
		}
	}

	// Convert requests to JSONL format
	jsonlData, err := p.buildBatchJSONL(requests)
	if err != nil {
		return "", err
	}

	// Upload the JSONL file
	file, err := p.UploadFile(ctx, &FileUploadRequest{
		File:     bytes.NewReader(jsonlData),
		Filename: "batch_input.jsonl",
		Purpose:  FilePurposeBatch,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload batch file: %w", err)
	}

	// Create the batch
	createReq := openAIBatchCreateRequest{
		InputFileID:      file.ID,
		Endpoint:         batchEndpoint,
		CompletionWindow: "24h",
	}

	body, err := json.Marshal(createReq)
	if err != nil {
		return "", &core.ProviderError{
			Provider: "openai",
			Code:     "encode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	url := p.config.BaseURL + batchesPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}

	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return "", &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", p.parseFileError(resp)
	}

	var batch openAIBatch
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return "", &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return core.BatchID(batch.ID), nil
}

// GetBatchStatus returns the current status of a batch.
func (p *OpenAI) GetBatchStatus(ctx context.Context, id core.BatchID) (*core.BatchInfo, error) {
	url := p.config.BaseURL + batchesPath + "/" + string(id)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
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
		if resp.StatusCode == http.StatusNotFound {
			return nil, core.ErrBatchNotFound
		}
		return nil, p.parseFileError(resp)
	}

	var batch openAIBatch
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	return p.mapBatchInfo(&batch), nil
}

// GetBatchResults retrieves completed batch results.
func (p *OpenAI) GetBatchResults(ctx context.Context, id core.BatchID) ([]core.BatchResult, error) {
	// First get the batch to find the output file ID
	info, err := p.GetBatchStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	if info.OutputFileID == "" {
		if info.Status == core.BatchStatusFailed || info.Status == core.BatchStatusCancelled {
			return nil, &core.ProviderError{
				Provider: "openai",
				Code:     "batch_" + string(info.Status),
				Message:  "batch did not produce output",
				Err:      core.ErrBadRequest,
			}
		}
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "batch_not_complete",
			Message:  "batch has not completed processing",
			Err:      core.ErrBadRequest,
		}
	}

	// Download the output file
	content, err := p.DownloadFile(ctx, info.OutputFileID)
	if err != nil {
		return nil, fmt.Errorf("failed to download batch results: %w", err)
	}
	defer content.Close()

	// Parse JSONL output
	return p.parseBatchResults(content)
}

// CancelBatch cancels a pending or in-progress batch.
func (p *OpenAI) CancelBatch(ctx context.Context, id core.BatchID) error {
	url := p.config.BaseURL + batchesPath + "/" + string(id) + "/cancel"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
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
		if resp.StatusCode == http.StatusNotFound {
			return core.ErrBatchNotFound
		}
		return p.parseFileError(resp)
	}

	return nil
}

// ListBatches returns all batches for the account.
func (p *OpenAI) ListBatches(ctx context.Context, limit int) ([]core.BatchInfo, error) {
	url := p.config.BaseURL + batchesPath
	if limit > 0 {
		url += "?limit=" + strconv.Itoa(limit)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
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

	var listResp openAIBatchListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  err.Error(),
			Err:      core.ErrDecode,
		}
	}

	results := make([]core.BatchInfo, len(listResp.Data))
	for i, batch := range listResp.Data {
		results[i] = *p.mapBatchInfo(&batch)
	}

	return results, nil
}

// buildBatchJSONL converts batch requests to JSONL format.
func (p *OpenAI) buildBatchJSONL(requests []core.BatchRequest) ([]byte, error) {
	var buf bytes.Buffer

	for _, req := range requests {
		// Build OpenAI request body
		oaiReq := buildRequest(&req.Request, false)

		line := openAIBatchRequestLine{
			CustomID: req.CustomID,
			Method:   "POST",
			URL:      batchEndpoint,
			Body: openAIBatchReqBody{
				Model:       oaiReq.Model,
				Messages:    oaiReq.Messages,
				Temperature: oaiReq.Temperature,
				MaxTokens:   oaiReq.MaxTokens,
				Tools:       oaiReq.Tools,
			},
		}

		lineBytes, err := json.Marshal(line)
		if err != nil {
			return nil, &core.ProviderError{
				Provider: "openai",
				Code:     "encode_error",
				Message:  fmt.Sprintf("failed to marshal request %s: %v", req.CustomID, err),
				Err:      core.ErrDecode,
			}
		}

		buf.Write(lineBytes)
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

// parseBatchResults parses JSONL output from a batch.
func (p *OpenAI) parseBatchResults(r io.Reader) ([]core.BatchResult, error) {
	var results []core.BatchResult
	scanner := bufio.NewScanner(r)

	// Increase buffer size for large responses
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var respLine openAIBatchResponseLine
		if err := json.Unmarshal(line, &respLine); err != nil {
			return nil, &core.ProviderError{
				Provider: "openai",
				Code:     "decode_error",
				Message:  fmt.Sprintf("failed to parse response line: %v", err),
				Err:      core.ErrDecode,
			}
		}

		result := core.BatchResult{
			CustomID: respLine.CustomID,
		}

		if respLine.Error != nil {
			result.Error = &core.BatchError{
				Code:    respLine.Error.Code,
				Message: respLine.Error.Message,
			}
		} else if respLine.Response != nil && respLine.Response.StatusCode == 200 {
			// Parse the response body
			var oaiResp openAIResponse
			if err := json.Unmarshal(respLine.Response.Body, &oaiResp); err != nil {
				result.Error = &core.BatchError{
					Code:    "decode_error",
					Message: err.Error(),
				}
			} else {
				chatResp, err := mapResponse(&oaiResp)
				if err != nil {
					result.Error = &core.BatchError{
						Code:    "decode_error",
						Message: err.Error(),
					}
				} else {
					result.Response = chatResp
				}
			}
		} else if respLine.Response != nil {
			result.Error = &core.BatchError{
				Code:    fmt.Sprintf("http_%d", respLine.Response.StatusCode),
				Message: fmt.Sprintf("request failed with status %d", respLine.Response.StatusCode),
			}
		}

		results = append(results, result)
	}

	if err := scanner.Err(); err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "decode_error",
			Message:  fmt.Sprintf("error reading batch results: %v", err),
			Err:      core.ErrDecode,
		}
	}

	return results, nil
}

// mapBatchInfo converts an OpenAI batch to core.BatchInfo.
func (p *OpenAI) mapBatchInfo(batch *openAIBatch) *core.BatchInfo {
	info := &core.BatchInfo{
		ID:           core.BatchID(batch.ID),
		Status:       p.mapBatchStatus(batch.Status),
		Total:        batch.RequestCounts.Total,
		Completed:    batch.RequestCounts.Completed,
		Failed:       batch.RequestCounts.Failed,
		CreatedAt:    batch.CreatedAt,
		Endpoint:     batch.Endpoint,
		OutputFileID: batch.OutputFileID,
		ErrorFileID:  batch.ErrorFileID,
	}

	if batch.CompletedAt != nil {
		info.CompletedAt = batch.CompletedAt
	}
	if batch.ExpiresAt != nil {
		info.ExpiresAt = batch.ExpiresAt
	}

	return info
}

// mapBatchStatus converts OpenAI batch status to core.BatchStatus.
func (p *OpenAI) mapBatchStatus(status string) core.BatchStatus {
	switch status {
	case "validating", "pending":
		return core.BatchStatusPending
	case "in_progress", "finalizing":
		return core.BatchStatusInProgress
	case "completed":
		return core.BatchStatusCompleted
	case "failed":
		return core.BatchStatusFailed
	case "cancelled", "cancelling":
		return core.BatchStatusCancelled
	case "expired":
		return core.BatchStatusExpired
	default:
		return core.BatchStatusPending
	}
}
