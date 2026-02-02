package core

import (
	"errors"
	"testing"
)

func TestProviderErrorImplementsError(t *testing.T) {
	err := &ProviderError{
		Provider:  "openai",
		Status:    401,
		RequestID: "req_123",
		Code:      "invalid_api_key",
		Message:   "Invalid API key provided",
	}

	// Verify it implements error interface
	var _ error = err

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() returned empty string")
	}

	// Check that key fields are in error message
	if !contains(errStr, "openai") {
		t.Error("Error() should contain provider name")
	}
	if !contains(errStr, "401") {
		t.Error("Error() should contain status code")
	}
	if !contains(errStr, "req_123") {
		t.Error("Error() should contain request ID")
	}
	if !contains(errStr, "invalid_api_key") {
		t.Error("Error() should contain error code")
	}
}

func TestProviderErrorWithoutRequestID(t *testing.T) {
	err := &ProviderError{
		Provider: "anthropic",
		Status:   429,
		Code:     "rate_limit_exceeded",
		Message:  "Rate limit exceeded",
	}

	errStr := err.Error()

	if !contains(errStr, "anthropic") {
		t.Error("Error() should contain provider name")
	}
	if !contains(errStr, "429") {
		t.Error("Error() should contain status code")
	}
	// Should not contain request_id when empty
	if contains(errStr, "request_id") {
		t.Error("Error() should not contain request_id when empty")
	}
}

func TestProviderErrorUnwrap(t *testing.T) {
	underlying := ErrRateLimited

	err := &ProviderError{
		Provider: "openai",
		Status:   429,
		Code:     "rate_limit",
		Message:  "Too many requests",
		Err:      underlying,
	}

	// Test Unwrap returns the underlying error
	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}

	// Test errors.Is works with wrapped error
	if !errors.Is(err, ErrRateLimited) {
		t.Error("errors.Is(err, ErrRateLimited) should be true")
	}
}

func TestProviderErrorUnwrapNil(t *testing.T) {
	err := &ProviderError{
		Provider: "openai",
		Status:   400,
		Code:     "bad_request",
		Message:  "Bad request",
		Err:      nil,
	}

	if err.Unwrap() != nil {
		t.Error("Unwrap() should return nil when Err is nil")
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Fatal("ErrNotFound should not be nil")
	}
	if ErrNotFound.Error() != "not found" {
		t.Errorf("expected 'not found', got %q", ErrNotFound.Error())
	}
}

func TestProviderErrorUnwrapsNotFound(t *testing.T) {
	err := &ProviderError{
		Provider: "test",
		Status:   404,
		Code:     "not_found",
		Message:  "resource not found",
		Err:      ErrNotFound,
	}

	if !errors.Is(err, ErrNotFound) {
		t.Error("expected ProviderError to unwrap to ErrNotFound")
	}
}

func TestSentinelErrorsCanBeCheckedWithErrorsIs(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
	}{
		{"ErrUnauthorized", ErrUnauthorized},
		{"ErrRateLimited", ErrRateLimited},
		{"ErrBadRequest", ErrBadRequest},
		{"ErrNotFound", ErrNotFound},
		{"ErrServer", ErrServer},
		{"ErrNetwork", ErrNetwork},
		{"ErrDecode", ErrDecode},
		{"ErrModelRequired", ErrModelRequired},
		{"ErrNoMessages", ErrNoMessages},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Direct check
			if !errors.Is(tt.sentinel, tt.sentinel) {
				t.Errorf("errors.Is(%v, %v) should be true", tt.sentinel, tt.sentinel)
			}

			// Wrapped check
			wrapped := &ProviderError{
				Provider: "test",
				Status:   500,
				Code:     "test",
				Message:  "test",
				Err:      tt.sentinel,
			}
			if !errors.Is(wrapped, tt.sentinel) {
				t.Errorf("errors.Is(wrapped, %v) should be true", tt.sentinel)
			}
		})
	}
}

func TestSentinelErrorsAreDifferent(t *testing.T) {
	sentinels := []error{
		ErrUnauthorized,
		ErrRateLimited,
		ErrBadRequest,
		ErrNotFound,
		ErrServer,
		ErrNetwork,
		ErrDecode,
		ErrModelRequired,
		ErrNoMessages,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel errors should be distinct: %v == %v", a, b)
			}
		}
	}
}

func TestSentinelErrorMessages(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrUnauthorized, "unauthorized"},
		{ErrRateLimited, "rate limited"},
		{ErrBadRequest, "bad request"},
		{ErrNotFound, "not found"},
		{ErrServer, "server error"},
		{ErrNetwork, "network error"},
		{ErrDecode, "decode error"},
		{ErrModelRequired, "model required"},
		{ErrNoMessages, "no messages"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Create a chain of errors
	baseErr := ErrUnauthorized
	providerErr := &ProviderError{
		Provider: "openai",
		Status:   401,
		Code:     "invalid_api_key",
		Message:  "API key invalid",
		Err:      baseErr,
	}

	// Verify chain works
	if !errors.Is(providerErr, ErrUnauthorized) {
		t.Error("should be able to check for ErrUnauthorized in chain")
	}

	// Verify we can unwrap to get the base error
	var pe *ProviderError
	if !errors.As(providerErr, &pe) {
		t.Error("errors.As should work for ProviderError")
	}
	if pe.Provider != "openai" {
		t.Errorf("Provider = %v, want openai", pe.Provider)
	}
}

// Helper function
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
