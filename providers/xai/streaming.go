package xai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/toolcalls"
)

// toolCallAssembler accumulates streaming tool call fragments.
type toolCallAssembler struct {
	asm *toolcalls.Assembler
}

func newToolCallAssembler() *toolCallAssembler {
	return &toolCallAssembler{
		asm: toolcalls.NewAssembler(toolcalls.Config{}),
	}
}

// addFragment processes a streaming tool call fragment.
func (a *toolCallAssembler) addFragment(tc xaiStreamToolCall) {
	a.asm.AddFragment(toolcalls.Fragment{
		Index:     tc.Index,
		ID:        tc.ID,
		Name:      tc.Function.Name,
		Arguments: tc.Function.Arguments,
	})
}

// finalize validates and returns the assembled tool calls.
func (a *toolCallAssembler) finalize() ([]core.ToolCall, error) {
	calls, err := a.asm.Finalize()
	if err != nil {
		if errors.Is(err, toolcalls.ErrInvalidJSON) {
			return nil, ErrToolArgsInvalidJSON
		}
		return nil, err
	}
	return calls, nil
}

// doStreamChat performs a streaming chat completion request.
func (p *Xai) doStreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	// Build xAI request with stream=true
	xaiReq := buildRequest(req, true)

	// Marshal request body
	body, err := json.Marshal(xaiReq)
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
func (p *Xai) processSSEStream(
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
	var usage *xaiUsage

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
		var chunk xaiStreamChunk
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
