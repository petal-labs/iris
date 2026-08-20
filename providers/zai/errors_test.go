package zai

import (
	"errors"
	"net/http"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestNormalizeErrorNotFound(t *testing.T) {
	err := normalizeError(
		http.StatusNotFound,
		[]byte(`{"error":{"code":"model_not_found","message":"Model not found"}}`),
		"req-123",
	)

	var providerErr *core.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %v (%T), want *core.ProviderError", err, err)
	}
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
