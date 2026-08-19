package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petal-labs/iris/cli/config"
	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
)

// mockChatProvider is a test double for core.Provider used by the CLI tests.
// It avoids HTTP entirely: chat/stream behavior is driven by the function
// fields, which may be nil for a trivial canned response.
type mockChatProvider struct {
	id     string
	models []core.ModelInfo
	sup    map[core.Feature]bool
	chat   func(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error)
	stream func(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error)
}

func (m *mockChatProvider) ID() string { return m.id }

func (m *mockChatProvider) Models() []core.ModelInfo { return m.models }

func (m *mockChatProvider) Supports(f core.Feature) bool {
	if m.sup == nil {
		return true
	}
	return m.sup[f]
}

func (m *mockChatProvider) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if m.chat != nil {
		return m.chat(ctx, req)
	}
	return &core.ChatResponse{ID: "resp-1", Model: req.Model, Output: "mock"}, nil
}

func (m *mockChatProvider) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	if m.stream != nil {
		return m.stream(ctx, req)
	}
	return nil, core.ErrNotSupported
}

// appWithMockProvider builds an App wired to a mock provider (registered under
// "ollama" so the keyless path applies and no API key is required) and an empty
// keystore. The returned provider may be mutated by callers to customize chat.
func appWithMockProvider(t *testing.T, mp *mockChatProvider) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := NewApp(
		WithProviderFactory(func(providerID, apiKey string, cfg *config.Config) (core.Provider, error) {
			return mp, nil
		}),
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader(""), &stdout, &stderr),
	)
	app.cfg = &config.Config{}
	app.provider = "ollama"
	app.model = "llama3.2"
	return app, &stdout, &stderr
}
