package perplexity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
)

// toolCallAssembler accumulates streaming tool call fragments.
type toolCallAssembler struct {
	calls map[int]*assemblingToolCall
}

type assemblingToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func newToolCallAssembler() *toolCallAssembler {
	return &toolCallAssembler{
		calls: make(map[int]*assemblingToolCall),
	}
}

// addFragment processes a streaming tool call fragment.
func (a *toolCallAssembler) addFragment(tc perplexityStreamToolCall) {
	call, exists := a.calls[tc.Index]
	if !exists {
		call = &assemblingToolCall{}
		a.calls[tc.Index] = call
	}

	if tc.ID != "" {
		call.ID = tc.ID
	}
	if tc.Function.Name != "" {
		call.Name = tc.Function.Name
	}
	if tc.Function.Arguments != "" {
		call.Arguments.WriteString(tc.Function.Arguments)
	}
}

// finalize validates and returns the assembled tool calls.
func (a *toolCallAssembler) finalize() ([]core.ToolCall, error) {
	if len(a.calls) == 0 {
		return nil, nil
	}

	// Find max index to determine slice size
	maxIndex := 0
	for idx := range a.calls {
		if idx > maxIndex {
			maxIndex = idx
		}
	}

	result := make([]core.ToolCall, 0, len(a.calls))
	for i := 0; i <= maxIndex; i++ {
		call, exists := a.calls[i]
		if !exists {
			continue
		}

		args := call.Arguments.String()
		if !json.Valid([]byte(args)) {
			return nil, ErrToolArgsInvalidJSON
		}

		result = append(result, core.ToolCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: json.RawMessage(args),
		})
	}

	return result, nil
}

// doStreamChat performs a streaming chat completion request.
func (p *Perplexity) doStreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	// Build Perplexity request with stream=true
	pReq := buildRequest(req, true)

	// Marshal request body
	body, err := json.Marshal(pReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request
	url := p.config.BaseURL + chatCompletionsPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Set headers
	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	// Execute request
	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Extract request ID from response headers
	requestID := resp.Header.Get("x-request-id")

	// Check for error status
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	// Create channels
	chunkCh := make(chan core.ChatChunk, 100)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)

	// Start goroutine to process SSE stream
	go p.processSSEStream(ctx, resp.Body, chunkCh, errCh, finalCh)

	return &core.ChatStream{
		Ch:    chunkCh,
		Err:   errCh,
		Final: finalCh,
	}, nil
}

// processSSEStream reads the SSE stream and emits chunks.
func (p *Perplexity) processSSEStream(
	ctx context.Context,
	body io.ReadCloser,
	chunkCh chan<- core.ChatChunk,
	errCh chan<- error,
	finalCh chan<- *core.ChatResponse,
) {
	defer body.Close()
	defer close(chunkCh)
	defer close(errCh)
	defer close(finalCh)

	reader := bufio.NewReader(body)
	assembler := newToolCallAssembler()

	var responseID string
	var responseModel string
	var usage *perplexityUsage

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}

		// Read line
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			errCh <- newNetworkError(err)
			return
		}

		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Process data lines
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

		// Check for done signal
		if payload == "[DONE]" {
			break
		}

		// Parse chunk
		var chunk perplexityStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			errCh <- newDecodeError(err)
			return
		}

		// Capture metadata
		if chunk.ID != "" {
			responseID = chunk.ID
		}
		if chunk.Model != "" {
			responseModel = chunk.Model
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		// Process choices
		for _, choice := range chunk.Choices {
			// Emit content delta
			if choice.Delta.Content != "" {
				select {
				case chunkCh <- core.ChatChunk{Delta: choice.Delta.Content}:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}

			// Accumulate tool calls
			for _, tc := range choice.Delta.ToolCalls {
				assembler.addFragment(tc)
			}
		}
	}

	// Finalize tool calls
	toolCalls, err := assembler.finalize()
	if err != nil {
		errCh <- err
		return
	}

	// Build final response
	finalResp := &core.ChatResponse{
		ID:        responseID,
		Model:     core.ModelID(responseModel),
		ToolCalls: toolCalls,
	}

	if usage != nil {
		finalResp.Usage = core.TokenUsage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		}
	}

	finalCh <- finalResp
}
