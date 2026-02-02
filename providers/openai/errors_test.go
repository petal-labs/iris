package openai

import (
	"errors"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestNormalizeError400(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid model","type":"invalid_request_error","code":"invalid_model"}}`)
	err := normalizeError(400, body, "req-123")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if pErr.Status != 400 {
		t.Errorf("Status = %d, want 400", pErr.Status)
	}

	if pErr.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", pErr.RequestID, "req-123")
	}

	if pErr.Message != "Invalid model" {
		t.Errorf("Message = %q, want %q", pErr.Message, "Invalid model")
	}

	if !errors.Is(err, core.ErrBadRequest) {
		t.Error("expected error to wrap ErrBadRequest")
	}
}

func TestNormalizeError401(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid API key"}}`)
	err := normalizeError(401, body, "")

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Error("expected error to wrap ErrUnauthorized")
	}
}

func TestNormalizeError403(t *testing.T) {
	body := []byte(`{"error":{"message":"Access denied"}}`)
	err := normalizeError(403, body, "")

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Error("expected error to wrap ErrUnauthorized")
	}
}

func TestNormalizeError429(t *testing.T) {
	body := []byte(`{"error":{"message":"Rate limit exceeded"}}`)
	err := normalizeError(429, body, "req-456")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if !errors.Is(err, core.ErrRateLimited) {
		t.Error("expected error to wrap ErrRateLimited")
	}

	if pErr.RequestID != "req-456" {
		t.Errorf("RequestID = %q, want %q", pErr.RequestID, "req-456")
	}
}

func TestNormalizeError500(t *testing.T) {
	body := []byte(`{"error":{"message":"Internal server error"}}`)
	err := normalizeError(500, body, "")

	if !errors.Is(err, core.ErrServer) {
		t.Error("expected error to wrap ErrServer")
	}
}

func TestNormalizeError502(t *testing.T) {
	body := []byte(`{}`)
	err := normalizeError(502, body, "")

	if !errors.Is(err, core.ErrServer) {
		t.Error("expected error to wrap ErrServer")
	}
}

func TestNormalizeError503(t *testing.T) {
	body := []byte(`{"error":{"message":"Service unavailable"}}`)
	err := normalizeError(503, body, "")

	if !errors.Is(err, core.ErrServer) {
		t.Error("expected error to wrap ErrServer")
	}
}

func TestNormalizeErrorEmptyBody(t *testing.T) {
	err := normalizeError(500, []byte{}, "")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	// Should use HTTP status text as fallback message
	if pErr.Message != "Internal Server Error" {
		t.Errorf("Message = %q, want %q", pErr.Message, "Internal Server Error")
	}
}

func TestNormalizeErrorInvalidJSON(t *testing.T) {
	err := normalizeError(400, []byte(`not json`), "")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	// Should use HTTP status text as fallback
	if pErr.Message != "Bad Request" {
		t.Errorf("Message = %q, want %q", pErr.Message, "Bad Request")
	}
}

func TestNormalizeErrorCodeFromType(t *testing.T) {
	body := []byte(`{"error":{"message":"Error","type":"invalid_request_error"}}`)
	err := normalizeError(400, body, "")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	// Should use type as code when code is not present
	if pErr.Code != "invalid_request_error" {
		t.Errorf("Code = %q, want %q", pErr.Code, "invalid_request_error")
	}
}

func TestNormalizeErrorProvider(t *testing.T) {
	err := normalizeError(400, []byte(`{}`), "")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if pErr.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", pErr.Provider, "openai")
	}
}

func TestNewNetworkError(t *testing.T) {
	originalErr := errors.New("connection refused")
	err := newNetworkError(originalErr)

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if pErr.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", pErr.Provider, "openai")
	}

	if !errors.Is(err, core.ErrNetwork) {
		t.Error("expected error to wrap ErrNetwork")
	}

	if pErr.Message != "connection refused" {
		t.Errorf("Message = %q, want %q", pErr.Message, "connection refused")
	}
}

func TestNewDecodeError(t *testing.T) {
	originalErr := errors.New("unexpected EOF")
	err := newDecodeError(originalErr)

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if !errors.Is(err, core.ErrDecode) {
		t.Error("expected error to wrap ErrDecode")
	}
}
