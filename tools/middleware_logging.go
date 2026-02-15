package tools

import (
	"context"
	"encoding/json"
	"time"
)

// Logger is the interface for logging middleware.
type Logger interface {
	Printf(format string, v ...any)
}

// WithLogging creates middleware that logs tool calls.
func WithLogging(logger Logger) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			toolName := "unknown"
			if tc != nil {
				toolName = tc.ToolName
			}

			logger.Printf("tool call start: %s", toolName)
			start := time.Now()

			result, err := next(ctx, args)

			duration := time.Since(start)
			if err != nil {
				logger.Printf("tool call error: %s, duration=%v, error=%v", toolName, duration, err)
			} else {
				logger.Printf("tool call success: %s, duration=%v", toolName, duration)
			}

			return result, err
		}
	}
}

// WithDetailedLogging creates middleware that logs tool calls with arguments.
// WARNING: May log sensitive data. Use only in development.
func WithDetailedLogging(logger Logger) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			toolName := "unknown"
			if tc != nil {
				toolName = tc.ToolName
			}

			logger.Printf("tool call: %s, args=%s", toolName, string(args))
			start := time.Now()

			result, err := next(ctx, args)

			duration := time.Since(start)
			if err != nil {
				logger.Printf("tool error: %s, duration=%v, error=%v", toolName, duration, err)
			} else {
				resultJSON, _ := json.Marshal(result)
				logger.Printf("tool result: %s, duration=%v, result=%s", toolName, duration, string(resultJSON))
			}

			return result, err
		}
	}
}
