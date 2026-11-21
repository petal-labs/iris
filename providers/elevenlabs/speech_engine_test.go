package elevenlabs

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func signSpeechEngineTestJWT(t *testing.T, alg string, claims map[string]any, secret []byte) string {
	t.Helper()
	hdrJSON, err := json.Marshal(map[string]string{"alg": alg, "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal jwt header: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal jwt payload: %v", err)
	}
	h := base64.RawURLEncoding.EncodeToString(hdrJSON)
	p := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := h + "." + p
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig
}

type speechEngineFakeLLM struct {
	id        string
	streaming bool
	chatFn    func(context.Context, *core.ChatRequest) (*core.ChatResponse, error)
	streamFn  func(context.Context, *core.ChatRequest) (*core.ChatStream, error)
}

func (p *speechEngineFakeLLM) ID() string               { return p.id }
func (p *speechEngineFakeLLM) Models() []core.ModelInfo { return nil }
func (p *speechEngineFakeLLM) Supports(f core.Feature) bool {
	if f == core.FeatureChat {
		return true
	}
	return f == core.FeatureChatStreaming && p.streaming
}

func (p *speechEngineFakeLLM) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if p.chatFn != nil {
		return p.chatFn(ctx, req)
	}
	return &core.ChatResponse{Output: "sync answer"}, nil
}

func (p *speechEngineFakeLLM) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	if p.streamFn != nil {
		return p.streamFn(ctx, req)
	}
	return nil, core.ErrNotSupported
}

type speechEngineTestTool struct {
	name string
	fn   func(context.Context, json.RawMessage) (any, error)
}

func (t speechEngineTestTool) Name() string            { return t.name }
func (t speechEngineTestTool) Description() string     { return "test tool" }
func (t speechEngineTestTool) Schema() core.ToolSchema { return core.ToolSchema{} }
func (t speechEngineTestTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	return t.fn(ctx, args)
}

func TestVerifySpeechEngineJWT(t *testing.T) {
	apiKey := "xi_super_secret_speech_engine_key"
	sum := sha256.Sum256([]byte(apiKey))
	hexSecret := []byte(hex.EncodeToString(sum[:]))
	now := time.Unix(1700000000, 0)

	validClaims := func(expOffset int64) map[string]any {
		return map[string]any{
			"iss": SpeechEngineJWTIssuer,
			"sub": SpeechEngineJWTSubject,
			"exp": now.Unix() + expOffset,
		}
	}

	t.Run("valid hex sha256 secret", func(t *testing.T) {
		tok := signSpeechEngineTestJWT(t, "HS256", validClaims(30), hexSecret)
		claims, err := verifySpeechEngineJWTAt(apiKey, tok, 60*time.Second, now)
		if err != nil {
			t.Fatalf("verify err = %v", err)
		}
		if claims.Issuer != SpeechEngineJWTIssuer || claims.Subject != SpeechEngineJWTSubject {
			t.Errorf("unexpected claims: %+v", claims)
		}
	})

	t.Run("valid raw 32-byte sha256 secret with Bearer prefix", func(t *testing.T) {
		tok := signSpeechEngineTestJWT(t, "HS256", validClaims(30), sum[:])
		if _, err := verifySpeechEngineJWTAt(apiKey, "Bearer "+tok, 0, now); err != nil {
			t.Fatalf("verify raw digest err = %v", err)
		}
	})

	t.Run("valid within 60s clock skew leeway", func(t *testing.T) {
		tok := signSpeechEngineTestJWT(t, "HS256", validClaims(-30), hexSecret)
		if _, err := verifySpeechEngineJWTAt(apiKey, tok, 60*time.Second, now); err != nil {
			t.Fatalf("expected token within leeway to succeed, got %v", err)
		}
	})

	badCases := []struct {
		name  string
		key   string
		token string
	}{
		{"blank api key", "", signSpeechEngineTestJWT(t, "HS256", validClaims(30), hexSecret)},
		{"missing token", apiKey, ""},
		{"malformed segments", apiKey, "not.a.valid.jwt"},
		{"wrong alg none", apiKey, signSpeechEngineTestJWT(t, "none", validClaims(30), hexSecret)},
		{"wrong alg HS512", apiKey, signSpeechEngineTestJWT(t, "HS512", validClaims(30), hexSecret)},
		{"wrong signing key", apiKey, signSpeechEngineTestJWT(t, "HS256", validClaims(30), []byte("wrong-secret"))},
		{"wrong issuer", apiKey, signSpeechEngineTestJWT(t, "HS256", map[string]any{"iss": "https://evil.test", "sub": SpeechEngineJWTSubject, "exp": now.Unix() + 30}, hexSecret)},
		{"wrong subject", apiKey, signSpeechEngineTestJWT(t, "HS256", map[string]any{"iss": SpeechEngineJWTIssuer, "sub": "other", "exp": now.Unix() + 30}, hexSecret)},
		{"missing exp", apiKey, signSpeechEngineTestJWT(t, "HS256", map[string]any{"iss": SpeechEngineJWTIssuer, "sub": SpeechEngineJWTSubject}, hexSecret)},
		{"expired past leeway", apiKey, signSpeechEngineTestJWT(t, "HS256", validClaims(-61), hexSecret)},
		{"nbf in future past leeway", apiKey, signSpeechEngineTestJWT(t, "HS256", map[string]any{"iss": SpeechEngineJWTIssuer, "sub": SpeechEngineJWTSubject, "exp": now.Unix() + 300, "nbf": now.Unix() + 120}, hexSecret)},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := verifySpeechEngineJWTAt(tc.key, tc.token, 60*time.Second, now)
			if !errors.Is(err, ErrInvalidSpeechEngineToken) {
				t.Fatalf("err = %v, want ErrInvalidSpeechEngineToken", err)
			}
			if strings.Contains(err.Error(), apiKey) {
				t.Errorf("error leaked API key: %v", err)
			}
		})
	}
}

func TestSpeechEngineConfigValidationAndAuthGate(t *testing.T) {
	llm := &speechEngineFakeLLM{id: "openai"}
	cfg := SpeechEngineConfig{
		LLM:          llm,
		Model:        "gpt-5",
		SystemPrompt: "SECRET_SPEECH_ENGINE_PROMPT",
	}
	for _, out := range []string{cfg.String(), fmt.Sprintf("%#v", cfg)} {
		if strings.Contains(out, "SECRET_SPEECH_ENGINE_PROMPT") {
			t.Errorf("config leaked SystemPrompt: %s", out)
		}
	}

	p := New("xi_valid_key")
	var nilP *ElevenLabs
	if _, err := nilP.NewSpeechEngineHandler(cfg); !errors.Is(err, ErrInvalidSpeechEngineConfig) {
		t.Errorf("nil provider err = %v, want ErrInvalidSpeechEngineConfig", err)
	}
	if _, err := New("").NewSpeechEngineHandler(cfg); !errors.Is(err, ErrInvalidSpeechEngineConfig) {
		t.Errorf("empty api key with auth enabled err = %v, want ErrInvalidSpeechEngineConfig", err)
	}
	if _, err := New("").NewSpeechEngineHandler(SpeechEngineConfig{LLM: llm, Model: "gpt-5", DisableAuth: true}); err != nil {
		t.Errorf("empty api key with DisableAuth=true unexpected err = %v", err)
	}

	badConfigs := []SpeechEngineConfig{
		{},
		{LLM: llm},
		{LLM: llm, Model: "gpt-5", MaxToolRounds: -1},
		{LLM: llm, Model: "gpt-5", ClockSkew: -time.Second},
		{LLM: llm, Model: "gpt-5", Tools: []core.Tool{nil}},
		{LLM: llm, Model: "gpt-5", Tools: []core.Tool{speechEngineTestTool{name: "  "}}},
	}
	for i, bc := range badConfigs {
		if _, err := p.NewSpeechEngineHandler(bc); !errors.Is(err, ErrInvalidSpeechEngineConfig) {
			t.Errorf("badConfigs[%d] err = %v, want ErrInvalidSpeechEngineConfig", i, err)
		}
	}

	// ServeHTTP rejects missing/invalid JWT with 401 before upgrade.
	h, err := p.NewSpeechEngineHandler(cfg)
	if err != nil {
		t.Fatalf("NewSpeechEngineHandler() err = %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/speech-engine", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing auth status = %d, want 401", rec.Code)
	}
}

func TestSpeechEngineServeConnStreamingPingAndClose(t *testing.T) {
	conn := newMockWSConn(8)

	var seenMsgs []core.Message
	llm := &speechEngineFakeLLM{
		id:        "openai",
		streaming: true,
		streamFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
			seenMsgs = append([]core.Message(nil), req.Messages...)
			ch := make(chan core.ChatChunk, 3)
			errCh := make(chan error, 1)
			finalCh := make(chan *core.ChatResponse, 1)
			ch <- core.ChatChunk{Delta: "Your balance "}
			ch <- core.ChatChunk{Delta: "is $120."}
			close(ch)
			finalCh <- &core.ChatResponse{Output: "Your balance is $120."}
			close(finalCh)
			close(errCh)
			return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
		},
	}

	var (
		evMu   sync.Mutex
		events []core.VoiceEvent
	)
	p := New("xi_test_key")
	h, err := p.NewSpeechEngineHandler(SpeechEngineConfig{
		LLM:          llm,
		Model:        "gpt-5",
		SystemPrompt: "Be concise.",
		OnEvent: func(e core.VoiceEvent) {
			evMu.Lock()
			events = append(events, e)
			evMu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewSpeechEngineHandler() err = %v", err)
	}

	conn.push(`{"type":"init","conversation_id":"conv_999"}`)
	conn.push(`{"type":"ping"}`)
	conn.push(`{"type":"user_transcript","event_id":42,"user_transcript":[{"role":"user","content":"What is my balance?"}]}`)
	// Duplicate/stale event_id should be ignored.
	conn.push(`{"type":"user_transcript","event_id":42,"user_transcript":[{"role":"user","content":"duplicate"}]}`)

	done := make(chan error, 1)
	go func() {
		done <- h.serveConn(context.Background(), conn)
	}()

	// Wait until final agent_response is sent, then send close.
	deadline := time.After(2 * time.Second)
	for {
		sent := conn.sentMessages()
		foundFinal := false
		for _, raw := range sent {
			var ar speechEngineAgentResponse
			if json.Unmarshal(raw, &ar) == nil && ar.Type == "agent_response" && ar.IsFinal {
				foundFinal = true
				break
			}
		}
		if foundFinal {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for final agent_response; sent=%d", len(sent))
		case <-time.After(5 * time.Millisecond):
		}
	}

	conn.push(`{"type":"close"}`)
	if err := <-done; err != nil {
		t.Fatalf("serveConn() err = %v", err)
	}

	// Verify SystemPrompt + history passed to LLM.
	if len(seenMsgs) != 2 || seenMsgs[0].Role != core.RoleSystem || seenMsgs[1].Content != "What is my balance?" {
		t.Errorf("unexpected LLM messages: %+v", seenMsgs)
	}

	// Verify wire messages include pong, incremental agent_response chunks, and final is_final=true.
	var sawPong bool
	var streamed strings.Builder
	var sawFinal bool
	for _, raw := range conn.sentMessages() {
		var frame map[string]any
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("unmarshal sent frame: %v", err)
		}
		switch frame["type"] {
		case "pong":
			sawPong = true
		case "agent_response":
			var ar speechEngineAgentResponse
			_ = json.Unmarshal(raw, &ar)
			if ar.EventID != 42 {
				t.Errorf("agent_response event_id = %d, want 42", ar.EventID)
			}
			streamed.WriteString(ar.Content)
			if ar.IsFinal {
				sawFinal = true
			}
		}
	}
	if !sawPong || !sawFinal || streamed.String() != "Your balance is $120." {
		t.Errorf("wire mismatch: pong=%t final=%t content=%q", sawPong, sawFinal, streamed.String())
	}

	evMu.Lock()
	defer evMu.Unlock()
	var sawStart, sawCommitted, sawTurnDone, sawEnd bool
	for _, e := range events {
		if e.SessionID != "conv_999" {
			t.Errorf("event %s missing conv_999 session_id", e.Type)
		}
		switch e.Type {
		case core.VoiceEventSessionStarted:
			sawStart = true
		case core.VoiceEventUserTranscriptCommitted:
			sawCommitted = true
		case core.VoiceEventTurnCompleted:
			sawTurnDone = true
		case core.VoiceEventSessionEnded:
			sawEnd = true
		}
	}
	if !sawStart || !sawCommitted || !sawTurnDone || !sawEnd {
		t.Errorf("missing lifecycle events: start=%t committed=%t turnDone=%t end=%t", sawStart, sawCommitted, sawTurnDone, sawEnd)
	}
}

func TestSpeechEngineInterruptionHigherEventIDAndTools(t *testing.T) {
	t.Run("higher event_id cancels in-flight turn immediately", func(t *testing.T) {
		conn := newMockWSConn(8)
		turn1Started := make(chan struct{}, 1)
		turn1Cancelled := make(chan struct{}, 1)

		var calls atomic.Int32
		llm := &speechEngineFakeLLM{
			id:        "openai",
			streaming: true,
			streamFn: func(ctx context.Context, _ *core.ChatRequest) (*core.ChatStream, error) {
				n := calls.Add(1)
				ch := make(chan core.ChatChunk, 2)
				errCh := make(chan error, 1)
				finalCh := make(chan *core.ChatResponse, 1)
				go func() {
					defer close(ch)
					defer close(errCh)
					defer close(finalCh)
					if n == 1 {
						ch <- core.ChatChunk{Delta: "Stale start "}
						turn1Started <- struct{}{}
						<-ctx.Done()
						turn1Cancelled <- struct{}{}
						errCh <- ctx.Err()
						return
					}
					ch <- core.ChatChunk{Delta: "New turn reply."}
					finalCh <- &core.ChatResponse{Output: "New turn reply."}
				}()
				return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
			},
		}

		var intrCount atomic.Int32
		h, err := New("xi_key").NewSpeechEngineHandler(SpeechEngineConfig{
			LLM:   llm,
			Model: "gpt-5",
			OnEvent: func(e core.VoiceEvent) {
				if e.Type == core.VoiceEventTurnInterrupted {
					intrCount.Add(1)
				}
			},
		})
		if err != nil {
			t.Fatalf("NewSpeechEngineHandler() err = %v", err)
		}

		done := make(chan error, 1)
		go func() { done <- h.serveConn(context.Background(), conn) }()

		conn.push(`{"type":"user_transcript","event_id":10,"user_transcript":"first"}`)
		<-turn1Started

		// Send higher event_id to interrupt turn 10.
		conn.push(`{"type":"user_transcript","event_id":11,"user_transcript":"second"}`)

		select {
		case <-turn1Cancelled:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for turn 10 cancellation on higher event_id")
		}

		// Wait for event_id 11 final response.
		deadline := time.After(2 * time.Second)
		for {
			found := false
			for _, raw := range conn.sentMessages() {
				var ar speechEngineAgentResponse
				if json.Unmarshal(raw, &ar) == nil && ar.EventID == 11 && ar.IsFinal {
					found = true
				}
			}
			if found {
				break
			}
			select {
			case <-deadline:
				t.Fatal("timed out waiting for event_id 11 completion")
			case <-time.After(5 * time.Millisecond):
			}
		}

		conn.push(`{"type":"close"}`)
		if err := <-done; err != nil {
			t.Fatalf("serveConn err = %v", err)
		}
		if intrCount.Load() < 1 {
			t.Error("expected VoiceEventTurnInterrupted to be emitted")
		}
	})

	t.Run("non-streaming LLM with executable tools", func(t *testing.T) {
		conn := newMockWSConn(4)
		var round atomic.Int32
		llm := &speechEngineFakeLLM{
			id:        "anthropic",
			streaming: false,
			chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
				if round.Add(1) == 1 {
					return &core.ChatResponse{
						ToolCalls: []core.ToolCall{
							{ID: "tc-1", Name: "lookup_time", Arguments: json.RawMessage(`{}`)},
						},
					}, nil
				}
				return &core.ChatResponse{Output: "It is 3 PM."}, nil
			},
		}

		tool := speechEngineTestTool{
			name: "lookup_time",
			fn:   func(_ context.Context, _ json.RawMessage) (any, error) { return "3 PM", nil },
		}

		h, err := New("xi_key").NewSpeechEngineHandler(SpeechEngineConfig{
			LLM:   llm,
			Model: "claude-sonnet",
			Tools: []core.Tool{tool},
		})
		if err != nil {
			t.Fatalf("NewSpeechEngineHandler() err = %v", err)
		}

		done := make(chan error, 1)
		go func() { done <- h.serveConn(context.Background(), conn) }()

		conn.push(`{"type":"user_transcript","event_id":1,"user_transcript":"What time is it?"}`)

		deadline := time.After(2 * time.Second)
		for {
			found := false
			for _, raw := range conn.sentMessages() {
				var ar speechEngineAgentResponse
				if json.Unmarshal(raw, &ar) == nil && ar.IsFinal && ar.Content == "It is 3 PM." {
					found = true
				}
			}
			if found {
				break
			}
			select {
			case <-deadline:
				t.Fatal("timed out waiting for tool-assisted agent_response")
			case <-time.After(5 * time.Millisecond):
			}
		}

		conn.push(`{"type":"close"}`)
		if err := <-done; err != nil {
			t.Fatalf("serveConn err = %v", err)
		}
	})
}

func TestSpeechEngineErrorsAndSecretRedaction(t *testing.T) {
	secret := "xi_secret_speech_engine_err_key"
	var gotErr error
	var mu sync.Mutex

	h, err := New(secret).NewSpeechEngineHandler(SpeechEngineConfig{
		LLM:   &speechEngineFakeLLM{id: "openai"},
		Model: "gpt-5",
		OnEvent: func(e core.VoiceEvent) {
			if e.Type == core.VoiceEventError && e.Err != nil {
				mu.Lock()
				gotErr = e.Err
				mu.Unlock()
			}
		},
	})
	if err != nil {
		t.Fatalf("NewSpeechEngineHandler() err = %v", err)
	}

	// Server error frame containing secret must scrub API key.
	conn := newMockWSConn(4)
	conn.push(`{"type":"error","message":"upstream failed for ` + secret + `"}`)
	conn.push(`{"type":"close"}`)
	if err := h.serveConn(context.Background(), conn); err != nil {
		t.Fatalf("serveConn() err = %v", err)
	}
	mu.Lock()
	errSnapshot := gotErr
	mu.Unlock()
	if !errors.Is(errSnapshot, core.ErrServer) {
		t.Fatalf("server error frame = %v, want core.ErrServer", errSnapshot)
	}
	if strings.Contains(errSnapshot.Error(), secret) {
		t.Errorf("error leaked secret: %v", errSnapshot)
	}

	// Malformed JSON frame returns decode error.
	conn2 := newMockWSConn(2)
	conn2.push(`{broken-json`)
	if err := h.serveConn(context.Background(), conn2); !errors.Is(err, core.ErrDecode) {
		t.Errorf("malformed JSON err = %v, want core.ErrDecode", err)
	}

	// Unexpected network error scrubs secret.
	conn3 := newMockWSConn(1)
	conn3.readErr = errors.New("tcp reset " + secret)
	close(conn3.incoming)
	err3 := h.serveConn(context.Background(), conn3)
	if !errors.Is(err3, core.ErrNetwork) {
		t.Errorf("network drop err = %v, want core.ErrNetwork", err3)
	}
	if strings.Contains(err3.Error(), secret) {
		t.Errorf("network error leaked secret: %v", err3)
	}
}
