package voyageai

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestNew(t *testing.T) {
	p := New("test-key")
	if p == nil {
		t.Fatal("New() returned nil")
	}
}

func TestVoyageAI_ID(t *testing.T) {
	p := New("test-key")
	if p.ID() != "voyageai" {
		t.Errorf("ID() = %q, want voyageai", p.ID())
	}
}

func TestVoyageAI_Supports(t *testing.T) {
	p := New("test-key")

	tests := []struct {
		feature core.Feature
		want    bool
	}{
		{core.FeatureEmbeddings, true},
		{core.FeatureContextualizedEmbeddings, true},
		{core.FeatureReranking, true},
		{core.FeatureChat, false},
		{core.FeatureChatStreaming, false},
	}

	for _, tt := range tests {
		if got := p.Supports(tt.feature); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.feature, got, tt.want)
		}
	}
}

func TestVoyageAI_WithBaseURL(t *testing.T) {
	p := New("test-key", WithBaseURL("https://custom.api.com"))
	if p.config.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want https://custom.api.com", p.config.BaseURL)
	}
}
