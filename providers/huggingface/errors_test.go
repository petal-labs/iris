package huggingface

import (
	"errors"
	"net/http"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestNormalizeError(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         []byte
		requestID    string
		wantCode     string
		wantMsg      string
		wantSentinel error
	}{
		{
			name:         "bad request",
			status:       http.StatusBadRequest,
			body:         []byte(`{"error":{"message":"Invalid model","type":"invalid_request_error","code":"invalid_model"}}`),
			requestID:    "req-123",
			wantCode:     "invalid_model",
			wantMsg:      "Invalid model",
			wantSentinel: core.ErrBadRequest,
		},
		{
			name:         "unauthorized",
			status:       http.StatusUnauthorized,
			body:         []byte(`{"error":{"message":"Invalid API key","type":"authentication_error"}}`),
			requestID:    "req-456",
			wantCode:     "authentication_error",
			wantMsg:      "Invalid API key",
			wantSentinel: core.ErrUnauthorized,
		},
		{
			name:         "forbidden",
			status:       http.StatusForbidden,
			body:         []byte(`{"error":{"message":"Access denied","type":"permission_error"}}`),
			requestID:    "",
			wantCode:     "permission_error",
			wantMsg:      "Access denied",
			wantSentinel: core.ErrUnauthorized,
		},
		{
			name:         "rate limited",
			status:       http.StatusTooManyRequests,
			body:         []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error","code":"rate_limit"}}`),
			requestID:    "req-789",
			wantCode:     "rate_limit",
			wantMsg:      "Rate limit exceeded",
			wantSentinel: core.ErrRateLimited,
		},
		{
			name:         "server error",
			status:       http.StatusInternalServerError,
			body:         []byte(`{"error":{"message":"Internal error","type":"server_error"}}`),
			requestID:    "req-abc",
			wantCode:     "server_error",
			wantMsg:      "Internal error",
			wantSentinel: core.ErrServer,
		},
		{
			name:         "bad gateway",
			status:       http.StatusBadGateway,
			body:         []byte(`{}`),
			requestID:    "",
			wantCode:     "",
			wantMsg:      "Bad Gateway",
			wantSentinel: core.ErrServer,
		},
		{
			name:         "invalid JSON body",
			status:       http.StatusBadRequest,
			body:         []byte(`not json`),
			requestID:    "req-def",
			wantCode:     "",
			wantMsg:      "Bad Request",
			wantSentinel: core.ErrBadRequest,
		},
		{
			name:         "empty body",
			status:       http.StatusServiceUnavailable,
			body:         nil,
			requestID:    "",
			wantCode:     "",
			wantMsg:      "Service Unavailable",
			wantSentinel: core.ErrServer,
		},
		{
			name:         "unknown 4xx status",
			status:       418, // I'm a teapot
			body:         []byte(`{}`),
			requestID:    "",
			wantCode:     "",
			wantMsg:      "I'm a teapot",
			wantSentinel: core.ErrServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := normalizeError(tt.status, tt.body, tt.requestID)

			var provErr *core.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatal("normalizeError() should return *core.ProviderError")
			}

			if provErr.Provider != "huggingface" {
				t.Errorf("Provider = %q, want %q", provErr.Provider, "huggingface")
			}

			if provErr.Status != tt.status {
				t.Errorf("Status = %d, want %d", provErr.Status, tt.status)
			}

			if provErr.RequestID != tt.requestID {
				t.Errorf("RequestID = %q, want %q", provErr.RequestID, tt.requestID)
			}

			if provErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", provErr.Code, tt.wantCode)
			}

			if provErr.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", provErr.Message, tt.wantMsg)
			}

			if !errors.Is(err, tt.wantSentinel) {
				t.Errorf("err should wrap %v", tt.wantSentinel)
			}
		})
	}
}

func TestNewNetworkError(t *testing.T) {
	originalErr := errors.New("connection refused")
	err := newNetworkError(originalErr)

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("newNetworkError() should return *core.ProviderError")
	}

	if provErr.Provider != "huggingface" {
		t.Errorf("Provider = %q, want %q", provErr.Provider, "huggingface")
	}

	if provErr.Message != "connection refused" {
		t.Errorf("Message = %q, want %q", provErr.Message, "connection refused")
	}

	if !errors.Is(err, core.ErrNetwork) {
		t.Error("err should wrap core.ErrNetwork")
	}
}

func TestNewDecodeError(t *testing.T) {
	originalErr := errors.New("unexpected EOF")
	err := newDecodeError(originalErr)

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("newDecodeError() should return *core.ProviderError")
	}

	if provErr.Provider != "huggingface" {
		t.Errorf("Provider = %q, want %q", provErr.Provider, "huggingface")
	}

	if provErr.Message != "unexpected EOF" {
		t.Errorf("Message = %q, want %q", provErr.Message, "unexpected EOF")
	}

	if !errors.Is(err, core.ErrDecode) {
		t.Error("err should wrap core.ErrDecode")
	}
}

func TestErrToolArgsInvalidJSON(t *testing.T) {
	if ErrToolArgsInvalidJSON.Error() != "tool args invalid json" {
		t.Errorf("ErrToolArgsInvalidJSON = %q, want %q", ErrToolArgsInvalidJSON.Error(), "tool args invalid json")
	}
}
