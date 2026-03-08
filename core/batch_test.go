package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBatchInfoIsComplete(t *testing.T) {
	tests := []struct {
		status   BatchStatus
		expected bool
	}{
		{BatchStatusPending, false},
		{BatchStatusInProgress, false},
		{BatchStatusCompleted, true},
		{BatchStatusFailed, true},
		{BatchStatusCancelled, true},
		{BatchStatusExpired, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			info := &BatchInfo{Status: tc.status}
			if got := info.IsComplete(); got != tc.expected {
				t.Errorf("IsComplete() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestBatchResultIsSuccess(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := &BatchResult{
			CustomID: "test",
			Response: &ChatResponse{Output: "hello"},
		}
		if !r.IsSuccess() {
			t.Error("IsSuccess() should return true for successful result")
		}
	})

	t.Run("error", func(t *testing.T) {
		r := &BatchResult{
			CustomID: "test",
			Error:    &BatchError{Code: "rate_limit", Message: "too many requests"},
		}
		if r.IsSuccess() {
			t.Error("IsSuccess() should return false for error result")
		}
	})

	t.Run("nil response", func(t *testing.T) {
		r := &BatchResult{CustomID: "test"}
		if r.IsSuccess() {
			t.Error("IsSuccess() should return false when response is nil")
		}
	})
}

// mockBatchProvider implements BatchProvider for testing.
type mockBatchProvider struct {
	mu          sync.Mutex
	statuses    []BatchInfo
	statusIndex int
	createErr   error
	statusErr   error
	resultsErr  error
	cancelErr   error
	results     []BatchResult
}

func (m *mockBatchProvider) CreateBatch(_ context.Context, _ []BatchRequest) (BatchID, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "batch_123", nil
}

func (m *mockBatchProvider) GetBatchStatus(_ context.Context, _ BatchID) (*BatchInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.statusErr != nil {
		return nil, m.statusErr
	}

	if m.statusIndex < len(m.statuses) {
		info := m.statuses[m.statusIndex]
		m.statusIndex++
		return &info, nil
	}

	// If we've exhausted statuses, return the last one (for timeout tests)
	if len(m.statuses) > 0 {
		return &m.statuses[len(m.statuses)-1], nil
	}

	return &BatchInfo{Status: BatchStatusCompleted}, nil
}

func (m *mockBatchProvider) GetBatchResults(_ context.Context, _ BatchID) ([]BatchResult, error) {
	if m.resultsErr != nil {
		return nil, m.resultsErr
	}
	return m.results, nil
}

func (m *mockBatchProvider) CancelBatch(_ context.Context, _ BatchID) error {
	return m.cancelErr
}

func (m *mockBatchProvider) ListBatches(_ context.Context, _ int) ([]BatchInfo, error) {
	return nil, nil
}

func TestAsBatchProvider(t *testing.T) {
	t.Run("supports batch", func(t *testing.T) {
		mock := &fullBatchProvider{}
		bp, ok := AsBatchProvider(mock)
		if !ok {
			t.Error("AsBatchProvider should return true for BatchProvider")
		}
		if bp == nil {
			t.Error("AsBatchProvider should return non-nil BatchProvider")
		}
	})

	t.Run("does not support batch", func(t *testing.T) {
		// Use a type that implements Provider but not BatchProvider
		var p Provider = &nonBatchProvider{}
		bp, ok := AsBatchProvider(p)
		if ok {
			t.Error("AsBatchProvider should return false for non-BatchProvider")
		}
		if bp != nil {
			t.Error("AsBatchProvider should return nil for non-BatchProvider")
		}
	})
}

// fullBatchProvider implements both Provider and BatchProvider.
type fullBatchProvider struct {
	mockBatchProvider
}

func (p *fullBatchProvider) ID() string              { return "test" }
func (p *fullBatchProvider) Models() []ModelInfo     { return nil }
func (p *fullBatchProvider) Supports(_ Feature) bool { return true }
func (p *fullBatchProvider) Chat(_ context.Context, _ *ChatRequest) (*ChatResponse, error) {
	return nil, nil
}
func (p *fullBatchProvider) StreamChat(_ context.Context, _ *ChatRequest) (*ChatStream, error) {
	return nil, nil
}

// nonBatchProvider implements Provider but not BatchProvider.
type nonBatchProvider struct{}

func (p *nonBatchProvider) ID() string              { return "test" }
func (p *nonBatchProvider) Models() []ModelInfo     { return nil }
func (p *nonBatchProvider) Supports(_ Feature) bool { return false }
func (p *nonBatchProvider) Chat(_ context.Context, _ *ChatRequest) (*ChatResponse, error) {
	return nil, nil
}
func (p *nonBatchProvider) StreamChat(_ context.Context, _ *ChatRequest) (*ChatStream, error) {
	return nil, nil
}

func TestBatchWaiter(t *testing.T) {
	t.Run("waits for completion", func(t *testing.T) {
		mock := &mockBatchProvider{
			statuses: []BatchInfo{
				{ID: "batch_1", Status: BatchStatusPending},
				{ID: "batch_1", Status: BatchStatusInProgress},
				{ID: "batch_1", Status: BatchStatusCompleted},
			},
		}

		waiter := NewBatchWaiter(mock).
			WithPollInterval(1 * time.Millisecond).
			WithMaxWait(1 * time.Second)

		info, err := waiter.Wait(context.Background(), "batch_1")
		if err != nil {
			t.Fatalf("Wait() error: %v", err)
		}
		if info.Status != BatchStatusCompleted {
			t.Errorf("Status = %v, want %v", info.Status, BatchStatusCompleted)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		mock := &mockBatchProvider{
			statuses: []BatchInfo{
				{ID: "batch_1", Status: BatchStatusPending},
				{ID: "batch_1", Status: BatchStatusPending},
				{ID: "batch_1", Status: BatchStatusPending},
			},
		}

		waiter := NewBatchWaiter(mock).
			WithPollInterval(10 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()

		_, err := waiter.Wait(ctx, "batch_1")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Wait() error = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("returns timeout error", func(t *testing.T) {
		mock := &mockBatchProvider{
			statuses: []BatchInfo{
				{ID: "batch_1", Status: BatchStatusPending},
			},
		}

		waiter := NewBatchWaiter(mock).
			WithPollInterval(1 * time.Millisecond).
			WithMaxWait(5 * time.Millisecond)

		_, err := waiter.Wait(context.Background(), "batch_1")
		if !errors.Is(err, ErrBatchTimeout) {
			t.Errorf("Wait() error = %v, want ErrBatchTimeout", err)
		}
	})

	t.Run("returns status error", func(t *testing.T) {
		expectedErr := errors.New("status error")
		mock := &mockBatchProvider{
			statusErr: expectedErr,
		}

		waiter := NewBatchWaiter(mock).
			WithPollInterval(1 * time.Millisecond)

		_, err := waiter.Wait(context.Background(), "batch_1")
		if err != expectedErr {
			t.Errorf("Wait() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestBatchWaiterWaitAndCollect(t *testing.T) {
	t.Run("waits and collects results", func(t *testing.T) {
		mock := &mockBatchProvider{
			statuses: []BatchInfo{
				{ID: "batch_1", Status: BatchStatusPending},
				{ID: "batch_1", Status: BatchStatusCompleted},
			},
			results: []BatchResult{
				{CustomID: "req_1", Response: &ChatResponse{Output: "hello"}},
				{CustomID: "req_2", Response: &ChatResponse{Output: "world"}},
			},
		}

		waiter := NewBatchWaiter(mock).
			WithPollInterval(1 * time.Millisecond)

		results, err := waiter.WaitAndCollect(context.Background(), "batch_1")
		if err != nil {
			t.Fatalf("WaitAndCollect() error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("len(results) = %d, want 2", len(results))
		}
	})

	t.Run("returns wait error", func(t *testing.T) {
		expectedErr := errors.New("wait error")
		mock := &mockBatchProvider{
			statusErr: expectedErr,
		}

		waiter := NewBatchWaiter(mock).
			WithPollInterval(1 * time.Millisecond)

		_, err := waiter.WaitAndCollect(context.Background(), "batch_1")
		if err != expectedErr {
			t.Errorf("WaitAndCollect() error = %v, want %v", err, expectedErr)
		}
	})
}
