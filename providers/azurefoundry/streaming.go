package azurefoundry

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/toolcalls"
)

// Streaming response types for Azure SSE protocol.
// Azure uses the same SSE format as OpenAI.

// azureStreamChunk represents a single SSE chunk from Azure.
type azureStreamChunk struct {
	ID                string               `json:"id"`
	Object            string               `json:"object"`
	Created           int64                `json:"created"`
	Model             string               `json:"model"`
	Choices           []azureStreamChoice  `json:"choices"`
	Usage             *azureUsage          `json:"usage,omitempty"`
	SystemFingerprint string               `json:"system_fingerprint,omitempty"`
}

// azureStreamChoice represents a choice in a streaming response.
type azureStreamChoice struct {
	Index                int                   `json:"index"`
	Delta                azureStreamDelta      `json:"delta"`
	FinishReason         *string               `json:"finish_reason,omitempty"`
	ContentFilterResults *azureContentFilters  `json:"content_filter_results,omitempty"`
}

// azureStreamDelta represents the incremental content in a streaming choice.
type azureStreamDelta struct {
	Role      string                 `json:"role,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolCalls []azureStreamToolCall  `json:"tool_calls,omitempty"`
	Refusal   string                 `json:"refusal,omitempty"`
}

// azureStreamToolCall represents a tool call fragment in streaming.
type azureStreamToolCall struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function azureStreamFunction    `json:"function,omitempty"`
}

// azureStreamFunction represents function details in a streaming tool call.
type azureStreamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// toolCallAssembler accumulates streaming tool call fragments.
type toolCallAssembler struct {
	asm *toolcalls.Assembler
}

// newToolCallAssembler creates a new tool call assembler.
func newToolCallAssembler() *toolCallAssembler {
	return &toolCallAssembler{
		asm: toolcalls.NewAssembler(toolcalls.Config{}),
	}
}

// addFragment processes a streaming tool call fragment.
func (a *toolCallAssembler) addFragment(tc azureStreamToolCall) {
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

// processSSEStream reads the SSE stream and emits chunks.
// This replaces the placeholder implementation in client_chat.go.
func (p *AzureFoundry) processSSEStream(
	ctx context.Context,
	resp *http.Response,
	model core.ModelID,
) (*core.ChatStream, error) {
	// Create channels
	chunkCh := make(chan core.ChatChunk, 100)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)

	// Start goroutine to process SSE stream
	go p.readSSEStream(ctx, resp.Body, model, chunkCh, errCh, finalCh)

	return &core.ChatStream{
		Ch:    chunkCh,
		Err:   errCh,
		Final: finalCh,
	}, nil
}

// readSSEStream reads and processes the SSE stream from Azure.
func (p *AzureFoundry) readSSEStream(
	ctx context.Context,
	body io.ReadCloser,
	requestModel core.ModelID,
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
	var usage *azureUsage
	var contentFiltered bool
	var contentBuilder strings.Builder

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
		var chunk azureStreamChunk
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
			// Check for content filtering
			if choice.ContentFilterResults != nil && choice.ContentFilterResults.IsFiltered() {
				contentFiltered = true
			}

			// Emit content delta
			if choice.Delta.Content != "" {
				contentBuilder.WriteString(choice.Delta.Content)
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

	// Check if content was filtered
	if contentFiltered {
		errCh <- ErrContentFiltered
		return
	}

	// Finalize tool calls
	toolCalls, err := assembler.finalize()
	if err != nil {
		errCh <- err
		return
	}

	// Use request model if response model not provided
	finalModel := responseModel
	if finalModel == "" {
		finalModel = string(requestModel)
	}

	// Build final response
	finalResp := &core.ChatResponse{
		ID:        responseID,
		Model:     core.ModelID(finalModel),
		Output:    contentBuilder.String(),
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
