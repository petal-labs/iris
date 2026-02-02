package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

type WeatherArgs struct {
	Location string `json:"location"`
	Unit     string `json:"unit"`
}

type ComplexArgs struct {
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Enabled bool     `json:"enabled"`
	Tags    []string `json:"tags"`
}

func TestParseArgsSuccess(t *testing.T) {
	call := core.ToolCall{
		ID:        "call_123",
		Name:      "get_weather",
		Arguments: json.RawMessage(`{"location": "San Francisco", "unit": "celsius"}`),
	}

	args, err := tools.ParseArgs[WeatherArgs](call)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if args.Location != "San Francisco" {
		t.Errorf("args.Location = %q, want %q", args.Location, "San Francisco")
	}

	if args.Unit != "celsius" {
		t.Errorf("args.Unit = %q, want %q", args.Unit, "celsius")
	}
}

func TestParseArgsComplexType(t *testing.T) {
	call := core.ToolCall{
		ID:        "call_456",
		Name:      "complex_tool",
		Arguments: json.RawMessage(`{"name": "test", "count": 42, "enabled": true, "tags": ["a", "b", "c"]}`),
	}

	args, err := tools.ParseArgs[ComplexArgs](call)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if args.Name != "test" {
		t.Errorf("args.Name = %q, want %q", args.Name, "test")
	}

	if args.Count != 42 {
		t.Errorf("args.Count = %d, want %d", args.Count, 42)
	}

	if !args.Enabled {
		t.Error("args.Enabled = false, want true")
	}

	if len(args.Tags) != 3 {
		t.Errorf("len(args.Tags) = %d, want 3", len(args.Tags))
	}
}

func TestParseArgsInvalidJSON(t *testing.T) {
	call := core.ToolCall{
		ID:        "call_789",
		Name:      "broken",
		Arguments: json.RawMessage(`{invalid json`),
	}

	_, err := tools.ParseArgs[WeatherArgs](call)
	if err == nil {
		t.Error("ParseArgs() error = nil, want error for invalid JSON")
	}
}

func TestParseArgsTypeMismatch(t *testing.T) {
	// Provide a string where an int is expected
	call := core.ToolCall{
		ID:        "call_000",
		Name:      "type_mismatch",
		Arguments: json.RawMessage(`{"name": "test", "count": "not a number", "enabled": true, "tags": []}`),
	}

	_, err := tools.ParseArgs[ComplexArgs](call)
	if err == nil {
		t.Error("ParseArgs() error = nil, want error for type mismatch")
	}
}

func TestParseArgsEmptyArguments(t *testing.T) {
	call := core.ToolCall{
		ID:        "call_empty",
		Name:      "empty",
		Arguments: json.RawMessage(`{}`),
	}

	args, err := tools.ParseArgs[WeatherArgs](call)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	// Empty object should result in zero values
	if args.Location != "" {
		t.Errorf("args.Location = %q, want empty string", args.Location)
	}

	if args.Unit != "" {
		t.Errorf("args.Unit = %q, want empty string", args.Unit)
	}
}

func TestParseArgsExtraFields(t *testing.T) {
	// JSON has extra fields not in the struct - should be ignored
	call := core.ToolCall{
		ID:        "call_extra",
		Name:      "extra",
		Arguments: json.RawMessage(`{"location": "NYC", "unit": "fahrenheit", "extra_field": "ignored"}`),
	}

	args, err := tools.ParseArgs[WeatherArgs](call)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if args.Location != "NYC" {
		t.Errorf("args.Location = %q, want %q", args.Location, "NYC")
	}
}

func TestParseArgsNullArguments(t *testing.T) {
	call := core.ToolCall{
		ID:        "call_null",
		Name:      "null_args",
		Arguments: json.RawMessage(`null`),
	}

	// Unmarshaling null into a struct should work (results in zero value)
	args, err := tools.ParseArgs[WeatherArgs](call)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if args.Location != "" {
		t.Errorf("args.Location = %q, want empty string", args.Location)
	}
}

func TestParseArgsPointerType(t *testing.T) {
	type ArgsWithPointer struct {
		Value *string `json:"value"`
	}

	call := core.ToolCall{
		ID:        "call_ptr",
		Name:      "ptr_tool",
		Arguments: json.RawMessage(`{"value": "hello"}`),
	}

	args, err := tools.ParseArgs[ArgsWithPointer](call)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if args.Value == nil {
		t.Fatal("args.Value = nil, want non-nil")
	}

	if *args.Value != "hello" {
		t.Errorf("*args.Value = %q, want %q", *args.Value, "hello")
	}
}
