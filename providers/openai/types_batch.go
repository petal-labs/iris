package openai

import "encoding/json"

// OpenAI Batch API types
// Reference: https://platform.openai.com/docs/api-reference/batch

// batchEndpoint is the API endpoint for batch operations.
const batchEndpoint = "/v1/chat/completions"

// openAIBatch represents an OpenAI batch object.
type openAIBatch struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Endpoint string `json:"endpoint"`
	Errors   *struct {
		Object string           `json:"object"`
		Data   []openAIBatchErr `json:"data"`
	} `json:"errors,omitempty"`
	InputFileID      string              `json:"input_file_id"`
	CompletionWindow string              `json:"completion_window"`
	Status           string              `json:"status"`
	OutputFileID     string              `json:"output_file_id,omitempty"`
	ErrorFileID      string              `json:"error_file_id,omitempty"`
	CreatedAt        int64               `json:"created_at"`
	InProgressAt     *int64              `json:"in_progress_at,omitempty"`
	ExpiresAt        *int64              `json:"expires_at,omitempty"`
	FinalizingAt     *int64              `json:"finalizing_at,omitempty"`
	CompletedAt      *int64              `json:"completed_at,omitempty"`
	FailedAt         *int64              `json:"failed_at,omitempty"`
	ExpiredAt        *int64              `json:"expired_at,omitempty"`
	CancellingAt     *int64              `json:"cancelling_at,omitempty"`
	CancelledAt      *int64              `json:"cancelled_at,omitempty"`
	RequestCounts    openAIBatchCounts   `json:"request_counts"`
	Metadata         map[string]string   `json:"metadata,omitempty"`
}

// openAIBatchErr represents an error within a batch.
type openAIBatchErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
	Line    *int   `json:"line,omitempty"`
}

// openAIBatchCounts tracks request processing counts.
type openAIBatchCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// openAIBatchCreateRequest is the request body for creating a batch.
type openAIBatchCreateRequest struct {
	InputFileID      string            `json:"input_file_id"`
	Endpoint         string            `json:"endpoint"`
	CompletionWindow string            `json:"completion_window"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// openAIBatchListResponse is the response from listing batches.
type openAIBatchListResponse struct {
	Object  string        `json:"object"`
	Data    []openAIBatch `json:"data"`
	HasMore bool          `json:"has_more"`
	FirstID string        `json:"first_id,omitempty"`
	LastID  string        `json:"last_id,omitempty"`
}

// openAIBatchRequestLine represents a single request in a batch JSONL file.
// This is the format required by OpenAI for batch input files.
type openAIBatchRequestLine struct {
	CustomID string               `json:"custom_id"`
	Method   string               `json:"method"`
	URL      string               `json:"url"`
	Body     openAIBatchReqBody   `json:"body"`
}

// openAIBatchReqBody is the body of a batch request line.
type openAIBatchReqBody struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature *float32        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
}

// openAIBatchResponseLine represents a single response in a batch output JSONL file.
type openAIBatchResponseLine struct {
	ID       string                    `json:"id"`
	CustomID string                    `json:"custom_id"`
	Response *openAIBatchResponseBody  `json:"response,omitempty"`
	Error    *openAIBatchResponseError `json:"error,omitempty"`
}

// openAIBatchResponseBody contains the response data.
type openAIBatchResponseBody struct {
	StatusCode int             `json:"status_code"`
	RequestID  string          `json:"request_id"`
	Body       json.RawMessage `json:"body"`
}

// openAIBatchResponseError contains error details.
type openAIBatchResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
