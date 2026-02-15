package tools

import (
	"context"
	"encoding/json"
)

// ForTools applies middleware only to tools with the specified names.
func ForTools(toolNames []string, middleware Middleware) Middleware {
	nameSet := make(map[string]bool)
	for _, name := range toolNames {
		nameSet[name] = true
	}

	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			if tc != nil && nameSet[tc.ToolName] {
				return middleware(next)(ctx, args)
			}
			return next(ctx, args)
		}
	}
}

// ExceptTools applies middleware to all tools except those with the specified names.
func ExceptTools(toolNames []string, middleware Middleware) Middleware {
	nameSet := make(map[string]bool)
	for _, name := range toolNames {
		nameSet[name] = true
	}

	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			if tc == nil || !nameSet[tc.ToolName] {
				return middleware(next)(ctx, args)
			}
			return next(ctx, args)
		}
	}
}
