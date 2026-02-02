package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
)

// doStreamChat performs a streaming chat request.
func (p *Gemini) doStreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	// Build Gemini request
	gemReq := buildRequest(req)

	// Marshal request body
	body, err := json.Marshal(gemReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request with streaming endpoint
	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", p.config.BaseURL, req.Model)
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

	// Check for error status
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, normalizeError(resp.StatusCode, respBody)
	}

	// Create channels
	chunkCh := make(chan core.ChatChunk, 100)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)

	// Start goroutine to process SSE stream
	go p.processSSEStream(ctx, resp.Body, chunkCh, errCh, finalCh, string(req.Model))

	return &core.ChatStream{
		Ch:    chunkCh,
		Err:   errCh,
		Final: finalCh,
	}, nil
}

// processSSEStream reads the SSE stream and emits chunks.
func (p *Gemini) processSSEStream(
	ctx context.Context,
	body io.ReadCloser,
	chunkCh chan<- core.ChatChunk,
	errCh chan<- error,
	finalCh chan<- *core.ChatResponse,
	model string,
) {
	defer body.Close()
	defer close(chunkCh)
	defer close(errCh)
	defer close(finalCh)

	reader := bufio.NewReader(body)

	var accumulatedText strings.Builder
	var toolCalls []core.ToolCall
	var thoughtParts []string
	var usage *geminiUsage
	toolCallIndex := 0

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

		// Skip empty lines
		if line == "" {
			continue
		}

		// Process data lines
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

		// Skip empty data
		if payload == "" {
			continue
		}

		// Parse event
		var event geminiResponse
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			errCh <- newDecodeError(err)
			return
		}

		// Update usage if present
		if event.UsageMetadata != nil {
			usage = event.UsageMetadata
		}

		// Process candidates
		if len(event.Candidates) == 0 {
			continue
		}

		candidate := event.Candidates[0]

		for _, part := range candidate.Content.Parts {
			// Check if this is a thought part
			if part.Thought != nil && *part.Thought {
				if part.Text != "" {
					thoughtParts = append(thoughtParts, part.Text)
				}
				continue
			}

			// Emit text delta
			if part.Text != "" {
				accumulatedText.WriteString(part.Text)
				select {
				case chunkCh <- core.ChatChunk{Delta: part.Text}:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}

			// Accumulate tool calls
			if part.FunctionCall != nil {
				toolCalls = append(toolCalls, core.ToolCall{
					ID:        fmt.Sprintf("call_%d", toolCallIndex),
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				})
				toolCallIndex++
			}
		}
	}

	// Build final response
	finalResp := &core.ChatResponse{
		Model:     core.ModelID(model),
		Output:    accumulatedText.String(),
		ToolCalls: toolCalls,
	}

	if usage != nil {
		finalResp.Usage = core.TokenUsage{
			PromptTokens:     usage.PromptTokenCount,
			CompletionTokens: usage.CandidatesTokenCount,
			TotalTokens:      usage.PromptTokenCount + usage.CandidatesTokenCount,
		}
	}

	// Add reasoning output if thoughts were present
	if len(thoughtParts) > 0 {
		finalResp.Reasoning = &core.ReasoningOutput{
			Summary: thoughtParts,
		}
	}

	finalCh <- finalResp
}
