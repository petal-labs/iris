package elevenlabs

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/providers/internal/normalize"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

const apiKeyHeader = "xi-api-key"

func (p *ElevenLabs) requireAPIKey() error {
	return normalize.RequireAPIKey(providerID, p.config.APIKey)
}

func (p *ElevenLabs) endpointURL(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return p.config.BaseURL + path
}

// buildHeaders constructs outgoing HTTP headers for an ElevenLabs API request.
// Extra configured headers are copied first, then xi-api-key (and optional
// Content-Type) are set authoritatively so configured auth cannot be silently
// overwritten or duplicated.
func (p *ElevenLabs) buildHeaders(contentType string) http.Header {
	headers := make(http.Header)

	for key, values := range p.config.Headers {
		if strings.EqualFold(key, apiKeyHeader) {
			continue
		}
		for _, v := range values {
			headers.Add(key, v)
		}
	}

	headers.Set(apiKeyHeader, p.config.APIKey.Expose())
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}

	return headers
}

// cancelOnClose wraps an HTTP response body so closing it releases the
// per-request timeout context created by timeoutx.Apply.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// doRequest executes a unary HTTP request against the ElevenLabs API with
// API-key preflight validation, per-provider timeout handling, and typed error
// normalization. Callers own closing the returned response body on success,
// which also releases any provider-applied timeout context.
func (p *ElevenLabs) doRequest(ctx context.Context, method, path, contentType string, body io.Reader) (*http.Response, error) {
	if err := p.requireAPIKey(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	reqCtx, cancel := timeoutx.Apply(ctx, p.config.Timeout)

	req, err := http.NewRequestWithContext(reqCtx, method, p.endpointURL(path), body)
	if err != nil {
		cancel()
		return nil, newNetworkError(err)
	}
	req.Header = p.buildHeaders(contentType)

	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		ctxErr := reqCtx.Err()
		cancel()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if ctxErr != nil {
			return nil, ctxErr
		}
		return nil, newNetworkError(err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		_ = resp.Body.Close()
		cancel()
		requestID := extractRequestID(resp.Header)
		return nil, normalizeError(resp.StatusCode, errBody, requestID, resp.Header)
	}

	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func extractRequestID(headers http.Header) string {
	for _, key := range []string{"request-id", "x-request-id"} {
		if v := strings.TrimSpace(headers.Get(key)); v != "" {
			return v
		}
	}
	return ""
}
