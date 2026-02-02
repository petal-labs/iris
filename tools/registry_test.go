package tools_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/petal-labs/iris/tools"
)

func newMockTool(name, description string) *mockTool {
	return &mockTool{
		name:        name,
		description: description,
		schema:      tools.ToolSchema{JSONSchema: json.RawMessage(`{}`)},
		callFn:      func(ctx context.Context, args json.RawMessage) (any, error) { return nil, nil },
	}
}

func TestNewRegistry(t *testing.T) {
	r := tools.NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	list := r.List()
	if len(list) != 0 {
		t.Errorf("New registry has %d tools, want 0", len(list))
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := tools.NewRegistry()
	tool := newMockTool("my_tool", "My tool description")

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := r.Get("my_tool")
	if !ok {
		t.Fatal("Get() returned false, want true")
	}

	if got.Name() != "my_tool" {
		t.Errorf("Get() returned tool with Name() = %q, want %q", got.Name(), "my_tool")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := tools.NewRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get() returned true for nonexistent tool, want false")
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	r := tools.NewRegistry()
	tool1 := newMockTool("duplicate", "First tool")
	tool2 := newMockTool("duplicate", "Second tool")

	if err := r.Register(tool1); err != nil {
		t.Fatalf("First Register() error = %v", err)
	}

	err := r.Register(tool2)
	if err == nil {
		t.Fatal("Second Register() error = nil, want ErrDuplicateTool")
	}

	if err != tools.ErrDuplicateTool {
		t.Errorf("Second Register() error = %v, want ErrDuplicateTool", err)
	}
}

func TestRegistryRegisterNil(t *testing.T) {
	r := tools.NewRegistry()

	err := r.Register(nil)
	if err == nil {
		t.Error("Register(nil) error = nil, want error")
	}
}

func TestRegistryList(t *testing.T) {
	r := tools.NewRegistry()
	tool1 := newMockTool("tool1", "First")
	tool2 := newMockTool("tool2", "Second")
	tool3 := newMockTool("tool3", "Third")

	_ = r.Register(tool1)
	_ = r.Register(tool2)
	_ = r.Register(tool3)

	list := r.List()
	if len(list) != 3 {
		t.Errorf("List() returned %d tools, want 3", len(list))
	}

	// Verify all tools are in the list
	names := make(map[string]bool)
	for _, tool := range list {
		names[tool.Name()] = true
	}

	for _, name := range []string{"tool1", "tool2", "tool3"} {
		if !names[name] {
			t.Errorf("List() missing tool %q", name)
		}
	}
}

func TestRegistryListReturnsCopy(t *testing.T) {
	r := tools.NewRegistry()
	tool := newMockTool("tool", "A tool")
	_ = r.Register(tool)

	list1 := r.List()
	list2 := r.List()

	// Modifying one list should not affect the other
	if len(list1) > 0 {
		list1[0] = nil
	}

	if list2[0] == nil {
		t.Error("Modifying List() result affected subsequent List() calls")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := tools.NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tool := newMockTool(string(rune('a'+n%26))+string(rune('0'+n)), "Tool")
			_ = r.Register(tool) // Ignore duplicate errors
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.List()
			_, _ = r.Get("nonexistent")
		}()
	}

	wg.Wait()
}
