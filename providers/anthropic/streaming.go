package anthropic

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

// Streaming types for tool call assembly.

// toolCallAssembler accumulates streaming tool call fragments.
type toolCallAssembler struct {
	asm *toolcalls.Assembler
}

func newToolCallAssembler() *toolCallAssembler {
	return &toolCallAssembler{
		asm: toolcalls.NewAssembler(toolcalls.Config{
			EmptyArgumentsJSON: "{}",
		}),
	}
}

// startToolUse begins tracking a new tool use block.
func (a *toolCallAssembler) startToolUse(index int, id, name string) {
	a.asm.StartCall(index, id, name)
}

// addFragment processes a streaming tool call fragment.
func (a *toolCallAssembler) addFragment(index int, partialJSON string) {
	a.asm.AddArguments(index, partialJSON)
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

// doStreamChat performs a streaming chat request.
func (p *Anthropic) doStreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	// Build Anthropic request with stream=true
	antReq := buildRequest(req, true)

	// Marshal request body
	body, err := json.Marshal(antReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request
	url := p.config.BaseURL + messagesPath
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
	requestID := resp.Header.Get("request-id")

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
func (p *Anthropic) processSSEStream(
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
	var usage anthropicUsage
	var currentBlockIndex int

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

		// Process event lines
		if strings.HasPrefix(line, "event:") {
			// Event type line - continue to read data
			continue
		}

		// Process data lines
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

		// Parse event
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			errCh <- newDecodeError(err)
			return
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				responseID = event.Message.ID
				responseModel = event.Message.Model
				usage = event.Message.Usage
			}

		case "content_block_start":
			currentBlockIndex = event.Index
			if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
				assembler.startToolUse(event.Index, event.ContentBlock.ID, event.ContentBlock.Name)
			}

		case "content_block_delta":
			if event.Delta != nil {
				switch event.Delta.Type {
				case "text_delta":
					// Emit text delta
					if event.Delta.Text != "" {
						select {
						case chunkCh <- core.ChatChunk{Delta: event.Delta.Text}:
						case <-ctx.Done():
							errCh <- ctx.Err()
							return
						}
					}
				case "input_json_delta":
					// Accumulate tool input JSON
					if event.Delta.PartialJSON != "" {
						assembler.addFragment(currentBlockIndex, event.Delta.PartialJSON)
					}
				}
			}

		case "content_block_stop":
			// Block finished - nothing special to do

		case "message_delta":
			// Update usage from final delta
			if event.Usage != nil {
				usage.OutputTokens = event.Usage.OutputTokens
			}

		case "message_stop":
			// Stream finished - break the loop
			goto done

		case "error":
			if event.Error != nil {
				errCh <- &core.ProviderError{
					Provider: "anthropic",
					Code:     event.Error.Type,
					Message:  event.Error.Message,
					Err:      core.ErrServer,
				}
				return
			}
		}
	}

done:
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
		Usage: core.TokenUsage{
			PromptTokens:     usage.InputTokens,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      usage.InputTokens + usage.OutputTokens,
		},
	}

	finalCh <- finalResp
}
