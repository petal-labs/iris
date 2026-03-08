package testing

import (
	"context"
	"sync"
	"time"

	"github.com/petal-labs/iris/core"
)

// RecordedCall contains details about a call made to the underlying provider.
type RecordedCall struct {
	Method    string             // "Chat" or "StreamChat"
	Request   *core.ChatRequest  // The request that was made
	Response  *core.ChatResponse // The response received (nil for streaming until drained)
	Error     error              // Any error that occurred
	StartTime time.Time          // When the call started
	EndTime   time.Time          // When the call completed
	Duration  time.Duration      // How long the call took
}

// RecordingProvider wraps a real provider and records all calls for inspection.
// This is useful for debugging, testing, and creating replay fixtures.
// RecordingProvider is safe for concurrent use.
type RecordingProvider struct {
	underlying core.Provider
	mu         sync.Mutex
	recordings []RecordedCall
}

// NewRecordingProvider creates a recording wrapper around an existing provider.
func NewRecordingProvider(underlying core.Provider) *RecordingProvider {
	return &RecordingProvider{
		underlying: underlying,
	}
}

// ID returns the underlying provider's ID.
func (r *RecordingProvider) ID() string {
	return r.underlying.ID()
}

// Models returns the underlying provider's models.
func (r *RecordingProvider) Models() []core.ModelInfo {
	return r.underlying.Models()
}

// Supports returns whether the underlying provider supports a feature.
func (r *RecordingProvider) Supports(f core.Feature) bool {
	return r.underlying.Supports(f)
}

// Chat performs a chat request and records the call.
func (r *RecordingProvider) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	start := time.Now()
	resp, err := r.underlying.Chat(ctx, req)
	end := time.Now()

	r.mu.Lock()
	r.recordings = append(r.recordings, RecordedCall{
		Method:    "Chat",
		Request:   cloneRequest(req),
		Response:  resp,
		Error:     err,
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
	})
	r.mu.Unlock()

	return resp, err
}

// StreamChat performs a streaming chat request and records the call.
// Note: The recorded response will be nil; use the stream to get the response.
func (r *RecordingProvider) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	start := time.Now()
	stream, err := r.underlying.StreamChat(ctx, req)
	end := time.Now()

	r.mu.Lock()
	r.recordings = append(r.recordings, RecordedCall{
		Method:    "StreamChat",
		Request:   cloneRequest(req),
		Response:  nil, // Streaming responses aren't captured here
		Error:     err,
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
	})
	r.mu.Unlock()

	return stream, err
}

// Recordings returns all recorded calls.
// The returned slice is a copy and safe to modify.
func (r *RecordingProvider) Recordings() []RecordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]RecordedCall, len(r.recordings))
	copy(result, r.recordings)
	return result
}

// RecordingCount returns the number of recorded calls.
func (r *RecordingProvider) RecordingCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.recordings)
}

// LastRecording returns the most recent recorded call, or nil if none.
func (r *RecordingProvider) LastRecording() *RecordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.recordings) == 0 {
		return nil
	}
	recording := r.recordings[len(r.recordings)-1]
	return &recording
}

// Clear removes all recorded calls.
func (r *RecordingProvider) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recordings = nil
}

// Underlying returns the wrapped provider.
func (r *RecordingProvider) Underlying() core.Provider {
	return r.underlying
}

// Compile-time verification that RecordingProvider implements Provider.
var _ core.Provider = (*RecordingProvider)(nil)
