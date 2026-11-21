package elevenlabs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/petal-labs/iris/v2/core"
)

func TestGetAgentSignedURLAndRedaction(t *testing.T) {
	signedSecret := "wss://api.elevenlabs.io/v1/convai/conversation?agent_id=agent_123&token=SUPER_SECRET_SIGNED_TOKEN"
	var seenReq *http.Request

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			seenReq = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"signed_url":"` + signedSecret + `"}`)),
			}, nil
		}),
	}
	p := New("xi_key_signed", WithHTTPClient(client))

	resp, err := p.GetAgentSignedURL(context.Background(), &AgentSignedURLRequest{
		AgentID:               "agent_123",
		IncludeConversationID: true,
		BranchID:              "br_1",
		VersionID:             "ver_2",
		Environment:           "staging",
	})
	if err != nil {
		t.Fatalf("GetAgentSignedURL() err = %v", err)
	}
	if resp.SignedURL.Expose() != signedSecret {
		t.Errorf("SignedURL = %q, want %q", resp.SignedURL.Expose(), signedSecret)
	}
	if seenReq == nil || seenReq.Method != http.MethodGet || seenReq.URL.Path != "/v1/convai/conversation/get-signed-url" {
		t.Fatalf("unexpected request: %+v", seenReq)
	}
	q := seenReq.URL.Query()
	if q.Get("agent_id") != "agent_123" ||
		q.Get("include_conversation_id") != "true" ||
		q.Get("branch_id") != "br_1" ||
		q.Get("version_id") != "ver_2" ||
		q.Get("environment") != "staging" {
		t.Errorf("unexpected query params: %v", q)
	}

	// Privacy: SignedURL must be redacted in String, GoString, and JSON.
	rawJSON, _ := json.Marshal(resp)
	for _, out := range []string{resp.String(), fmt.Sprintf("%#v", resp), string(rawJSON)} {
		if strings.Contains(out, "SUPER_SECRET_SIGNED_TOKEN") {
			t.Errorf("AgentSignedURLResponse leaked signed URL token: %s", out)
		}
		if !strings.Contains(out, "[REDACTED]") {
			t.Errorf("missing [REDACTED] marker: %s", out)
		}
	}

	// Validation errors.
	if _, err := p.GetAgentSignedURL(context.Background(), nil); !errors.Is(err, core.ErrInvalidVoiceSessionConfig) {
		t.Errorf("nil req err = %v, want ErrInvalidVoiceSessionConfig", err)
	}
	if _, err := p.GetAgentSignedURL(context.Background(), &AgentSignedURLRequest{}); !errors.Is(err, core.ErrInvalidVoiceSessionConfig) {
		t.Errorf("empty agent_id err = %v, want ErrInvalidVoiceSessionConfig", err)
	}

	// Empty signed_url body -> decode error.
	pEmpty := New("xi_key", WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"signed_url":""}`)),
			}, nil
		}),
	}))
	if _, err := pEmpty.GetAgentSignedURL(context.Background(), &AgentSignedURLRequest{AgentID: "a1"}); !errors.Is(err, core.ErrDecode) {
		t.Errorf("empty signed_url err = %v, want ErrDecode", err)
	}
}

func TestAgentSessionPrivateAuthEventsAudioToolsAndInterrupt(t *testing.T) {
	signedURL := "wss://api.elevenlabs.io/v1/convai/conversation?agent_id=agent_priv&token=SECRET_PRIV_TOK"
	var signedURLFetched bool
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/v1/convai/conversation/get-signed-url" {
				signedURLFetched = true
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"signed_url":"` + signedURL + `"}`)),
				}, nil
			}
			return nil, errors.New("unexpected HTTP path: " + req.URL.Path)
		}),
	}

	conn := newMockWSConn(16)
	var dialedURL string
	var dialedHeaders http.Header

	p := New("xi_api_key_priv",
		WithHTTPClient(httpClient),
		WithHeader("X-Custom-Trace", "trace-1"),
		withWSDialer(func(_ context.Context, u string, h http.Header) (wsConn, *http.Response, error) {
			dialedURL = u
			dialedHeaders = h.Clone()
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}),
	)

	// Use core.Client.StartVoiceSession to verify full VoiceSessionProvider wiring.
	irisClient := core.NewClient(p)
	sess, err := irisClient.StartVoiceSession(context.Background(), core.VoiceSessionConfig{
		AgentID:      "agent_priv",
		SystemPrompt: "Be helpful",
		VoiceID:      "voice_override",
		Language:     "en",
		Metadata:     map[string]string{"user_id": "u_42"},
	})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}
	defer sess.Close()

	if !signedURLFetched {
		t.Error("expected private agent session to mint signed URL automatically")
	}
	if dialedURL != signedURL {
		t.Errorf("dialedURL = %q, want %q", dialedURL, signedURL)
	}
	if dialedHeaders.Get("xi-api-key") != "" {
		t.Error("xi-api-key should not be sent when dialing a signed URL")
	}
	if dialedHeaders.Get("X-Custom-Trace") != "trace-1" {
		t.Errorf("custom header missing: %v", dialedHeaders)
	}

	// Push server events: initiation metadata, ping, partial, committed, response, correction, audio, tool call, interruption.
	audioPayload := []byte{0x10, 0x20, 0x30, 0x40}
	conn.push(`{"type":"conversation_initiation_metadata","conversation_initiation_metadata_event":{"conversation_id":"conv_abc","agent_output_audio_format":"pcm_24000","user_input_audio_format":"pcm_16000"}}`)
	conn.push(`{"type":"ping","ping_event":{"event_id":77,"ping_ms":12}}`)
	conn.push(`{"type":"tentative_user_transcript","tentative_user_transcription_event":{"user_transcript":"Hel"}}`)
	conn.push(`{"type":"user_transcript","user_transcription_event":{"user_transcript":"Hello agent","event_id":1}}`)
	conn.push(`{"type":"agent_response","agent_response_event":{"agent_response":"Hi there!","event_id":2}}`)
	conn.push(`{"type":"agent_response_correction","agent_response_correction_event":{"original_agent_response":"Hi there!","corrected_agent_response":"Hi there, how can I help?"}}`)
	conn.push(`{"type":"audio","audio_event":{"audio_base_64":"` + base64.StdEncoding.EncodeToString(audioPayload) + `","event_id":3}}`)
	conn.push(`{"type":"client_tool_call","client_tool_call":{"tool_name":"check_status","tool_call_id":"call_9","parameters":{"id":42}}}`)
	conn.push(`{"type":"interruption","interruption_event":{"event_id":4}}`)

	// Exercise client-to-server operations.
	if err := sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2, 3, 4}, Format: core.AudioFormatPCM16000}); err != nil {
		t.Fatalf("SendAudio() err = %v", err)
	}
	if err := sess.SendText(context.Background(), "Text fallback"); err != nil {
		t.Fatalf("SendText() err = %v", err)
	}
	if err := sess.SubmitToolResult(context.Background(), core.ToolResult{
		CallID:  "call_9",
		Content: map[string]string{"status": "ok"},
	}); err != nil {
		t.Fatalf("SubmitToolResult() err = %v", err)
	}
	if err := sess.Interrupt(context.Background()); err != nil {
		t.Fatalf("Interrupt() err = %v", err)
	}

	close(conn.incoming)

	var (
		sawStarted    bool
		sawPartial    bool
		sawCommitted  bool
		sawAgentText  int
		sawAudio      bool
		sawToolCall   bool
		sawInterrupt  bool
		audioCombined bytes.Buffer
		audioFmt      core.AudioFormat
	)
	for evt := range sess.Events() {
		switch evt.Type {
		case core.VoiceEventSessionStarted:
			sawStarted = true
		case core.VoiceEventUserTranscriptPartial:
			sawPartial = true
		case core.VoiceEventUserTranscriptCommitted:
			sawCommitted = true
		case core.VoiceEventAgentTextCommitted:
			sawAgentText++
		case core.VoiceEventAudioOutput:
			sawAudio = true
			audioFmt = evt.Audio.Format
			audioCombined.Write(evt.Audio.Data)
		case core.VoiceEventToolCall:
			sawToolCall = true
			if evt.ToolCall.ID != "call_9" || evt.ToolCall.Name != "check_status" {
				t.Errorf("unexpected tool call: %+v", evt.ToolCall)
			}
		case core.VoiceEventTurnInterrupted:
			sawInterrupt = true
		}
	}
	if err := <-sess.Err(); err != nil {
		t.Fatalf("unexpected Err(): %v", err)
	}

	if sess.ID() != "conv_abc" {
		t.Errorf("sess.ID() = %q, want conv_abc", sess.ID())
	}
	if !sawStarted || !sawPartial || !sawCommitted || sawAgentText != 2 || !sawAudio || !sawToolCall || !sawInterrupt {
		t.Errorf("missing events: start=%t partial=%t committed=%t agentText=%d audio=%t tool=%t interrupt=%t",
			sawStarted, sawPartial, sawCommitted, sawAgentText, sawAudio, sawToolCall, sawInterrupt)
	}
	if !bytes.Equal(audioCombined.Bytes(), audioPayload) || audioFmt.SampleRate != 24000 {
		t.Errorf("audio mismatch: bytes=%x fmt=%v", audioCombined.Bytes(), audioFmt)
	}

	// Verify wire messages sent by client: init, audio, text, tool_result, user_activity, and pong.
	var sawInit, sawPong, sawAudioChunk, sawUserMsg, sawToolRes, sawActivity bool
	for _, raw := range conn.sentMessages() {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal sent: %v", err)
		}
		if _, ok := m["user_audio_chunk"]; ok {
			sawAudioChunk = true
			continue
		}
		switch m["type"] {
		case "conversation_initiation_client_data":
			sawInit = true
		case "pong":
			if id, _ := m["event_id"].(float64); int64(id) == 77 {
				sawPong = true
			}
		case "user_message":
			sawUserMsg = true
		case "client_tool_result":
			if m["tool_call_id"] == "call_9" {
				sawToolRes = true
			}
		case "user_activity":
			sawActivity = true
		}
	}
	if !sawInit || !sawPong || !sawAudioChunk || !sawUserMsg || !sawToolRes || !sawActivity {
		t.Errorf("missing client frames: init=%t pong=%t audio=%t user_msg=%t tool_res=%t activity=%t",
			sawInit, sawPong, sawAudioChunk, sawUserMsg, sawToolRes, sawActivity)
	}
}

func TestAgentSessionAuthModesValidationAndPrivacy(t *testing.T) {
	t.Run("config privacy redacts secrets", func(t *testing.T) {
		cfg := AgentSessionConfig{
			AgentID:          "agent_1",
			SignedURL:        core.NewSecret("wss://api.elevenlabs.io/v1/convai/conversation?token=SECRET_TOK"),
			SystemPrompt:     "SECRET_PROMPT_CONTENT",
			FirstMessage:     "SECRET_FIRST_MSG",
			DynamicVariables: map[string]any{"ssn": "SECRET_DYN_VAR"},
		}
		for _, out := range []string{cfg.String(), fmt.Sprintf("%#v", cfg)} {
			for _, sec := range []string{"SECRET_TOK", "SECRET_PROMPT_CONTENT", "SECRET_FIRST_MSG", "SECRET_DYN_VAR"} {
				if strings.Contains(out, sec) {
					t.Errorf("AgentSessionConfig leaked %q: %s", sec, out)
				}
			}
		}
	})

	t.Run("pre-minted signed URL works without API key", func(t *testing.T) {
		conn := newMockWSConn(2)
		var dialedURL string
		p := New("", withWSDialer(func(_ context.Context, u string, _ http.Header) (wsConn, *http.Response, error) {
			dialedURL = u
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		target := "wss://api.elevenlabs.io/v1/convai/conversation?agent_id=a1&token=tok_xyz"
		sess, err := p.StartAgentSession(context.Background(), AgentSessionConfig{
			SignedURL: core.NewSecret(target),
		})
		if err != nil {
			t.Fatalf("StartAgentSession(SignedURL) err = %v", err)
		}
		if dialedURL != target {
			t.Errorf("dialedURL = %q, want %q", dialedURL, target)
		}
		close(conn.incoming)
		_ = sess.Close()
	})

	t.Run("direct auth dials convai endpoint with xi-api-key", func(t *testing.T) {
		conn := newMockWSConn(2)
		var dialedURL string
		var gotKey string
		p := New("xi_direct_key", withWSDialer(func(_ context.Context, u string, h http.Header) (wsConn, *http.Response, error) {
			dialedURL = u
			gotKey = h.Get("xi-api-key")
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		sess, err := p.StartAgentSession(context.Background(), AgentSessionConfig{
			AgentID:       "agent_direct",
			UseDirectAuth: true,
		})
		if err != nil {
			t.Fatalf("StartAgentSession(UseDirectAuth) err = %v", err)
		}
		parsed, _ := url.Parse(dialedURL)
		if parsed.Path != "/v1/convai/conversation" || parsed.Query().Get("agent_id") != "agent_direct" || gotKey != "xi_direct_key" {
			t.Errorf("unexpected direct dial: url=%q key=%q", dialedURL, gotKey)
		}
		close(conn.incoming)
		_ = sess.Close()
	})

	t.Run("validation errors", func(t *testing.T) {
		p := New("xi_key")
		//nolint:staticcheck // intentionally testing nil context guard
		if _, err := p.StartAgentSession(nil, AgentSessionConfig{AgentID: "a1"}); err == nil {
			t.Error("expected error for nil ctx")
		}
		badCfgs := []AgentSessionConfig{
			{},
			{SignedURL: core.NewSecret("https://not-ws.example.com")},
			{AgentID: "a1", InputFormat: core.AudioFormat{Encoding: "bad"}},
			{AgentID: "a1", OutputFormat: core.AudioFormat{Encoding: "bad"}},
			{AgentID: "a1", EventBufferSize: -1},
			{AgentID: "a1", MaxAudioChunkBytes: -1},
		}
		for i, bc := range badCfgs {
			if _, err := p.StartAgentSession(context.Background(), bc); !errors.Is(err, core.ErrInvalidVoiceSessionConfig) {
				t.Errorf("badCfgs[%d] err = %v, want ErrInvalidVoiceSessionConfig", i, err)
			}
		}
	})

	t.Run("SendAudio chunk guards and closed lifecycle", func(t *testing.T) {
		conn := newMockWSConn(2)
		p := New("xi_key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		sess, err := p.StartAgentSession(context.Background(), AgentSessionConfig{
			AgentID:            "a1",
			UseDirectAuth:      true,
			MaxAudioChunkBytes: 4,
		})
		if err != nil {
			t.Fatalf("StartAgentSession() err = %v", err)
		}

		// Oversized chunk rejected.
		if err := sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2, 3, 4, 5, 6}}); !errors.Is(err, core.ErrInvalidAudioChunk) {
			t.Errorf("oversized err = %v, want ErrInvalidAudioChunk", err)
		}
		// Odd-byte PCM chunk rejected.
		if err := sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2, 3}}); !errors.Is(err, core.ErrInvalidAudioChunk) {
			t.Errorf("odd PCM err = %v, want ErrInvalidAudioChunk", err)
		}
		// Blank SendText rejected.
		if err := sess.SendText(context.Background(), "  "); !errors.Is(err, core.ErrInvalidSpeechRequest) {
			t.Errorf("blank text err = %v, want ErrInvalidSpeechRequest", err)
		}
		// Blank CallID rejected.
		if err := sess.SubmitToolResult(context.Background(), core.ToolResult{}); !errors.Is(err, core.ErrInvalidVoiceSessionConfig) {
			t.Errorf("blank call_id err = %v, want ErrInvalidVoiceSessionConfig", err)
		}

		if err := sess.Close(); err != nil {
			t.Fatalf("Close() err = %v", err)
		}
		if err := sess.Close(); err != nil {
			t.Fatalf("second Close() err = %v", err)
		}
		if err := sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2}}); !errors.Is(err, core.ErrVoiceSessionClosed) {
			t.Errorf("SendAudio after Close = %v, want ErrVoiceSessionClosed", err)
		}
		if err := sess.SendText(context.Background(), "hi"); !errors.Is(err, core.ErrVoiceSessionClosed) {
			t.Errorf("SendText after Close = %v, want ErrVoiceSessionClosed", err)
		}
		if err := sess.SubmitToolResult(context.Background(), core.ToolResult{CallID: "c1"}); !errors.Is(err, core.ErrVoiceSessionClosed) {
			t.Errorf("SubmitToolResult after Close = %v, want ErrVoiceSessionClosed", err)
		}
		if err := sess.Interrupt(context.Background()); !errors.Is(err, core.ErrVoiceSessionClosed) {
			t.Errorf("Interrupt after Close = %v, want ErrVoiceSessionClosed", err)
		}
	})
}

func TestAgentSessionErrorsCancellationAndRace(t *testing.T) {
	apiSecret := "xi_secret_agent_key"
	signedSecret := "wss://api.elevenlabs.io/v1/convai/conversation?agent_id=a1&token=SECRET_URL_TOK"

	t.Run("server error frame scrubs API key and signed URL", func(t *testing.T) {
		conn := newMockWSConn(2)
		p := New(apiSecret, withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		sess, err := p.StartAgentSession(context.Background(), AgentSessionConfig{
			SignedURL: core.NewSecret(signedSecret),
		})
		if err != nil {
			t.Fatalf("StartAgentSession() err = %v", err)
		}
		defer sess.Close()

		conn.push(`{"type":"error","code":401,"error":"unauthorized","message":"denied for ` + apiSecret + ` at ` + signedSecret + `"}`)
		for range sess.Events() {
		}
		gotErr := <-sess.Err()
		if !errors.Is(gotErr, core.ErrUnauthorized) {
			t.Fatalf("err = %v, want ErrUnauthorized", gotErr)
		}
		if strings.Contains(gotErr.Error(), apiSecret) || strings.Contains(gotErr.Error(), "SECRET_URL_TOK") {
			t.Errorf("error leaked secret: %v", gotErr)
		}
	})

	t.Run("context cancel and Close unblock full bounded buffer", func(t *testing.T) {
		conn := newMockWSConn(4)
		p := New("xi_key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		sess, err := p.StartAgentSession(ctx, AgentSessionConfig{
			AgentID:         "a1",
			UseDirectAuth:   true,
			EventBufferSize: 1,
		})
		if err != nil {
			t.Fatalf("StartAgentSession() err = %v", err)
		}
		defer sess.Close()

		conn.push(`{"type":"conversation_initiation_metadata","conversation_initiation_metadata_event":{"conversation_id":"c1"}}`)
		conn.push(`{"type":"agent_response","agent_response_event":{"agent_response":"blocks on buffer"}}`)
		time.Sleep(15 * time.Millisecond)

		cancel()
		select {
		case gotErr := <-sess.Err():
			if !errors.Is(gotErr, context.Canceled) {
				t.Errorf("Err() = %v, want context.Canceled", gotErr)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for cancel error")
		}
	})

	t.Run("concurrent SendAudio SendText Interrupt and Close", func(t *testing.T) {
		conn := newMockWSConn(16)
		p := New("xi_key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		sess, err := p.StartAgentSession(context.Background(), AgentSessionConfig{
			AgentID:       "a1",
			UseDirectAuth: true,
		})
		if err != nil {
			t.Fatalf("StartAgentSession() err = %v", err)
		}

		var wg sync.WaitGroup
		for range 6 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					_ = sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2}})
					_ = sess.SendText(context.Background(), "hello")
					_ = sess.Interrupt(context.Background())
				}
			}()
		}
		time.Sleep(5 * time.Millisecond)
		_ = sess.Close()
		wg.Wait()
	})
}
