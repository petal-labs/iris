package tools

import (
	"context"
	"encoding/json"

	"github.com/petal-labs/iris/core"
)

// Tool defines the interface for executable AI-callable tools.
// Tools provide a schema for argument validation and a Call method for execution.
//
// Tool embeds core.Tool, so any type implementing Tool can be passed
// directly to ChatBuilder.Tools (or ChatRequest.Tools) and providers will
// transmit its schema.
type Tool interface {
	core.Tool

	// Call executes the tool with the given arguments.
	// The args parameter contains the raw JSON arguments from the model.
	// Returns the tool's result or an error if execution fails.
	Call(ctx context.Context, args json.RawMessage) (any, error)
}

// ToolSchema describes the parameters a tool accepts.
// It is an alias of core.ToolSchema; the schema contract is owned by core
// so providers can consume it without importing this package.
type ToolSchema = core.ToolSchema

// Ensure the executable interface satisfies the request-carrier interface.
var _ core.Tool = (Tool)(nil)
