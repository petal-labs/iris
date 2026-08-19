package core

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// -----------------------------------------------------------------------------
// InMemoryStore Tests
// -----------------------------------------------------------------------------

func TestInMemoryStoreAddMessage(t *testing.T) {
	store := NewInMemoryStore()

	msg := Message{Role: RoleUser, Content: "Hello"}
	store.AddMessage(context.Background(), msg)

	if store.Len(context.Background()) != 1 {
		t.Errorf("Len() = %d, want 1", store.Len(context.Background()))
	}

	history := store.GetHistory(context.Background())
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
	store.AddMessages(context.Background(), msgs)

	if store.Len(context.Background()) != 3 {
		t.Errorf("Len() = %d, want 3", store.Len(context.Background()))
	}

	// Test empty add
	store.AddMessages(context.Background(), nil)
	store.AddMessages(context.Background(), []Message{})
	if store.Len(context.Background()) != 3 {
		t.Errorf("Len() after empty adds = %d, want 3", store.Len(context.Background()))
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
	store.AddMessages(context.Background(), msgs)

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
		got := store.GetLastN(context.Background(), tc.n)
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
	store.AddMessages(context.Background(), []Message{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi"},
	})

	store.Clear(context.Background())

	if store.Len(context.Background()) != 0 {
		t.Errorf("Len() after Clear = %d, want 0", store.Len(context.Background()))
	}
}

func TestInMemoryStoreSetMessages(t *testing.T) {
	store := NewInMemoryStore()
	store.AddMessage(context.Background(), Message{Role: RoleUser, Content: "Original"})

	newMsgs := []Message{
		{Role: RoleSystem, Content: "System"},
		{Role: RoleUser, Content: "New"},
	}
	store.SetMessages(context.Background(), newMsgs)

	if store.Len(context.Background()) != 2 {
		t.Errorf("Len() = %d, want 2", store.Len(context.Background()))
	}

	history := store.GetHistory(context.Background())
	if history[0].Role != RoleSystem {
		t.Errorf("First message role = %q, want %q", history[0].Role, RoleSystem)
	}
}

func TestInMemoryStoreGetHistoryReturnsCopy(t *testing.T) {
	store := NewInMemoryStore()
	store.AddMessage(context.Background(), Message{Role: RoleUser, Content: "Original"})

	history := store.GetHistory(context.Background())
	history[0].Content = "Modified"

	// Original should be unchanged
	newHistory := store.GetHistory(context.Background())
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
			store.AddMessage(context.Background(), Message{Role: RoleUser, Content: "msg"})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.GetHistory(context.Background())
			_ = store.Len(context.Background())
			_ = store.GetLastN(context.Background(), 5)
		}()
	}

	wg.Wait()

	if store.Len(context.Background()) != 100 {
		t.Errorf("Len() = %d, want 100 after concurrent operations", store.Len(context.Background()))
	}
}

// -----------------------------------------------------------------------------
// Conversation Tests
// -----------------------------------------------------------------------------

func TestNewConversation(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)
	ctx := context.Background()

	conv := NewConversation(ctx, client, "test-model")

	if conv.MessageCount(ctx) != 0 {
		t.Errorf("MessageCount() = %d, want 0", conv.MessageCount(ctx))
	}
}

func TestNewConversationWithSystemMessage(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)
	ctx := context.Background()

	conv := NewConversation(ctx, client, "test-model", WithSystemMessage("You are helpful"))

	if conv.MessageCount(ctx) != 1 {
		t.Errorf("MessageCount() = %d, want 1", conv.MessageCount(ctx))
	}

	history := conv.GetHistory(ctx)
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
	ctx := context.Background()

	customMemory := NewInMemoryStore()
	customMemory.AddMessage(ctx, Message{Role: RoleUser, Content: "Pre-existing"})

	conv := NewConversation(ctx, client, "test-model", WithMemoryStore(customMemory))

	if conv.MessageCount(ctx) != 1 {
		t.Errorf("MessageCount() = %d, want 1", conv.MessageCount(ctx))
	}
}

func TestConversationClear(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)
	ctx := context.Background()

	conv := NewConversation(ctx, client, "test-model", WithSystemMessage("System"))
	conv.memory.AddMessage(ctx, Message{Role: RoleUser, Content: "Hello"})

	if conv.MessageCount(ctx) != 2 {
		t.Errorf("MessageCount() before clear = %d, want 2", conv.MessageCount(ctx))
	}

	conv.Clear(ctx)

	// System message should be re-added
	if conv.MessageCount(ctx) != 1 {
		t.Errorf("MessageCount() after clear = %d, want 1", conv.MessageCount(ctx))
	}
	history := conv.GetHistory(ctx)
	if history[0].Role != RoleSystem {
		t.Errorf("After clear, first message role = %q, want %q", history[0].Role, RoleSystem)
	}
}

func TestConversationClearNoSystem(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)
	ctx := context.Background()

	conv := NewConversation(ctx, client, "test-model")
	conv.memory.AddMessage(ctx, Message{Role: RoleUser, Content: "Hello"})

	conv.Clear(ctx)

	if conv.MessageCount(ctx) != 0 {
		t.Errorf("MessageCount() after clear = %d, want 0", conv.MessageCount(ctx))
	}
}

func TestMemoryInterfaceImplementation(t *testing.T) {
	// Verify InMemoryStore implements Memory interface
	var _ Memory = (*InMemoryStore)(nil)
}

type contextRecordingMemory struct {
	*InMemoryStore
	seen []context.Context
}

func (m *contextRecordingMemory) AddMessage(ctx context.Context, msg Message) {
	m.seen = append(m.seen, ctx)
	m.InMemoryStore.AddMessage(ctx, msg)
}

func (m *contextRecordingMemory) GetHistory(ctx context.Context) []Message {
	m.seen = append(m.seen, ctx)
	return m.InMemoryStore.GetHistory(ctx)
}

func TestConversationSendPropagatesContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-60")
	memory := &contextRecordingMemory{InMemoryStore: NewInMemoryStore()}
	provider := &mockProvider{id: "test", chatFunc: func(got context.Context, _ *ChatRequest) (*ChatResponse, error) {
		if got.Value(contextKey{}) != "request-60" {
			t.Error("provider did not receive the conversation context")
		}
		return &ChatResponse{Output: "Hello!"}, nil
	}}
	conv := NewConversation(ctx, NewClient(provider), "test-model",
		WithMemoryStore(memory),
		WithSystemMessage("You are helpful"),
	)

	if _, err := conv.Send(ctx, "Hello"); err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if len(memory.seen) != 4 {
		t.Fatalf("memory context calls = %d, want 4", len(memory.seen))
	}
	for _, got := range memory.seen {
		if got.Value(contextKey{}) != "request-60" {
			t.Error("memory did not receive the conversation context")
		}
	}
}

func TestConversationStreamHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &mockProvider{id: "test", streamFunc: func(got context.Context, _ *ChatRequest) (*ChatStream, error) {
		return nil, got.Err()
	}}
	conv := NewConversation(context.Background(), NewClient(provider), "test-model")

	_, err := conv.Stream(ctx, "Hello")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Stream() error = %v, want context.Canceled", err)
	}
}

func TestConversationReplaysToolTurns(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	store.AddMessages(ctx, []Message{
		{Role: RoleUser, Content: "What's the weather?"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call-1", Name: "weather", Arguments: []byte(`{"city":"Tokyo"}`)}}},
		{Role: RoleTool, ToolResults: []ToolResult{{CallID: "call-1", Content: "sunny"}}},
	})

	var got []Message
	provider := &mockProvider{id: "test", chatFunc: func(_ context.Context, req *ChatRequest) (*ChatResponse, error) {
		got = append([]Message(nil), req.Messages...)
		return &ChatResponse{Output: "It is sunny."}, nil
	}}
	conv := NewConversation(ctx, NewClient(provider), "test-model", WithMemoryStore(store))

	if _, err := conv.Send(ctx, "What should I wear?"); err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("replayed message count = %d, want 4", len(got))
	}
	if len(got[1].ToolCalls) != 1 || got[1].ToolCalls[0].ID != "call-1" {
		t.Errorf("assistant tool calls = %#v, want call-1", got[1].ToolCalls)
	}
	if got[2].Role != RoleTool || len(got[2].ToolResults) != 1 {
		t.Errorf("tool result message = %#v, want one RoleTool result", got[2])
	}
}

func TestConversationStoresResponseToolCalls(t *testing.T) {
	ctx := context.Background()
	provider := &mockProvider{id: "test", chatFunc: func(_ context.Context, _ *ChatRequest) (*ChatResponse, error) {
		return &ChatResponse{ToolCalls: []ToolCall{{ID: "call-1", Name: "weather", Arguments: []byte(`{"city":"Tokyo"}`)}}}, nil
	}}
	conv := NewConversation(ctx, NewClient(provider), "test-model")

	if _, err := conv.Send(ctx, "What's the weather?"); err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	history := conv.GetHistory(ctx)
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
	if len(history[1].ToolCalls) != 1 || history[1].ToolCalls[0].ID != "call-1" {
		t.Errorf("stored tool calls = %#v, want call-1", history[1].ToolCalls)
	}
}

func TestConversationAddToolResultsReplaysCompleteTurn(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	var replayed []Message
	provider := &mockProvider{id: "test", chatFunc: func(_ context.Context, req *ChatRequest) (*ChatResponse, error) {
		callCount++
		if callCount == 1 {
			return &ChatResponse{ToolCalls: []ToolCall{{ID: "call-1", Name: "weather"}}}, nil
		}
		replayed = append([]Message(nil), req.Messages...)
		return &ChatResponse{Output: "It is sunny."}, nil
	}}
	conv := NewConversation(ctx, NewClient(provider), "test-model")

	if _, err := conv.Send(ctx, "What's the weather?"); err != nil {
		t.Fatalf("first Send() error: %v", err)
	}
	conv.AddToolResults(ctx, []ToolResult{{CallID: "call-1", Content: "sunny"}})
	if _, err := conv.Send(ctx, "What should I wear?"); err != nil {
		t.Fatalf("second Send() error: %v", err)
	}

	if len(replayed) != 4 {
		t.Fatalf("replayed message count = %d, want 4", len(replayed))
	}
	if len(replayed[1].ToolCalls) != 1 || replayed[2].Role != RoleTool {
		t.Errorf("replayed tool turn = %#v, want assistant call followed by tool result", replayed[1:3])
	}
}

// -----------------------------------------------------------------------------
// Conversation Streaming Tests
// -----------------------------------------------------------------------------

func TestConversationStream(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)
	ctx := context.Background()

	conv := NewConversation(ctx, client, "test-model")

	// Get stream
	stream, err := conv.Stream(ctx, "Hello")
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Use DrainStream for proper handling
	resp, err := DrainStream(ctx, stream)
	if err != nil {
		t.Fatalf("DrainStream error: %v", err)
	}
	if resp == nil {
		t.Fatal("No response received")
	}

	// Verify history was updated
	history := conv.GetHistory(ctx)
	if len(history) != 2 {
		t.Fatalf("History length = %d, want 2", len(history))
	}

	if history[0].Role != RoleUser {
		t.Errorf("First message role = %q, want %q", history[0].Role, RoleUser)
	}
	if history[0].Content != "Hello" {
		t.Errorf("First message content = %q, want %q", history[0].Content, "Hello")
	}

	if history[1].Role != RoleAssistant {
		t.Errorf("Second message role = %q, want %q", history[1].Role, RoleAssistant)
	}
}

func TestConversationStreamWithContextAlias(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)
	ctx := context.Background()

	conv := NewConversation(ctx, client, "test-model", WithSystemMessage("You are helpful"))

	stream, err := conv.StreamWithContext(ctx, "How are you?")
	if err != nil {
		t.Fatalf("StreamWithContext() error: %v", err)
	}

	// Use DrainStream for proper synchronization
	resp, err := DrainStream(ctx, stream)
	if err != nil {
		t.Fatalf("DrainStream error: %v", err)
	}
	if resp == nil {
		t.Fatal("No response received")
	}

	// History should have: system, user, assistant
	history := conv.GetHistory(ctx)
	if len(history) != 3 {
		t.Fatalf("History length = %d, want 3", len(history))
	}

	if history[0].Role != RoleSystem {
		t.Errorf("Message 0 role = %q, want %q", history[0].Role, RoleSystem)
	}
	if history[1].Role != RoleUser {
		t.Errorf("Message 1 role = %q, want %q", history[1].Role, RoleUser)
	}
	if history[2].Role != RoleAssistant {
		t.Errorf("Message 2 role = %q, want %q", history[2].Role, RoleAssistant)
	}
}

func TestConversationStreamMultipleTurns(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)
	ctx := context.Background()
	conv := NewConversation(ctx, client, "test-model")

	// First turn
	stream1, err := conv.Stream(ctx, "First")
	if err != nil {
		t.Fatalf("Stream() first turn error: %v", err)
	}
	_, err = DrainStream(ctx, stream1)
	if err != nil {
		t.Fatalf("DrainStream first turn error: %v", err)
	}

	// Second turn
	stream2, err := conv.Stream(ctx, "Second")
	if err != nil {
		t.Fatalf("Stream() second turn error: %v", err)
	}
	_, err = DrainStream(ctx, stream2)
	if err != nil {
		t.Fatalf("DrainStream second turn error: %v", err)
	}

	// Should have 4 messages: user1, assistant1, user2, assistant2
	history := conv.GetHistory(ctx)
	if len(history) != 4 {
		t.Fatalf("History length = %d, want 4", len(history))
	}

	if history[0].Content != "First" {
		t.Errorf("Message 0 content = %q, want %q", history[0].Content, "First")
	}
	if history[2].Content != "Second" {
		t.Errorf("Message 2 content = %q, want %q", history[2].Content, "Second")
	}
}

func TestConversationStreamStoresResponseToolCalls(t *testing.T) {
	provider := &mockProvider{id: "test", streamFunc: func(_ context.Context, _ *ChatRequest) (*ChatStream, error) {
		chunks := make(chan ChatChunk)
		errs := make(chan error)
		final := make(chan *ChatResponse, 1)
		close(chunks)
		close(errs)
		final <- &ChatResponse{ToolCalls: []ToolCall{{ID: "call-1", Name: "weather"}}}
		close(final)
		return &ChatStream{Ch: chunks, Err: errs, Final: final}, nil
	}}
	ctx := context.Background()
	conv := NewConversation(ctx, NewClient(provider), "test-model")

	stream, err := conv.Stream(ctx, "What's the weather?")
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if _, err := DrainStream(ctx, stream); err != nil {
		t.Fatalf("DrainStream() error: %v", err)
	}
	history := conv.GetHistory(ctx)
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
	if len(history[1].ToolCalls) != 1 || history[1].ToolCalls[0].ID != "call-1" {
		t.Errorf("stored tool calls = %#v, want call-1", history[1].ToolCalls)
	}
}
