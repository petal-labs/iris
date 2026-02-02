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

func TestCreateProviderOpenAI(t *testing.T) {
	provider, err := createProvider("openai", "test-key")
	if err != nil {
		t.Fatalf("createProvider() error = %v", err)
	}

	if provider.ID() != "openai" {
		t.Errorf("provider.ID() = %q, want 'openai'", provider.ID())
	}
}

func TestCreateProviderUnsupported(t *testing.T) {
	_, err := createProvider("unsupported", "test-key")
	if err == nil {
		t.Fatal("createProvider() should return error for unsupported provider")
	}
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
