package testing

import (
	"context"
	"errors"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestMockProvider_ID(t *testing.T) {
	provider := NewMockProvider()
	if got := provider.ID(); got != "mock" {
		t.Errorf("ID() = %q, want %q", got, "mock")
	}

	provider.WithID("custom")
	if got := provider.ID(); got != "custom" {
		t.Errorf("ID() = %q, want %q", got, "custom")
	}
}

func TestMockProvider_Models(t *testing.T) {
	provider := NewMockProvider()
	models := provider.Models()
	if len(models) != 1 {
		t.Errorf("Models() returned %d models, want 1", len(models))
	}
	if models[0].ID != "mock-model" {
		t.Errorf("Models()[0].ID = %q, want %q", models[0].ID, "mock-model")
	}

	customModels := []core.ModelInfo{
		{ID: "model-1", DisplayName: "Model 1"},
		{ID: "model-2", DisplayName: "Model 2"},
	}
	provider.WithModels(customModels...)
	models = provider.Models()
	if len(models) != 2 {
		t.Errorf("Models() returned %d models, want 2", len(models))
	}
}

func TestMockProvider_Supports(t *testing.T) {
	provider := NewMockProvider()
	if provider.Supports(core.FeatureChat) {
		t.Error("Supports(FeatureChat) = true, want false (no features set)")
	}

	provider.WithFeatures(core.FeatureChat, core.FeatureToolCalling)
	if !provider.Supports(core.FeatureChat) {
		t.Error("Supports(FeatureChat) = false, want true")
	}
	if !provider.Supports(core.FeatureToolCalling) {
		t.Error("Supports(FeatureToolCalling) = false, want true")
	}
	if provider.Supports(core.FeatureReasoning) {
		t.Error("Supports(FeatureReasoning) = true, want false")
	}
}

func TestMockProvider_Chat_CannedResponses(t *testing.T) {
	ctx := context.Background()

	resp1 := core.ChatResponse{ID: "1", Output: "Hello!"}
	resp2 := core.ChatResponse{ID: "2", Output: "How are you?"}

	provider := NewMockProvider(resp1, resp2)
	req := &core.ChatRequest{Model: "test-model"}

	// First call returns first response
	got, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if got.Output != "Hello!" {
		t.Errorf("Chat() Output = %q, want %q", got.Output, "Hello!")
	}

	// Second call returns second response
	got, err = provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if got.Output != "How are you?" {
		t.Errorf("Chat() Output = %q, want %q", got.Output, "How are you?")
	}

	// Third call returns default response
	got, err = provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if got.Output != "mock response" {
		t.Errorf("Chat() Output = %q, want %q", got.Output, "mock response")
	}
}

func TestMockProvider_Chat_WithError(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	provider := NewMockProvider(core.ChatResponse{Output: "success"}).
		WithError(testErr)

	req := &core.ChatRequest{Model: "test-model"}

	// First call returns error
	_, err := provider.Chat(ctx, req)
	if err != testErr {
		t.Errorf("Chat() error = %v, want %v", err, testErr)
	}

	// Second call returns response
	got, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if got.Output != "success" {
		t.Errorf("Chat() Output = %q, want %q", got.Output, "success")
	}
}

func TestMockProvider_Chat_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	provider := NewMockProvider(core.ChatResponse{Output: "should not return"})
	req := &core.ChatRequest{Model: "test-model"}

	_, err := provider.Chat(ctx, req)
	if err != context.Canceled {
		t.Errorf("Chat() error = %v, want %v", err, context.Canceled)
	}
}

func TestMockProvider_Chat_RecordsCalls(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider()

	req1 := &core.ChatRequest{
		Model:    "model-1",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Hello"}},
	}
	req2 := &core.ChatRequest{
		Model:    "model-2",
		Messages: []core.Message{{Role: core.RoleUser, Content: "World"}},
	}

	provider.Chat(ctx, req1)
	provider.Chat(ctx, req2)

	calls := provider.Calls()
	if len(calls) != 2 {
		t.Fatalf("Calls() returned %d calls, want 2", len(calls))
	}

	if calls[0].Method != "Chat" {
		t.Errorf("calls[0].Method = %q, want %q", calls[0].Method, "Chat")
	}
	if calls[0].Request.Model != "model-1" {
		t.Errorf("calls[0].Request.Model = %q, want %q", calls[0].Request.Model, "model-1")
	}
	if calls[1].Request.Messages[0].Content != "World" {
		t.Errorf("calls[1].Request.Messages[0].Content = %q, want %q",
			calls[1].Request.Messages[0].Content, "World")
	}
}

func TestMockProvider_CallCount(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider()
	req := &core.ChatRequest{Model: "test"}

	if provider.CallCount() != 0 {
		t.Errorf("CallCount() = %d, want 0", provider.CallCount())
	}

	provider.Chat(ctx, req)
	provider.Chat(ctx, req)
	provider.Chat(ctx, req)

	if provider.CallCount() != 3 {
		t.Errorf("CallCount() = %d, want 3", provider.CallCount())
	}
}

func TestMockProvider_LastCall(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider()

	if provider.LastCall() != nil {
		t.Error("LastCall() should be nil when no calls made")
	}

	provider.Chat(ctx, &core.ChatRequest{Model: "first"})
	provider.Chat(ctx, &core.ChatRequest{Model: "second"})

	last := provider.LastCall()
	if last == nil {
		t.Fatal("LastCall() returned nil")
	}
	if last.Request.Model != "second" {
		t.Errorf("LastCall().Request.Model = %q, want %q", last.Request.Model, "second")
	}
}

func TestMockProvider_Reset(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider(
		core.ChatResponse{Output: "first"},
		core.ChatResponse{Output: "second"},
	)
	req := &core.ChatRequest{Model: "test"}

	// Make some calls
	provider.Chat(ctx, req)
	provider.Chat(ctx, req)

	if provider.CallCount() != 2 {
		t.Errorf("CallCount() = %d, want 2", provider.CallCount())
	}

	// Reset
	provider.Reset()

	if provider.CallCount() != 0 {
		t.Errorf("CallCount() after Reset = %d, want 0", provider.CallCount())
	}

	// Responses should start over
	got, _ := provider.Chat(ctx, req)
	if got.Output != "first" {
		t.Errorf("After Reset, first response = %q, want %q", got.Output, "first")
	}
}

func TestMockProvider_ResetAll(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider(core.ChatResponse{Output: "queued"}).
		WithError(errors.New("error"))

	provider.ResetAll()

	// Should not return the queued error or response
	got, err := provider.Chat(ctx, &core.ChatRequest{Model: "test"})
	if err != nil {
		t.Errorf("Chat() error = %v after ResetAll, want nil", err)
	}
	if got.Output != "mock response" {
		t.Errorf("Chat() Output = %q after ResetAll, want default", got.Output)
	}
}

func TestMockProvider_StreamChat_Basic(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider().
		WithStreamingResponse([]string{"Hello", " ", "World"}, nil)

	stream, err := provider.StreamChat(ctx, &core.ChatRequest{Model: "test"})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Collect chunks
	var chunks []string
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk.Delta)
	}

	if len(chunks) != 3 {
		t.Errorf("got %d chunks, want 3", len(chunks))
	}
	expected := []string{"Hello", " ", "World"}
	for i, c := range chunks {
		if c != expected[i] {
			t.Errorf("chunk[%d] = %q, want %q", i, c, expected[i])
		}
	}

	// Get final response
	final := <-stream.Final
	if final == nil {
		t.Fatal("Final response is nil")
	}
	if final.Output != "Hello World" {
		t.Errorf("Final.Output = %q, want %q", final.Output, "Hello World")
	}
}

func TestMockProvider_StreamChat_WithError(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("stream error")
	provider := NewMockProvider().
		WithStreamingError([]string{"partial"}, testErr)

	stream, err := provider.StreamChat(ctx, &core.ChatRequest{Model: "test"})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain chunks
	for range stream.Ch {
	}

	// Should get error
	streamErr := <-stream.Err
	if streamErr != testErr {
		t.Errorf("stream error = %v, want %v", streamErr, testErr)
	}
}

func TestMockProvider_StreamChat_WithCustomFinal(t *testing.T) {
	ctx := context.Background()
	customFinal := &core.ChatResponse{
		ID:     "custom-id",
		Output: "custom output",
		Usage:  core.TokenUsage{TotalTokens: 100},
	}
	provider := NewMockProvider().
		WithStreamingResponse([]string{"chunk"}, customFinal)

	stream, err := provider.StreamChat(ctx, &core.ChatRequest{Model: "test"})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain chunks
	for range stream.Ch {
	}

	final := <-stream.Final
	if final.ID != "custom-id" {
		t.Errorf("Final.ID = %q, want %q", final.ID, "custom-id")
	}
	if final.Usage.TotalTokens != 100 {
		t.Errorf("Final.Usage.TotalTokens = %d, want 100", final.Usage.TotalTokens)
	}
}

func TestMockProvider_StreamChat_RecordsCalls(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider()

	provider.StreamChat(ctx, &core.ChatRequest{Model: "stream-test"})

	calls := provider.Calls()
	if len(calls) != 1 {
		t.Fatalf("Calls() = %d, want 1", len(calls))
	}
	if calls[0].Method != "StreamChat" {
		t.Errorf("calls[0].Method = %q, want %q", calls[0].Method, "StreamChat")
	}
}

func TestMockProvider_StreamChat_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := NewMockProvider()
	_, err := provider.StreamChat(ctx, &core.ChatRequest{Model: "test"})
	if err != context.Canceled {
		t.Errorf("StreamChat() error = %v, want %v", err, context.Canceled)
	}
}

func TestMockProvider_WithDefaultResponse(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider().
		WithDefaultResponse(core.ChatResponse{Output: "custom default"})

	got, err := provider.Chat(ctx, &core.ChatRequest{Model: "test"})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if got.Output != "custom default" {
		t.Errorf("Chat() Output = %q, want %q", got.Output, "custom default")
	}
}

func TestMockProvider_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider()
	req := &core.ChatRequest{Model: "test"}

	// Run concurrent calls
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			provider.Chat(ctx, req)
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	if provider.CallCount() != 10 {
		t.Errorf("CallCount() = %d, want 10", provider.CallCount())
	}
}
