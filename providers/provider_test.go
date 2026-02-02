package providers

import (
	"context"
	"testing"

	"github.com/petal-labs/iris/core"
)

// testProvider is a mock implementation of Provider for testing.
type testProvider struct {
	id       string
	models   []ModelInfo
	features map[Feature]bool
}

func (p *testProvider) ID() string {
	return p.id
}

func (p *testProvider) Models() []ModelInfo {
	return p.models
}

func (p *testProvider) Supports(feature Feature) bool {
	return p.features[feature]
}

func (p *testProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{
		ID:     "test-resp",
		Model:  req.Model,
		Output: "Test response",
		Usage:  TokenUsage{TotalTokens: 10},
	}, nil
}

func (p *testProvider) StreamChat(ctx context.Context, req *ChatRequest) (*ChatStream, error) {
	ch := make(chan ChatChunk, 1)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	go func() {
		ch <- ChatChunk{Delta: "Test"}
		close(ch)
		finalCh <- &ChatResponse{Output: "Test"}
		close(finalCh)
		close(errCh)
	}()

	return &ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

func TestProviderImplementsInterface(t *testing.T) {
	p := &testProvider{
		id: "test",
		models: []ModelInfo{
			{ID: "test-model", DisplayName: "Test Model", Capabilities: []Feature{FeatureChat}},
		},
		features: map[Feature]bool{
			FeatureChat: true,
		},
	}

	// Verify it implements core.Provider
	var _ core.Provider = p

	if p.ID() != "test" {
		t.Errorf("ID() = %v, want test", p.ID())
	}
}

func TestProviderModels(t *testing.T) {
	p := &testProvider{
		id: "test",
		models: []ModelInfo{
			{ID: "model-1", DisplayName: "Model One", Capabilities: []Feature{FeatureChat, FeatureChatStreaming}},
			{ID: "model-2", DisplayName: "Model Two", Capabilities: []Feature{FeatureChat, FeatureToolCalling}},
		},
	}

	models := p.Models()
	if len(models) != 2 {
		t.Fatalf("len(Models()) = %d, want 2", len(models))
	}

	if models[0].ID != "model-1" {
		t.Errorf("Models()[0].ID = %v, want model-1", models[0].ID)
	}
	if models[1].DisplayName != "Model Two" {
		t.Errorf("Models()[1].DisplayName = %v, want Model Two", models[1].DisplayName)
	}
}

func TestProviderSupports(t *testing.T) {
	p := &testProvider{
		id: "test",
		features: map[Feature]bool{
			FeatureChat:          true,
			FeatureChatStreaming: true,
			FeatureToolCalling:   false,
		},
	}

	if !p.Supports(FeatureChat) {
		t.Error("Supports(FeatureChat) should be true")
	}
	if !p.Supports(FeatureChatStreaming) {
		t.Error("Supports(FeatureChatStreaming) should be true")
	}
	if p.Supports(FeatureToolCalling) {
		t.Error("Supports(FeatureToolCalling) should be false")
	}
}

func TestModelInfoHasCapability(t *testing.T) {
	model := ModelInfo{
		ID:           "gpt-4",
		DisplayName:  "GPT-4",
		Capabilities: []Feature{FeatureChat, FeatureChatStreaming, FeatureToolCalling},
	}

	if !model.HasCapability(FeatureChat) {
		t.Error("HasCapability(FeatureChat) should be true")
	}
	if !model.HasCapability(FeatureToolCalling) {
		t.Error("HasCapability(FeatureToolCalling) should be true")
	}

	// Test capability not in list
	model2 := ModelInfo{
		ID:           "basic-model",
		Capabilities: []Feature{FeatureChat},
	}
	if model2.HasCapability(FeatureToolCalling) {
		t.Error("HasCapability(FeatureToolCalling) should be false")
	}
}

func TestProviderChat(t *testing.T) {
	p := &testProvider{id: "test"}

	resp, err := p.Chat(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp.Output != "Test response" {
		t.Errorf("Output = %v, want Test response", resp.Output)
	}
}

func TestProviderStreamChat(t *testing.T) {
	p := &testProvider{id: "test"}

	stream, err := p.StreamChat(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("StreamChat() error: %v", err)
	}

	// Read chunk
	chunk := <-stream.Ch
	if chunk.Delta != "Test" {
		t.Errorf("Delta = %v, want Test", chunk.Delta)
	}

	// Read final
	final := <-stream.Final
	if final.Output != "Test" {
		t.Errorf("Final.Output = %v, want Test", final.Output)
	}
}

func TestTypeAliasesWork(t *testing.T) {
	// Verify type aliases are usable
	var _ Provider = &testProvider{}
	var _ Feature = FeatureChat
	var _ ModelID = ModelID("gpt-4")
	var _ Role = RoleUser

	// Verify constants are accessible
	if FeatureChat != core.FeatureChat {
		t.Error("FeatureChat should equal core.FeatureChat")
	}
	if RoleUser != core.RoleUser {
		t.Error("RoleUser should equal core.RoleUser")
	}

	// Verify errors are accessible
	if ErrUnauthorized != core.ErrUnauthorized {
		t.Error("ErrUnauthorized should equal core.ErrUnauthorized")
	}
}
