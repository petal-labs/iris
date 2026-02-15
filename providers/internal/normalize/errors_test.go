package normalize

import (
	"errors"
	"net/http"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestOpenAIStyleProviderError(t *testing.T) {
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
			name:         "fallback to type",
			status:       http.StatusUnauthorized,
			body:         []byte(`{"error":{"message":"Invalid API key","type":"authentication_error"}}`),
			requestID:    "req-456",
			wantCode:     "authentication_error",
			wantMsg:      "Invalid API key",
			wantSentinel: core.ErrUnauthorized,
		},
		{
			name:         "fallback to status text",
			status:       http.StatusBadGateway,
			body:         []byte(`{}`),
			requestID:    "",
			wantCode:     "",
			wantMsg:      "Bad Gateway",
			wantSentinel: core.ErrServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := OpenAIStyleProviderError("test-provider", tt.status, tt.body, tt.requestID)

			var provErr *core.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatal("expected *core.ProviderError")
			}

			if provErr.Provider != "test-provider" {
				t.Errorf("Provider = %q, want test-provider", provErr.Provider)
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
				t.Errorf("error should wrap %v", tt.wantSentinel)
			}
		})
	}
}

func TestNetworkError(t *testing.T) {
	err := NetworkError("test-provider", errors.New("connection refused"))

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if provErr.Provider != "test-provider" {
		t.Errorf("Provider = %q, want test-provider", provErr.Provider)
	}
	if provErr.Message != "connection refused" {
		t.Errorf("Message = %q, want connection refused", provErr.Message)
	}
	if !errors.Is(err, core.ErrNetwork) {
		t.Error("error should wrap core.ErrNetwork")
	}
}

func TestDecodeError(t *testing.T) {
	err := DecodeError("test-provider", errors.New("unexpected EOF"))

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if provErr.Provider != "test-provider" {
		t.Errorf("Provider = %q, want test-provider", provErr.Provider)
	}
	if provErr.Message != "unexpected EOF" {
		t.Errorf("Message = %q, want unexpected EOF", provErr.Message)
	}
	if !errors.Is(err, core.ErrDecode) {
		t.Error("error should wrap core.ErrDecode")
	}
}

func TestProviderErrorDefaults(t *testing.T) {
	err := ProviderError("test-provider", http.StatusBadGateway, "req-1", "", "", nil)

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if provErr.Message != "Bad Gateway" {
		t.Errorf("Message = %q, want Bad Gateway", provErr.Message)
	}
	if !errors.Is(err, core.ErrServer) {
		t.Error("error should wrap core.ErrServer")
	}
}

func TestSentinelForStatusWithOverrides(t *testing.T) {
	sentinel := SentinelForStatusWithOverrides(http.StatusNotFound, map[int]error{
		http.StatusNotFound: core.ErrNotFound,
	})
	if !errors.Is(sentinel, core.ErrNotFound) {
		t.Errorf("sentinel = %v, want ErrNotFound", sentinel)
	}
}
