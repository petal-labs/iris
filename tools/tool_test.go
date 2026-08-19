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

// capturingProvider captures the ChatRequest the client built, so tests can
// verify tools passed via ChatBuilder.Tools arrive intact.
type capturingProvider struct {
	lastReq *core.ChatRequest
}

func (p *capturingProvider) ID() string                         { return "capturing" }
func (p *capturingProvider) Models() []core.ModelInfo           { return nil }
func (p *capturingProvider) Supports(feature core.Feature) bool { return true }
func (p *capturingProvider) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	p.lastReq = req
	return &core.ChatResponse{ID: "resp-1", Model: req.Model, Output: "ok"}, nil
}
func (p *capturingProvider) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	p.lastReq = req
	ch := make(chan core.ChatChunk, 1)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)
	go func() {
		ch <- core.ChatChunk{Delta: "ok"}
		close(ch)
		finalCh <- &core.ChatResponse{ID: "resp-1", Model: req.Model, Output: "ok"}
		close(finalCh)
		close(errCh)
	}()
	return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

func TestToolSchemaReachesProviderThroughClient(t *testing.T) {
	schema := tools.ToolSchema{JSONSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)}
	tool := &mockTool{
		name:        "get_weather",
		description: "Get weather",
		schema:      schema,
		callFn:      func(ctx context.Context, args json.RawMessage) (any, error) { return "sunny", nil },
	}

	provider := &capturingProvider{}
	client := core.NewClient(provider)

	_, err := client.Chat("gpt-4").User("Weather in Tokyo?").Tools(tool).GetResponse(context.Background())
	if err != nil {
		t.Fatalf("GetResponse() error = %v", err)
	}

	if provider.lastReq == nil || len(provider.lastReq.Tools) != 1 {
		t.Fatalf("request did not carry the tool (tools: %v)", provider.lastReq)
	}

	got := provider.lastReq.Tools[0].Schema().JSONSchema
	if string(got) != string(schema.JSONSchema) {
		t.Errorf("schema at provider = %s, want %s", got, schema.JSONSchema)
	}
}
