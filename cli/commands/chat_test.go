package commands

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petal-labs/iris/cli/config"
	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
)

func TestExitError(t *testing.T) {
	err := exitWithCode(ExitValidation, errors.New("test error"))

	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want 'test error'", err.Error())
	}

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitValidation {
		t.Errorf("ExitCode() = %d, want %d", exitErr.ExitCode(), ExitValidation)
	}
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"success", ExitSuccess, 0},
		{"validation", ExitValidation, 1},
		{"provider", ExitProvider, 2},
		{"network", ExitNetwork, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("Exit%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

func TestCreateProviderAllProviders(t *testing.T) {
	app := NewApp()

	tests := []struct {
		providerID string
		apiKey     string
		wantID     string
	}{
		{"openai", "test-key", "openai"},
		{"anthropic", "test-key", "anthropic"},
		{"gemini", "test-key", "gemini"},
		{"xai", "test-key", "xai"},
		{"zai", "test-key", "zai"},
		{"ollama", "", "ollama"},         // ollama works without API key
		{"ollama", "test-key", "ollama"}, // ollama also works with API key
		{"huggingface", "test-key", "huggingface"},
	}

	for _, tt := range tests {
		t.Run(tt.providerID, func(t *testing.T) {
			provider, err := app.createProvider(tt.providerID, tt.apiKey, nil)
			if err != nil {
				t.Fatalf("createProvider(%q, %q) error = %v", tt.providerID, tt.apiKey, err)
			}

			if provider.ID() != tt.wantID {
				t.Errorf("provider.ID() = %q, want %q", provider.ID(), tt.wantID)
			}
		})
	}
}

func TestCreateProviderUnsupported(t *testing.T) {
	app := NewApp()
	_, err := app.createProvider("unsupported", "test-key", nil)
	if err == nil {
		t.Fatal("createProvider() should return error for unsupported provider")
	}
}

func TestCreateProviderErrorMessage(t *testing.T) {
	app := NewApp()
	_, err := app.createProvider("nonexistent", "test-key", nil)
	if err == nil {
		t.Fatal("createProvider() should return error")
	}

	// Error should mention unsupported provider
	errMsg := err.Error()
	if !contains(errMsg, "unsupported provider") {
		t.Errorf("Error message should contain 'unsupported provider', got: %q", errMsg)
	}
}

// contains checks if s contains substr (simple helper for tests)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHandleChatErrorValidation(t *testing.T) {
	app := NewApp()
	// Test with validation error
	err := app.handleChatError(core.ErrModelRequired)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitValidation {
		t.Errorf("ExitCode() = %d, want %d (ExitValidation)", exitErr.ExitCode(), ExitValidation)
	}
}

func TestHandleChatErrorNetwork(t *testing.T) {
	app := NewApp()
	err := app.handleChatError(core.ErrNetwork)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitNetwork {
		t.Errorf("ExitCode() = %d, want %d (ExitNetwork)", exitErr.ExitCode(), ExitNetwork)
	}
}

func TestHandleChatErrorProvider(t *testing.T) {
	app := NewApp()
	provErr := &core.ProviderError{
		Provider:  "openai",
		Status:    429,
		RequestID: "req_123",
		Code:      "rate_limited",
		Message:   "Too many requests",
		Err:       core.ErrRateLimited,
	}

	err := app.handleChatError(provErr)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitProvider {
		t.Errorf("ExitCode() = %d, want %d (ExitProvider)", exitErr.ExitCode(), ExitProvider)
	}
}

func TestProviderAllowsEmptyAPIKey(t *testing.T) {
	if !providerAllowsEmptyAPIKey("ollama") {
		t.Error("providerAllowsEmptyAPIKey(ollama) = false, want true")
	}
	for _, id := range []string{"openai", "anthropic", "gemini", "xai", "zai", "huggingface", ""} {
		if providerAllowsEmptyAPIKey(id) {
			t.Errorf("providerAllowsEmptyAPIKey(%q) = true, want false", id)
		}
	}
}

// TestRunChatKeylessOllama verifies that a missing keystore entry does not
// abort the request for providers that run without an API key (local
// Ollama). The fake Ollama server proves the request actually went out.
func TestRunChatKeylessOllama(t *testing.T) {
	var gotRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		gotRequest = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"llama3.2","created_at":"2026-08-17T00:00:00Z","message":{"role":"assistant","content":"hello"},"done":true}`)
	}))
	defer server.Close()

	// Keystore at an empty path: Get returns ErrKeyNotFound for everything.
	app := NewApp(
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}),
	)
	// runChat is invoked directly, so config is assigned here instead of
	// via the root command's PersistentPreRunE.
	app.cfg = &config.Config{Providers: map[string]config.ProviderConfig{
		"ollama": {BaseURL: server.URL},
	}}
	app.provider = "ollama"
	app.model = "llama3.2"
	app.chatPrompt = "hi"

	err := app.runChat(nil, nil)
	if err != nil {
		t.Fatalf("runChat() error = %v; want success with no stored key (local ollama)", err)
	}
	if !gotRequest {
		t.Error("no request reached the ollama server; keyless path did not proceed")
	}
}

// TestRunChatMissingKeyFailsForKeyedProviders verifies the actionable
// keystore hint still fires for providers that require a key.
func TestRunChatMissingKeyFailsForKeyedProviders(t *testing.T) {
	app := NewApp(
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}),
	)
	app.provider = "openai"
	app.model = "gpt-4o"
	app.chatPrompt = "hi"

	err := app.runChat(nil, nil)
	if err == nil {
		t.Fatal("runChat() should fail for openai with no stored key")
	}
	if !strings.Contains(err.Error(), "iris keys set openai") {
		t.Errorf("error = %q, want actionable 'iris keys set openai' hint", err.Error())
	}
}
