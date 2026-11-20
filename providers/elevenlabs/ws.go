package elevenlabs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

const maxWSMessageBytes int64 = 4 * 1024 * 1024 // 4 MiB per WebSocket message

// wsConn is an internal WebSocket connection abstraction so real-time protocol
// implementations can be unit tested deterministically without TCP sockets.
type wsConn interface {
	ReadText(ctx context.Context) ([]byte, error)
	WriteText(ctx context.Context, payload []byte) error
	Close(status websocket.StatusCode, reason string) error
}

type wsDialer func(ctx context.Context, urlStr string, headers http.Header) (wsConn, *http.Response, error)

type coderWSConn struct {
	conn *websocket.Conn
}

func (c *coderWSConn) ReadText(ctx context.Context) ([]byte, error) {
	_, data, err := c.conn.Read(ctx)
	return data, err
}

func (c *coderWSConn) WriteText(ctx context.Context, payload []byte) error {
	return c.conn.Write(ctx, websocket.MessageText, payload)
}

func (c *coderWSConn) Close(status websocket.StatusCode, reason string) error {
	return c.conn.Close(status, reason)
}

func (p *ElevenLabs) defaultWSDialer(ctx context.Context, urlStr string, headers http.Header) (wsConn, *http.Response, error) {
	conn, resp, err := websocket.Dial(ctx, urlStr, &websocket.DialOptions{
		HTTPHeader: headers,
		HTTPClient: p.config.HTTPClient,
	})
	if err != nil {
		return nil, resp, err
	}
	conn.SetReadLimit(maxWSMessageBytes)
	return &coderWSConn{conn: conn}, resp, nil
}

func (p *ElevenLabs) wsEndpointURL(path string) (string, error) {
	u, err := url.Parse(p.endpointURL(path))
	if err != nil {
		return "", fmt.Errorf("parse websocket endpoint URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "wss", "ws":
	default:
		return "", fmt.Errorf("unsupported websocket URL scheme %q", u.Scheme)
	}
	return u.String(), nil
}

func (p *ElevenLabs) dialWebSocket(ctx context.Context, path string) (wsConn, error) {
	if err := p.requireAPIKey(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	wsURL, err := p.wsEndpointURL(path)
	if err != nil {
		return nil, redactSecretError(newNetworkError(err), p.config.APIKey)
	}

	dial := p.config.wsDialer
	if dial == nil {
		dial = p.defaultWSDialer
	}

	conn, resp, err := dial(ctx, wsURL, p.buildHeaders(""))
	if err != nil {
		if resp != nil {
			var errBody []byte
			if resp.Body != nil {
				errBody, _ = io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
				_ = resp.Body.Close()
			}
			if resp.StatusCode >= 400 {
				requestID := extractRequestID(resp.Header)
				return nil, redactSecretError(normalizeError(resp.StatusCode, errBody, requestID, resp.Header), p.config.APIKey)
			}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, redactSecretError(newNetworkError(err), p.config.APIKey)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return conn, nil
}

func isNormalWSClose(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	status := websocket.CloseStatus(err)
	return status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway
}
