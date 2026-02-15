package tools

import (
	"context"
	"encoding/json"
)

// ToolCallFunc is the function signature for tool execution.
// Middleware wraps this function to add behavior.
type ToolCallFunc func(ctx context.Context, args json.RawMessage) (any, error)

// Middleware wraps a ToolCallFunc to add behavior before and/or after execution.
// Middleware functions receive the next handler in the chain and return a new handler.
type Middleware func(next ToolCallFunc) ToolCallFunc

// ToolContextSchemaMetadataKey is the metadata key used to store tool schemas.
const ToolContextSchemaMetadataKey = "schema"

// ToolContext provides metadata about the current tool call to middleware.
// It's stored in the context and accessible via ToolContextFromContext.
type ToolContext struct {
	// ToolName is the name of the tool being called.
	ToolName string

	// CallID is a unique identifier for this invocation (if available).
	CallID string

	// Iteration is the current workflow loop iteration (if provided by caller).
	Iteration int

	// Schema is the tool's JSON schema for this invocation.
	Schema json.RawMessage

	// Metadata allows middleware to share data with each other.
	Metadata map[string]any
}

// toolContextKey is the context key for ToolContext.
type toolContextKey struct{}

// ContextWithToolContext adds ToolContext to a context.
func ContextWithToolContext(ctx context.Context, tc *ToolContext) context.Context {
	return context.WithValue(ctx, toolContextKey{}, tc)
}

// ToolContextFromContext retrieves ToolContext from a context.
// Returns nil if not present.
func ToolContextFromContext(ctx context.Context) *ToolContext {
	tc, _ := ctx.Value(toolContextKey{}).(*ToolContext)
	return tc
}

// ToolSchemaFromContext retrieves a tool schema from context.
func ToolSchemaFromContext(ctx context.Context) (json.RawMessage, bool) {
	tc := ToolContextFromContext(ctx)
	if tc == nil {
		return nil, false
	}

	if len(tc.Schema) > 0 {
		return cloneRawMessage(tc.Schema), true
	}

	if tc.Metadata == nil {
		return nil, false
	}

	raw, ok := tc.Metadata[ToolContextSchemaMetadataKey]
	if !ok {
		return nil, false
	}

	switch value := raw.(type) {
	case json.RawMessage:
		if len(value) == 0 {
			return nil, false
		}
		return cloneRawMessage(value), true
	case []byte:
		if len(value) == 0 {
			return nil, false
		}
		return cloneRawMessage(json.RawMessage(value)), true
	case string:
		if value == "" {
			return nil, false
		}
		return json.RawMessage(value), true
	default:
		return nil, false
	}
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make(json.RawMessage, len(raw))
	copy(cloned, raw)
	return cloned
}

func setToolSchema(tc *ToolContext, schema json.RawMessage) {
	if tc.Metadata == nil {
		tc.Metadata = make(map[string]any)
	}

	if len(schema) == 0 {
		tc.Schema = nil
		delete(tc.Metadata, ToolContextSchemaMetadataKey)
		return
	}

	cloned := cloneRawMessage(schema)
	tc.Schema = cloned
	tc.Metadata[ToolContextSchemaMetadataKey] = cloned
}

// Chain combines multiple middleware into a single middleware.
// Middleware are executed in the order provided (first middleware is outermost).
func Chain(middlewares ...Middleware) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		// Apply in reverse order so first middleware is outermost.
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// ApplyMiddleware wraps a tool with middleware.
// Returns a new tool that executes middleware around the original.
func ApplyMiddleware(tool Tool, middlewares ...Middleware) Tool {
	if len(middlewares) == 0 {
		return tool
	}

	chain := Chain(middlewares...)
	wrapped := chain(tool.Call)

	return &wrappedTool{
		tool:    tool,
		wrapped: wrapped,
	}
}

// wrappedTool is a tool with middleware applied.
type wrappedTool struct {
	tool    Tool
	wrapped ToolCallFunc
}

func (w *wrappedTool) Name() string        { return w.tool.Name() }
func (w *wrappedTool) Description() string { return w.tool.Description() }
func (w *wrappedTool) Schema() ToolSchema  { return w.tool.Schema() }

func (w *wrappedTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Ensure ToolContext exists.
	tc := ToolContextFromContext(ctx)
	if tc == nil {
		tc = &ToolContext{
			ToolName: w.tool.Name(),
			Metadata: make(map[string]any),
		}
		ctx = ContextWithToolContext(ctx, tc)
	} else {
		if tc.ToolName == "" {
			tc.ToolName = w.tool.Name()
		}
		if tc.Metadata == nil {
			tc.Metadata = make(map[string]any)
		}
	}

	setToolSchema(tc, w.tool.Schema().JSONSchema)
	return w.wrapped(ctx, args)
}
