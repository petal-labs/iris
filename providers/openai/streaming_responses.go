package openai

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

// doResponsesStreamChat performs a streaming request to the Responses API.
func (p *OpenAI) doResponsesStreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	// Build Responses API request with stream=true
	respReq := buildResponsesRequest(req, true)

	// Marshal request body
	body, err := json.Marshal(respReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request
	url := p.config.BaseURL + responsesPath
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
	go p.processResponsesStream(ctx, resp.Body, chunkCh, errCh, finalCh)

	return &core.ChatStream{
		Ch:    chunkCh,
		Err:   errCh,
		Final: finalCh,
	}, nil
}

// responsesStreamState holds state during streaming.
type responsesStreamState struct {
	responseID    string
	responseModel string
	status        string
	usage         *responsesUsage
	toolCalls     *toolcalls.Assembler
	toolCallDelta map[int]bool // index -> whether argument deltas were seen
	reasoning     []string     // reasoning summaries
}

func newResponsesStreamState() *responsesStreamState {
	return &responsesStreamState{
		toolCalls:     toolcalls.NewAssembler(toolcalls.Config{EmptyArgumentsJSON: "{}"}),
		toolCallDelta: make(map[int]bool),
	}
}

// processResponsesStream reads the SSE stream from the Responses API and emits chunks.
func (p *OpenAI) processResponsesStream(
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
	state := newResponsesStreamState()

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

		// Process event lines
		if strings.HasPrefix(line, "event:") {
			// Event type - we handle based on the data that follows
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

		// Parse event
		var event responsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			errCh <- newDecodeError(err)
			return
		}

		// Process event based on type
		if err := p.handleResponsesStreamEvent(ctx, &event, state, chunkCh); err != nil {
			errCh <- err
			return
		}
	}

	// Build final response
	finalResp := &core.ChatResponse{
		ID:     state.responseID,
		Model:  core.ModelID(state.responseModel),
		Status: state.status,
	}

	if state.usage != nil {
		finalResp.Usage = core.TokenUsage{
			PromptTokens:     state.usage.InputTokens,
			CompletionTokens: state.usage.OutputTokens,
			TotalTokens:      state.usage.TotalTokens,
		}
	}

	// Finalize tool calls
	toolCalls, err := state.toolCalls.Finalize()
	if err != nil {
		if errors.Is(err, toolcalls.ErrInvalidJSON) {
			errCh <- ErrToolArgsInvalidJSON
			return
		}
		errCh <- err
		return
	}
	if len(toolCalls) > 0 {
		finalResp.ToolCalls = toolCalls
	}

	// Set reasoning if any
	if len(state.reasoning) > 0 {
		finalResp.Reasoning = &core.ReasoningOutput{
			Summary: state.reasoning,
		}
	}

	finalCh <- finalResp
}

// handleResponsesStreamEvent processes a single streaming event.
func (p *OpenAI) handleResponsesStreamEvent(
	ctx context.Context,
	event *responsesStreamEvent,
	state *responsesStreamState,
	chunkCh chan<- core.ChatChunk,
) error {
	switch event.Type {
	case "response.created", "response.in_progress":
		// Parse response data to get ID and model
		if len(event.Response) > 0 {
			var resp responsesResponse
			if err := json.Unmarshal(event.Response, &resp); err == nil {
				state.responseID = resp.ID
				state.responseModel = resp.Model
				state.status = resp.Status
			}
		}

	case "response.completed":
		// Final response with usage
		if len(event.Response) > 0 {
			var resp responsesResponse
			if err := json.Unmarshal(event.Response, &resp); err == nil {
				state.responseID = resp.ID
				state.responseModel = resp.Model
				state.status = resp.Status
				state.usage = resp.Usage
			}
		}

	case "response.output_item.added":
		// New output item - could be reasoning, message, or function_call
		// We'll handle the content in the delta events

	case "response.output_text.delta":
		// Text content delta
		if len(event.Delta) > 0 {
			var delta responsesContentDelta
			if err := json.Unmarshal(event.Delta, &delta); err == nil && delta.Text != "" {
				select {
				case chunkCh <- core.ChatChunk{Delta: delta.Text}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

	case "response.content_part.delta":
		// Content part delta (alternative format)
		if len(event.Delta) > 0 {
			var delta responsesContentDelta
			if err := json.Unmarshal(event.Delta, &delta); err == nil && delta.Text != "" {
				select {
				case chunkCh <- core.ChatChunk{Delta: delta.Text}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

	case "response.function_call_arguments.delta":
		// Function call arguments delta
		if len(event.Delta) > 0 {
			var delta responsesFunctionCallDelta
			if err := json.Unmarshal(event.Delta, &delta); err == nil {
				idx := event.OutputIndex
				state.toolCalls.AddFragment(toolcalls.Fragment{
					Index:     idx,
					Arguments: delta.Arguments,
				})
				state.toolCallDelta[idx] = true
			}
		}

	case "response.output_item.done":
		// Output item completed - extract final info
		if len(event.Item) > 0 {
			var item responsesOutput
			if err := json.Unmarshal(event.Item, &item); err == nil {
				switch item.Type {
				case "function_call":
					idx := event.OutputIndex
					fragment := toolcalls.Fragment{
						Index: idx,
						ID:    item.CallID,
						Name:  item.Name,
					}
					// Avoid duplicating arguments when delta events already carried them.
					if item.Arguments != "" && !state.toolCallDelta[idx] {
						fragment.Arguments = item.Arguments
					}
					state.toolCalls.AddFragment(fragment)

				case "reasoning":
					// Extract reasoning summary
					for _, summary := range item.Summary {
						if summary.Text != "" {
							state.reasoning = append(state.reasoning, summary.Text)
						}
					}
				}
			}
		}
	}

	return nil
}
