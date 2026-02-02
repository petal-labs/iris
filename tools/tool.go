package tools

import (
	"context"
	"encoding/json"
)

// Tool defines the interface for AI-callable tools.
// Tools provide a schema for argument validation and a Call method for execution.
//
// Any type implementing Tool also satisfies core.Tool (which requires only
// Name and Description), allowing tools to be used with ChatRequest.Tools.
type Tool interface {
	// Name returns the unique identifier for this tool.
	Name() string

	// Description returns a human-readable description of what this tool does.
	// This is provided to the AI model to help it decide when to use the tool.
	Description() string

	// Schema returns the JSON Schema that describes the tool's parameters.
	Schema() ToolSchema

	// Call executes the tool with the given arguments.
	// The args parameter contains the raw JSON arguments from the model.
	// Returns the tool's result or an error if execution fails.
	Call(ctx context.Context, args json.RawMessage) (any, error)
}

// ToolSchema describes the parameters a tool accepts.
// JSONSchema must be a valid JSON Schema object.
type ToolSchema struct {
	// JSONSchema is a valid JSON Schema object describing the tool's parameters.
	// Example: {"type": "object", "properties": {"location": {"type": "string"}}}
	JSONSchema json.RawMessage `json:"json_schema"`
}
