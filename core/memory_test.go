package core

import (
	"sync"
	"testing"
)

// -----------------------------------------------------------------------------
// InMemoryStore Tests
// -----------------------------------------------------------------------------

func TestInMemoryStoreAddMessage(t *testing.T) {
	store := NewInMemoryStore()

	msg := Message{Role: RoleUser, Content: "Hello"}
	store.AddMessage(msg)

	if store.Len() != 1 {
		t.Errorf("Len() = %d, want 1", store.Len())
	}

	history := store.GetHistory()
	if len(history) != 1 {
		t.Fatalf("GetHistory() len = %d, want 1", len(history))
	}
	if history[0].Content != "Hello" {
		t.Errorf("Content = %q, want %q", history[0].Content, "Hello")
	}
}

func TestInMemoryStoreAddMessages(t *testing.T) {
	store := NewInMemoryStore()

	msgs := []Message{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there"},
		{Role: RoleUser, Content: "How are you?"},
	}
	store.AddMessages(msgs)

	if store.Len() != 3 {
		t.Errorf("Len() = %d, want 3", store.Len())
	}

	// Test empty add
	store.AddMessages(nil)
	store.AddMessages([]Message{})
	if store.Len() != 3 {
		t.Errorf("Len() after empty adds = %d, want 3", store.Len())
	}
}

func TestInMemoryStoreGetLastN(t *testing.T) {
	store := NewInMemoryStore()

	msgs := []Message{
		{Role: RoleUser, Content: "1"},
		{Role: RoleAssistant, Content: "2"},
		{Role: RoleUser, Content: "3"},
		{Role: RoleAssistant, Content: "4"},
		{Role: RoleUser, Content: "5"},
	}
	store.AddMessages(msgs)

	tests := []struct {
		n    int
		want []string
	}{
		{0, nil},
		{-1, nil},
		{2, []string{"4", "5"}},
		{3, []string{"3", "4", "5"}},
		{5, []string{"1", "2", "3", "4", "5"}},
		{10, []string{"1", "2", "3", "4", "5"}}, // More than exists
	}

	for _, tc := range tests {
		got := store.GetLastN(tc.n)
		if tc.want == nil {
			if got != nil {
				t.Errorf("GetLastN(%d) = %v, want nil", tc.n, got)
			}
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("GetLastN(%d) len = %d, want %d", tc.n, len(got), len(tc.want))
			continue
		}
		for i, want := range tc.want {
			if got[i].Content != want {
				t.Errorf("GetLastN(%d)[%d].Content = %q, want %q", tc.n, i, got[i].Content, want)
			}
		}
	}
}

func TestInMemoryStoreClear(t *testing.T) {
	store := NewInMemoryStore()
	store.AddMessages([]Message{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi"},
	})

	store.Clear()

	if store.Len() != 0 {
		t.Errorf("Len() after Clear = %d, want 0", store.Len())
	}
}

func TestInMemoryStoreSetMessages(t *testing.T) {
	store := NewInMemoryStore()
	store.AddMessage(Message{Role: RoleUser, Content: "Original"})

	newMsgs := []Message{
		{Role: RoleSystem, Content: "System"},
		{Role: RoleUser, Content: "New"},
	}
	store.SetMessages(newMsgs)

	if store.Len() != 2 {
		t.Errorf("Len() = %d, want 2", store.Len())
	}

	history := store.GetHistory()
	if history[0].Role != RoleSystem {
		t.Errorf("First message role = %q, want %q", history[0].Role, RoleSystem)
	}
}

func TestInMemoryStoreGetHistoryReturnsCopy(t *testing.T) {
	store := NewInMemoryStore()
	store.AddMessage(Message{Role: RoleUser, Content: "Original"})

	history := store.GetHistory()
	history[0].Content = "Modified"

	// Original should be unchanged
	newHistory := store.GetHistory()
	if newHistory[0].Content != "Original" {
		t.Error("GetHistory did not return a copy")
	}
}

func TestInMemoryStoreConcurrency(t *testing.T) {
	store := NewInMemoryStore()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.AddMessage(Message{Role: RoleUser, Content: "msg"})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.GetHistory()
			_ = store.Len()
			_ = store.GetLastN(5)
		}()
	}

	wg.Wait()

	if store.Len() != 100 {
		t.Errorf("Len() = %d, want 100 after concurrent operations", store.Len())
	}
}

// -----------------------------------------------------------------------------
// Conversation Tests
// -----------------------------------------------------------------------------

func TestNewConversation(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	conv := NewConversation(client, "test-model")

	if conv.MessageCount() != 0 {
		t.Errorf("MessageCount() = %d, want 0", conv.MessageCount())
	}
}

func TestNewConversationWithSystemMessage(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	conv := NewConversation(client, "test-model", WithSystemMessage("You are helpful"))

	if conv.MessageCount() != 1 {
		t.Errorf("MessageCount() = %d, want 1", conv.MessageCount())
	}

	history := conv.GetHistory()
	if history[0].Role != RoleSystem {
		t.Errorf("First message role = %q, want %q", history[0].Role, RoleSystem)
	}
	if history[0].Content != "You are helpful" {
		t.Errorf("System message = %q, want %q", history[0].Content, "You are helpful")
	}
}

func TestNewConversationWithCustomMemory(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	customMemory := NewInMemoryStore()
	customMemory.AddMessage(Message{Role: RoleUser, Content: "Pre-existing"})

	conv := NewConversation(client, "test-model", WithMemoryStore(customMemory))

	if conv.MessageCount() != 1 {
		t.Errorf("MessageCount() = %d, want 1", conv.MessageCount())
	}
}

func TestConversationClear(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	conv := NewConversation(client, "test-model", WithSystemMessage("System"))
	conv.memory.AddMessage(Message{Role: RoleUser, Content: "Hello"})

	if conv.MessageCount() != 2 {
		t.Errorf("MessageCount() before clear = %d, want 2", conv.MessageCount())
	}

	conv.Clear()

	// System message should be re-added
	if conv.MessageCount() != 1 {
		t.Errorf("MessageCount() after clear = %d, want 1", conv.MessageCount())
	}
	history := conv.GetHistory()
	if history[0].Role != RoleSystem {
		t.Errorf("After clear, first message role = %q, want %q", history[0].Role, RoleSystem)
	}
}

func TestConversationClearNoSystem(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	conv := NewConversation(client, "test-model")
	conv.memory.AddMessage(Message{Role: RoleUser, Content: "Hello"})

	conv.Clear()

	if conv.MessageCount() != 0 {
		t.Errorf("MessageCount() after clear = %d, want 0", conv.MessageCount())
	}
}

func TestMemoryInterfaceImplementation(t *testing.T) {
	// Verify InMemoryStore implements Memory interface
	var _ Memory = (*InMemoryStore)(nil)
}
