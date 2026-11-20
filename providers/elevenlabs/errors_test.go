package elevenlabs

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestNormalizeErrorShapesAndStatuses(t *testing.T) {
	t.Run("structured detail object with request_id fallback", func(t *testing.T) {
		body := []byte(`{"detail":{"type":"validation_error","code":"invalid_parameters","message":"unsupported keyterms","request_id":"req_body_123","param":"keyterms"}}`)
		err := normalizeError(http.StatusUnprocessableEntity, body, "")

		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("422 err = %v, want core.ErrBadRequest", err)
		}
		var pe *core.ProviderError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *core.ProviderError, got %T", err)
		}
		if pe.Provider != "elevenlabs" || pe.Status != 422 || pe.Code != "invalid_parameters" || pe.Message != "unsupported keyterms" || pe.RequestID != "req_body_123" {
			t.Errorf("unexpected ProviderError: %+v", pe)
		}
	})

	t.Run("detail object status field and header request_id precedence", func(t *testing.T) {
		body := []byte(`{"detail":{"status":"invalid_api_key","message":"Invalid API key","request_id":"body_id"}}`)
		err := normalizeError(http.StatusUnauthorized, body, "hdr_id")

		if !errors.Is(err, core.ErrUnauthorized) {
			t.Fatalf("401 err = %v, want core.ErrUnauthorized", err)
		}
		var pe *core.ProviderError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *core.ProviderError, got %T", err)
		}
		if pe.Code != "invalid_api_key" || pe.RequestID != "hdr_id" || pe.Message != "Invalid API key" {
			t.Errorf("unexpected ProviderError: %+v", pe)
		}
	})

	t.Run("detail object type fallback", func(t *testing.T) {
		body := []byte(`{"detail":{"type":"quota_exceeded","message":"character limit reached"}}`)
		err := normalizeError(http.StatusForbidden, body, "")
		if !errors.Is(err, core.ErrUnauthorized) {
			t.Fatalf("403 err = %v, want core.ErrUnauthorized", err)
		}
		var pe *core.ProviderError
		_ = errors.As(err, &pe)
		if pe.Code != "quota_exceeded" {
			t.Errorf("Code = %q, want quota_exceeded", pe.Code)
		}
	})

	t.Run("plain string detail", func(t *testing.T) {
		body := []byte(`{"detail":"Voice not found"}`)
		err := normalizeError(http.StatusNotFound, body, "req-404")

		if !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("404 err = %v, want core.ErrNotFound", err)
		}
		var pe *core.ProviderError
		_ = errors.As(err, &pe)
		if pe.Message != "Voice not found" || pe.RequestID != "req-404" {
			t.Errorf("unexpected ProviderError: %+v", pe)
		}
	})

	t.Run("FastAPI validation list detail", func(t *testing.T) {
		body := []byte(`{"detail":[{"loc":["body","text"],"msg":"field required","type":"value_error.missing"},{"loc":[],"msg":"bad input","type":""}]}`)
		err := normalizeError(http.StatusUnprocessableEntity, body, "")

		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("422 list err = %v, want core.ErrBadRequest", err)
		}
		var pe *core.ProviderError
		_ = errors.As(err, &pe)
		if pe.Code != "value_error.missing" {
			t.Errorf("Code = %q, want value_error.missing", pe.Code)
		}
		if pe.Message != "body.text: field required; bad input" {
			t.Errorf("Message = %q", pe.Message)
		}
	})

	t.Run("429 rate limited with Retry-After", func(t *testing.T) {
		headers := http.Header{"Retry-After": []string{"7"}}
		err := normalizeError(http.StatusTooManyRequests, []byte(`{"detail":"too many requests"}`), "req-429", headers)

		if !errors.Is(err, core.ErrRateLimited) {
			t.Fatalf("429 err = %v, want core.ErrRateLimited", err)
		}
		var pe *core.ProviderError
		_ = errors.As(err, &pe)
		if pe.RetryAfter != 7*time.Second {
			t.Errorf("RetryAfter = %s, want 7s", pe.RetryAfter)
		}
	})

	t.Run("500 raw text and oversized body truncation", func(t *testing.T) {
		oversized := []byte(strings.Repeat("x", maxBodyLen+200))
		err := normalizeError(http.StatusInternalServerError, oversized, "")

		if !errors.Is(err, core.ErrServer) {
			t.Fatalf("500 err = %v, want core.ErrServer", err)
		}
		var pe *core.ProviderError
		_ = errors.As(err, &pe)
		if !strings.HasSuffix(pe.Body, "…(truncated)") {
			t.Errorf("expected truncated body suffix, got len=%d", len(pe.Body))
		}
	})

	t.Run("empty body falls back to status text", func(t *testing.T) {
		err := normalizeError(http.StatusBadGateway, nil, "")
		if !errors.Is(err, core.ErrServer) {
			t.Fatalf("502 err = %v, want core.ErrServer", err)
		}
		var pe *core.ProviderError
		_ = errors.As(err, &pe)
		if pe.Message != http.StatusText(http.StatusBadGateway) {
			t.Errorf("Message = %q, want Bad Gateway", pe.Message)
		}
	})
}

func TestNetworkAndDecodeErrors(t *testing.T) {
	root := errors.New("connection reset")
	netErr := newNetworkError(root)
	if !errors.Is(netErr, core.ErrNetwork) || !errors.Is(netErr, root) {
		t.Errorf("newNetworkError = %v, want ErrNetwork wrapping root", netErr)
	}

	decErr := newDecodeError(errors.New("bad json"), []byte("{broken"))
	if !errors.Is(decErr, core.ErrDecode) {
		t.Errorf("newDecodeError = %v, want ErrDecode", decErr)
	}
	var pe *core.ProviderError
	if !errors.As(decErr, &pe) || pe.Body != "{broken" {
		t.Errorf("expected decode error body to be preserved, got %+v", pe)
	}

	decEmpty := newDecodeError(errors.New("eof"), nil)
	if !errors.Is(decEmpty, core.ErrDecode) {
		t.Errorf("newDecodeError(nil) = %v, want ErrDecode", decEmpty)
	}
}
