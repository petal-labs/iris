package testing

import (
	"context"
	"strings"
	"sync"

	"github.com/petal-labs/iris/core"
)

// MockCall records a call made to the mock provider.
type MockCall struct {
	Method  string            // "Chat" or "StreamChat"
	Request *core.ChatRequest // The request that was made
}

// MockStreamConfig configures a streaming response.
type MockStreamConfig struct {
	Chunks   []string            // Text chunks to emit
	Final    *core.ChatResponse  // Final response (optional, auto-generated if nil)
	Error    error               // Error to emit (if set, sent after chunks)
}

// MockProvider is a test double for core.Provider.
// It allows you to define canned responses, queue errors, and verify calls.
// MockProvider is safe for concurrent use.
type MockProvider struct {
	mu sync.Mutex

	id       string
	models   []core.ModelInfo
	features map[core.Feature]bool

	// Response queue (responses are consumed in order)
	responses []core.ChatResponse
	errors    []error

	// Streaming response queue
	streamConfigs []MockStreamConfig

	// Call index for non-streaming responses
	responseIndex int

	// Stream index for streaming responses
	streamIndex int

	// Recorded calls
	calls []MockCall

	// Default response when queue is exhausted
	defaultResponse *core.ChatResponse
}

// NewMockProvider creates a mock provider with optional canned responses.
// Responses are returned in order; after exhaustion, a default response is used.
func NewMockProvider(responses ...core.ChatResponse) *MockProvider {
	return &MockProvider{
		id:        "mock",
		features:  make(map[core.Feature]bool),
		responses: responses,
		models: []core.ModelInfo{
			{
				ID:          "mock-model",
				DisplayName: "Mock Model",
				Capabilities: []core.Feature{
					core.FeatureChat,
					core.FeatureChatStreaming,
					core.FeatureToolCalling,
				},
			},
		},
		defaultResponse: &core.ChatResponse{
			ID:     "mock-response",
			Model:  "mock-model",
			Output: "mock response",
		},
	}
}

// WithID sets the provider ID.
func (m *MockProvider) WithID(id string) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.id = id
	return m
}

// WithModels sets the available models.
func (m *MockProvider) WithModels(models ...core.ModelInfo) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.models = models
	return m
}

// WithFeatures sets the supported features.
func (m *MockProvider) WithFeatures(features ...core.Feature) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, f := range features {
		m.features[f] = true
	}
	return m
}

// WithResponse adds a response to the queue.
func (m *MockProvider) WithResponse(resp core.ChatResponse) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, resp)
	return m
}

// WithResponses adds multiple responses to the queue.
func (m *MockProvider) WithResponses(responses ...core.ChatResponse) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, responses...)
	return m
}

// WithError queues an error to be returned by the next Chat call.
// Errors are consumed before responses.
func (m *MockProvider) WithError(err error) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, err)
	return m
}

// WithErrors queues multiple errors.
func (m *MockProvider) WithErrors(errs ...error) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, errs...)
	return m
}

// WithStreamingResponse adds a streaming response configuration.
func (m *MockProvider) WithStreamingResponse(chunks []string, final *core.ChatResponse) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamConfigs = append(m.streamConfigs, MockStreamConfig{
		Chunks: chunks,
		Final:  final,
	})
	return m
}

// WithStreamingError adds a streaming response that emits an error after chunks.
func (m *MockProvider) WithStreamingError(chunks []string, err error) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamConfigs = append(m.streamConfigs, MockStreamConfig{
		Chunks: chunks,
		Error:  err,
	})
	return m
}

// WithDefaultResponse sets the default response used when the queue is exhausted.
func (m *MockProvider) WithDefaultResponse(resp core.ChatResponse) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultResponse = &resp
	return m
}

// ID returns the provider identifier.
func (m *MockProvider) ID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.id
}

// Models returns the list of available models.
func (m *MockProvider) Models() []core.ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]core.ModelInfo, len(m.models))
	copy(result, m.models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (m *MockProvider) Supports(f core.Feature) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.features[f]
}

// Chat performs a non-streaming chat request.
func (m *MockProvider) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.calls = append(m.calls, MockCall{
		Method:  "Chat",
		Request: cloneRequest(req),
	})

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Return queued error if available
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return nil, err
	}

	// Return queued response if available
	if m.responseIndex < len(m.responses) {
		resp := m.responses[m.responseIndex]
		m.responseIndex++
		return &resp, nil
	}

	// Return default response
	if m.defaultResponse != nil {
		return m.defaultResponse, nil
	}

	return &core.ChatResponse{
		ID:     "mock-default",
		Model:  req.Model,
		Output: "mock response",
	}, nil
}

// StreamChat performs a streaming chat request.
func (m *MockProvider) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	m.mu.Lock()

	// Record the call
	m.calls = append(m.calls, MockCall{
		Method:  "StreamChat",
		Request: cloneRequest(req),
	})

	// Check for context cancellation
	if ctx.Err() != nil {
		m.mu.Unlock()
		return nil, ctx.Err()
	}

	// Return queued error if available (immediate error, not streaming error)
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		m.mu.Unlock()
		return nil, err
	}

	// Get stream config
	var config MockStreamConfig
	if m.streamIndex < len(m.streamConfigs) {
		config = m.streamConfigs[m.streamIndex]
		m.streamIndex++
	} else {
		// Default: use non-streaming response as single chunk
		var output string
		if m.responseIndex < len(m.responses) {
			output = m.responses[m.responseIndex].Output
			m.responseIndex++
		} else if m.defaultResponse != nil {
			output = m.defaultResponse.Output
		} else {
			output = "mock response"
		}
		config = MockStreamConfig{
			Chunks: []string{output},
		}
	}

	m.mu.Unlock()

	// Create channels
	chunkCh := make(chan core.ChatChunk)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)

	// Start goroutine to emit chunks
	go func() {
		defer close(chunkCh)
		defer close(errCh)
		defer close(finalCh)

		// Emit chunks
		for _, chunk := range config.Chunks {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case chunkCh <- core.ChatChunk{Delta: chunk}:
			}
		}

		// Emit error if configured
		if config.Error != nil {
			errCh <- config.Error
			return
		}

		// Emit final response
		final := config.Final
		if final == nil {
			// Build final from chunks
			var sb strings.Builder
			for _, c := range config.Chunks {
				sb.WriteString(c)
			}
			final = &core.ChatResponse{
				ID:     "mock-stream-response",
				Model:  req.Model,
				Output: sb.String(),
			}
		}
		finalCh <- final
	}()

	return &core.ChatStream{
		Ch:    chunkCh,
		Err:   errCh,
		Final: finalCh,
	}, nil
}

// Calls returns all recorded calls.
// The returned slice is a copy and safe to modify.
func (m *MockProvider) Calls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MockCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// CallCount returns the number of calls made.
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// LastCall returns the most recent call, or nil if no calls have been made.
func (m *MockProvider) LastCall() *MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return nil
	}
	call := m.calls[len(m.calls)-1]
	return &call
}

// Reset clears all recorded calls and resets response indices.
// Does not clear the queued responses or errors.
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.responseIndex = 0
	m.streamIndex = 0
}

// ResetAll clears everything including queued responses and errors.
func (m *MockProvider) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.responses = nil
	m.errors = nil
	m.streamConfigs = nil
	m.responseIndex = 0
	m.streamIndex = 0
}

// cloneRequest creates a shallow copy of a ChatRequest.
func cloneRequest(req *core.ChatRequest) *core.ChatRequest {
	if req == nil {
		return nil
	}
	clone := *req
	if len(req.Messages) > 0 {
		clone.Messages = make([]core.Message, len(req.Messages))
		copy(clone.Messages, req.Messages)
	}
	if len(req.Tools) > 0 {
		clone.Tools = make([]core.Tool, len(req.Tools))
		copy(clone.Tools, req.Tools)
	}
	return &clone
}

// Compile-time verification that MockProvider implements Provider.
var _ core.Provider = (*MockProvider)(nil)
