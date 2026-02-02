package providers

import (
	"context"
	"testing"

	"github.com/petal-labs/iris/core"
)

// mockProvider implements core.Provider for testing.
type mockProvider struct {
	id string
}

func (m *mockProvider) ID() string                 { return m.id }
func (m *mockProvider) Models() []core.ModelInfo   { return nil }
func (m *mockProvider) Supports(core.Feature) bool { return false }
func (m *mockProvider) Chat(context.Context, *core.ChatRequest) (*core.ChatResponse, error) {
	return nil, nil
}
func (m *mockProvider) StreamChat(context.Context, *core.ChatRequest) (*core.ChatStream, error) {
	return nil, nil
}

func TestRegister(t *testing.T) {
	// Register a test provider
	Register("test-provider", func(apiKey string) core.Provider {
		return &mockProvider{id: "test-provider"}
	})

	// Verify it's registered
	if !IsRegistered("test-provider") {
		t.Error("expected test-provider to be registered")
	}

	// Verify unregistered provider returns false
	if IsRegistered("nonexistent") {
		t.Error("expected nonexistent to not be registered")
	}
}

func TestGet(t *testing.T) {
	// Register a test provider
	Register("get-test", func(apiKey string) core.Provider {
		return &mockProvider{id: "get-test"}
	})

	// Get the factory
	factory := Get("get-test")
	if factory == nil {
		t.Fatal("expected factory to not be nil")
	}

	// Create a provider
	provider := factory("test-key")
	if provider.ID() != "get-test" {
		t.Errorf("expected ID 'get-test', got %q", provider.ID())
	}

	// Get non-existent provider
	if Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent provider")
	}
}

func TestCreate(t *testing.T) {
	// Register a test provider
	Register("create-test", func(apiKey string) core.Provider {
		return &mockProvider{id: "create-test-" + apiKey}
	})

	// Create provider
	provider, err := Create("create-test", "my-key")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if provider.ID() != "create-test-my-key" {
		t.Errorf("expected ID 'create-test-my-key', got %q", provider.ID())
	}

	// Create non-existent provider
	_, err = Create("nonexistent", "key")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestList(t *testing.T) {
	// Register some test providers
	Register("list-a", func(apiKey string) core.Provider { return nil })
	Register("list-b", func(apiKey string) core.Provider { return nil })
	Register("list-c", func(apiKey string) core.Provider { return nil })

	// Get the list
	list := List()

	// Verify it's sorted and contains our providers
	found := make(map[string]bool)
	for _, name := range list {
		found[name] = true
	}

	for _, name := range []string{"list-a", "list-b", "list-c"} {
		if !found[name] {
			t.Errorf("expected %q to be in list", name)
		}
	}

	// Verify sorted order
	for i := 1; i < len(list); i++ {
		if list[i-1] > list[i] {
			t.Errorf("list not sorted: %q > %q", list[i-1], list[i])
		}
	}
}
