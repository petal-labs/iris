package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// mockTool is a test implementation of the Tool interface.
type mockTool struct {
	name        string
	description string
	schema      tools.ToolSchema
	callFn      func(ctx context.Context, args json.RawMessage) (any, error)
}

func (m *mockTool) Name() string             { return m.name }
func (m *mockTool) Description() string      { return m.description }
func (m *mockTool) Schema() tools.ToolSchema { return m.schema }
func (m *mockTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	return m.callFn(ctx, args)
}

func TestToolInterfaceCanBeImplemented(t *testing.T) {
	tool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		schema: tools.ToolSchema{
			JSONSchema: json.RawMessage(`{"type": "object"}`),
		},
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			return "result", nil
		},
	}

	// Verify interface is satisfied
	var _ tools.Tool = tool

	if tool.Name() != "test_tool" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "test_tool")
	}

	if tool.Description() != "A test tool" {
		t.Errorf("Description() = %q, want %q", tool.Description(), "A test tool")
	}

	schema := tool.Schema()
	if string(schema.JSONSchema) != `{"type": "object"}` {
		t.Errorf("Schema().JSONSchema = %q, want %q", string(schema.JSONSchema), `{"type": "object"}`)
	}

	result, err := tool.Call(context.Background(), nil)
	if err != nil {
		t.Errorf("Call() error = %v, want nil", err)
	}
	if result != "result" {
		t.Errorf("Call() = %v, want %q", result, "result")
	}
}

func TestToolSatisfiesCoreTool(t *testing.T) {
	tool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
	}

	// tools.Tool should satisfy core.Tool
	var _ core.Tool = tool
}

func TestToolSchemaJSONSerialization(t *testing.T) {
	schema := tools.ToolSchema{
		JSONSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var parsed tools.ToolSchema
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if string(parsed.JSONSchema) != string(schema.JSONSchema) {
		t.Errorf("Round-trip failed: got %q, want %q", string(parsed.JSONSchema), string(schema.JSONSchema))
	}
}
