package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// doStreamChat sends a streaming chat request to the Ollama API.
func (p *Ollama) doStreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	// Build request body
	ollamaReq := mapRequest(req, true)

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := p.config.BaseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	// Send request
	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &core.ProviderError{
			Provider: "ollama",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseErrorResponse(resp)
	}

	// Create channels
	chunkCh := make(chan core.ChatChunk, 100)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)

	// Start goroutine to read stream
	go p.processNDJSONStream(ctx, resp, chunkCh, errCh, finalCh)

	return &core.ChatStream{
		Ch:    chunkCh,
		Err:   errCh,
		Final: finalCh,
	}, nil
}

// processNDJSONStream reads the NDJSON stream and sends chunks to channels.
func (p *Ollama) processNDJSONStream(
	ctx context.Context,
	resp *http.Response,
	chunkCh chan<- core.ChatChunk,
	errCh chan<- error,
	finalCh chan<- *core.ChatResponse,
) {
	defer resp.Body.Close()
	defer close(chunkCh)
	defer close(errCh)
	defer close(finalCh)

	scanner := bufio.NewScanner(resp.Body)

	// Accumulate content and tool calls for final response
	var accumulatedContent string
	var accumulatedToolCalls []ollamaToolCall
	var accumulatedThinking string
	var finalResp *ollamaResponse

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk ollamaResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			errCh <- fmt.Errorf("failed to parse stream chunk: %w", err)
			return
		}

		// Check for inline error
		if chunk.Error != "" {
			errCh <- newStreamError(chunk.Error)
			return
		}

		// Accumulate content
		if chunk.Message.Content != "" {
			accumulatedContent += chunk.Message.Content
			select {
			case chunkCh <- core.ChatChunk{Delta: chunk.Message.Content}:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		// Accumulate thinking
		if chunk.Message.Thinking != "" {
			accumulatedThinking += chunk.Message.Thinking
		}

		// Accumulate tool calls
		if len(chunk.Message.ToolCalls) > 0 {
			accumulatedToolCalls = append(accumulatedToolCalls, chunk.Message.ToolCalls...)
		}

		// Handle final response
		if chunk.Done {
			finalResp = &chunk
			finalResp.Message.Content = accumulatedContent
			finalResp.Message.Thinking = accumulatedThinking
			finalResp.Message.ToolCalls = accumulatedToolCalls
			break
		}
	}

	if err := scanner.Err(); err != nil {
		errCh <- fmt.Errorf("stream read error: %w", err)
		return
	}

	// Send final response
	if finalResp != nil {
		finalCh <- mapResponse(finalResp)
	}
}
