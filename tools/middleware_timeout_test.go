package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestWithTimeoutErrorTypes(t *testing.T) {
	// Create a tool that blocks indefinitely
	blockedTool := func(ctx context.Context, args json.RawMessage) (any, error) {
		// Block forever (until context is cancelled)
		<-ctx.Done()
		return nil, ctx.Err()
	}

	// Wrap with a short timeout
	wrapped := ApplyMiddleware(
		&mockTool{
			name:   "blocked_tool",
			callFn: blockedTool,
		},
		WithTimeout(50*time.Millisecond),
	)

	// Execute and expect a timeout error
	_, err := wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// The error should satisfy errors.Is for both ErrTimeout and DeadlineExceeded
	if !errors.Is(err, core.ErrTimeout) {
		t.Errorf("errors.Is(err, core.ErrTimeout) = false, want true. err = %v", err)
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, want true. err = %v", err)
	}
}
