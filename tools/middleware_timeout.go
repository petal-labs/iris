package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// WithTimeout creates middleware that enforces a timeout on tool execution.
func WithTimeout(d time.Duration) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()

			// Execute in goroutine to respect timeout.
			type result struct {
				value any
				err   error
			}
			ch := make(chan result, 1)

			go func() {
				v, err := next(ctx, args)
				ch <- result{v, err}
			}()

			select {
			case r := <-ch:
				return r.value, r.err
			case <-ctx.Done():
				return nil, fmt.Errorf("tool execution timeout after %v", d)
			}
		}
	}
}
