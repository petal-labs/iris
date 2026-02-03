package commands

import (
	"errors"
	"testing"

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
			provider, err := createProvider(tt.providerID, tt.apiKey)
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
	_, err := createProvider("unsupported", "test-key")
	if err == nil {
		t.Fatal("createProvider() should return error for unsupported provider")
	}
}

func TestCreateProviderErrorMessage(t *testing.T) {
	_, err := createProvider("nonexistent", "test-key")
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
	// Test with validation error
	err := handleChatError(core.ErrModelRequired)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitValidation {
		t.Errorf("ExitCode() = %d, want %d (ExitValidation)", exitErr.ExitCode(), ExitValidation)
	}
}

func TestHandleChatErrorNetwork(t *testing.T) {
	err := handleChatError(core.ErrNetwork)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitNetwork {
		t.Errorf("ExitCode() = %d, want %d (ExitNetwork)", exitErr.ExitCode(), ExitNetwork)
	}
}

func TestHandleChatErrorProvider(t *testing.T) {
	provErr := &core.ProviderError{
		Provider:  "openai",
		Status:    429,
		RequestID: "req_123",
		Code:      "rate_limited",
		Message:   "Too many requests",
		Err:       core.ErrRateLimited,
	}

	err := handleChatError(provErr)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitProvider {
		t.Errorf("ExitCode() = %d, want %d (ExitProvider)", exitErr.ExitCode(), ExitProvider)
	}
}
