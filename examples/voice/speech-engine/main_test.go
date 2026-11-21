package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
)

type mockLLM struct{}

func (m *mockLLM) ID() string                   { return "mock-llm" }
func (m *mockLLM) Models() []core.ModelInfo     { return nil }
func (m *mockLLM) Supports(f core.Feature) bool { return f == core.FeatureChat }
func (m *mockLLM) Chat(context.Context, *core.ChatRequest) (*core.ChatResponse, error) {
	return &core.ChatResponse{Output: "ok"}, nil
}
func (m *mockLLM) StreamChat(context.Context, *core.ChatRequest) (*core.ChatStream, error) {
	return nil, core.ErrNotSupported
}

func TestBuildSpeechEngineMux(t *testing.T) {
	t.Run("mounts handler with JWT protection and health endpoint", func(t *testing.T) {
		el := elevenlabs.New("test-key")
		mux, err := buildSpeechEngineMux(el, &mockLLM{}, speechEngineExampleConfig{
			Path:  "/speech-engine",
			Model: "test-model",
		})
		if err != nil {
			t.Fatalf("buildSpeechEngineMux() err = %v", err)
		}

		// Health check responds 200 OK.
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET /healthz status = %d, want 200", rec.Code)
		}

		// Missing JWT header is rejected with 401 Unauthorized.
		recAuth := httptest.NewRecorder()
		mux.ServeHTTP(recAuth, httptest.NewRequest(http.MethodGet, "/speech-engine", nil))
		if recAuth.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated /speech-engine status = %d, want 401", recAuth.Code)
		}
	})

	t.Run("nil provider fails fast", func(t *testing.T) {
		if _, err := buildSpeechEngineMux(nil, &mockLLM{}, speechEngineExampleConfig{Model: "m"}); err == nil {
			t.Fatal("expected error for nil elevenlabs provider")
		}
	})

	t.Run("missing LLM fails validation", func(t *testing.T) {
		el := elevenlabs.New("test-key")
		if _, err := buildSpeechEngineMux(el, nil, speechEngineExampleConfig{Model: "m"}); err == nil {
			t.Fatal("expected error for nil LLM")
		}
	})
}
