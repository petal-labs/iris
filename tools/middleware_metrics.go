package tools

import (
	"context"
	"encoding/json"
	"time"
)

// MetricsCollector receives tool execution metrics.
type MetricsCollector interface {
	// RecordCall records a tool call with its outcome.
	RecordCall(toolName string, duration time.Duration, err error)
}

// WithMetrics creates middleware that records tool execution metrics.
func WithMetrics(collector MetricsCollector) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			toolName := "unknown"
			if tc != nil {
				toolName = tc.ToolName
			}

			start := time.Now()
			result, err := next(ctx, args)
			duration := time.Since(start)

			collector.RecordCall(toolName, duration, err)
			return result, err
		}
	}
}
