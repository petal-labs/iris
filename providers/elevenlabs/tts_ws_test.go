package elevenlabs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/petal-labs/iris/v2/core"
)

func TestRealtimeTTSSuccessMultiTurnFlushAndFinish(t *testing.T) {
	conn := newMockWSConn(8)
	var dialedURL string
	var dialedHeaders http.Header

	p := New("xi_ws_tts_secret",
		WithBaseURL("https://api.elevenlabs.io"),
		withWSDialer(func(_ context.Context, u string, h http.Header) (wsConn, *http.Response, error) {
			dialedURL = u
			dialedHeaders = h.Clone()
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}),
	)

	stab := 0.55
	sim := 0.8
	sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{
		Model:               ModelElevenMultilingualV2,
		VoiceID:             "voice_abc",
		Format:              core.AudioFormatPCM16000,
		Language:            "en",
		VoiceSettings:       &core.VoiceSettings{Stability: &stab, SimilarityBoost: &sim},
		ChunkLengthSchedule: []int{120, 160, 250, 290},
		AutoMode:            true,
		InactivityTimeout:   45 * time.Second,
		SyncAlignment:       true,
		BufferSize:          8,
	})
	if err != nil {
		t.Fatalf("StartRealtimeSynthesis() err = %v", err)
	}
	defer sess.Close()

	// Verify dialed WebSocket URL and headers.
	parsed, err := url.Parse(dialedURL)
	if err != nil {
		t.Fatalf("parse dialedURL: %v", err)
	}
	if parsed.Scheme != "wss" || parsed.Host != "api.elevenlabs.io" || parsed.Path != "/v1/text-to-speech/voice_abc/stream-input" {
		t.Errorf("dialedURL = %q", dialedURL)
	}
	q := parsed.Query()
	if q.Get("model_id") != string(ModelElevenMultilingualV2) ||
		q.Get("output_format") != "pcm_16000" ||
		q.Get("language_code") != "en" ||
		q.Get("inactivity_timeout") != "45" ||
		q.Get("sync_alignment") != "true" ||
		q.Get("auto_mode") != "true" {
		t.Errorf("unexpected query params: %v", q)
	}
	if dialedHeaders.Get("xi-api-key") != "xi_ws_tts_secret" {
		t.Errorf("xi-api-key header = %q", dialedHeaders.Get("xi-api-key"))
	}

	// Turn 1: stream fragments and flush (connection stays open).
	if err := sess.SendText(context.Background(), "Hello "); err != nil {
		t.Fatalf("SendText(1) err = %v", err)
	}
	if err := sess.SendText(context.Background(), "world."); err != nil {
		t.Fatalf("SendText(2) err = %v", err)
	}
	if err := sess.Flush(context.Background()); err != nil {
		t.Fatalf("Flush() err = %v", err)
	}

	// Simulate server sending turn 1 chunks + empty keepalive + isFinal marker.
	chunk1 := []byte("pcm-turn1-a")
	chunk2 := []byte("pcm-turn1-b")
	chunk3 := []byte("pcm-turn2-final")
	conn.push(`{"audio":""}`) // ignored empty non-final frame
	conn.push(`{"audio":"` + base64.StdEncoding.EncodeToString(chunk1) + `","isFinal":false}`)
	conn.push(`{"audio":"` + base64.StdEncoding.EncodeToString(chunk2) + `","isFinal":true}`)

	// Turn 2: send another fragment and Finish (EOS).
	if err := sess.SendText(context.Background(), "Second turn."); err != nil {
		t.Fatalf("SendText(turn2) err = %v", err)
	}
	if err := sess.Finish(context.Background()); err != nil {
		t.Fatalf("Finish() err = %v", err)
	}

	// Post-Finish mutations must return ErrRealtimeSessionClosed.
	if err := sess.SendText(context.Background(), "late"); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("SendText after Finish err = %v, want ErrRealtimeSessionClosed", err)
	}
	if err := sess.Flush(context.Background()); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("Flush after Finish err = %v, want ErrRealtimeSessionClosed", err)
	}
	if err := sess.Finish(context.Background()); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("second Finish err = %v, want ErrRealtimeSessionClosed", err)
	}

	conn.push(`{"audio":"` + base64.StdEncoding.EncodeToString(chunk3) + `"}`)
	conn.push(`{"audio":null,"isFinal":true}`)
	close(conn.incoming)

	var received []core.AudioChunk
	var combined bytes.Buffer
	for c := range sess.Audio() {
		received = append(received, c)
		combined.Write(c.Data)
	}
	if err := <-sess.Err(); err != nil {
		t.Fatalf("unexpected session error: %v", err)
	}

	if len(received) != 4 {
		t.Fatalf("len(received) = %d, want 4", len(received))
	}
	for i, c := range received {
		if c.Sequence != int64(i) {
			t.Errorf("chunk[%d].Sequence = %d, want %d", i, c.Sequence, i)
		}
		if c.Format != core.AudioFormatPCM16000 {
			t.Errorf("chunk[%d].Format = %v, want PCM16000", i, c.Format)
		}
	}
	if received[0].IsFinal || !received[1].IsFinal || received[2].IsFinal || !received[3].IsFinal {
		t.Errorf("unexpected IsFinal flags: %v, %v, %v, %v",
			received[0].IsFinal, received[1].IsFinal, received[2].IsFinal, received[3].IsFinal)
	}
	wantCombined := append(append(append([]byte{}, chunk1...), chunk2...), chunk3...)
	if !bytes.Equal(combined.Bytes(), wantCombined) {
		t.Errorf("combined audio = %q, want %q", combined.Bytes(), wantCombined)
	}

	// Verify wire messages: BOS, SendText(1), SendText(2), Flush, SendText(turn2), Finish (EOS).
	sent := conn.sentMessages()
	if len(sent) != 6 {
		t.Fatalf("len(sent) = %d, want 6", len(sent))
	}

	var bos wsTTSBOSMessage
	if err := json.Unmarshal(sent[0], &bos); err != nil {
		t.Fatalf("unmarshal BOS: %v", err)
	}
	if bos.Text != " " || bos.VoiceSettings == nil || *bos.VoiceSettings.Stability != 0.55 ||
		bos.GenerationConfig == nil || len(bos.GenerationConfig.ChunkLengthSchedule) != 4 {
		t.Errorf("unexpected BOS payload: %+v", bos)
	}

	var flushMsg wsTTSTextMessage
	if err := json.Unmarshal(sent[3], &flushMsg); err != nil {
		t.Fatalf("unmarshal flush: %v", err)
	}
	if flushMsg.Text != " " || !flushMsg.Flush {
		t.Errorf("unexpected flush message: %+v", flushMsg)
	}

	var eosMsg map[string]any
	if err := json.Unmarshal(sent[5], &eosMsg); err != nil {
		t.Fatalf("unmarshal EOS: %v", err)
	}
	if text, ok := eosMsg["text"].(string); !ok || text != "" {
		t.Errorf("expected EOS {\"text\":\"\"}, got %s", string(sent[5]))
	}
}

func TestRealtimeTTSDefaults(t *testing.T) {
	conn := newMockWSConn(2)
	var dialedURL string
	p := New("test-key", withWSDialer(func(_ context.Context, u string, _ http.Header) (wsConn, *http.Response, error) {
		dialedURL = u
		return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
	}))

	sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{
		VoiceID: "voice-default",
	})
	if err != nil {
		t.Fatalf("StartRealtimeSynthesis() err = %v", err)
	}
	close(conn.incoming)
	if err := sess.Close(); err != nil {
		t.Fatalf("Close() err = %v", err)
	}

	parsed, _ := url.Parse(dialedURL)
	if parsed.Query().Get("model_id") != string(DefaultRealtimeTTSModel) {
		t.Errorf("default model_id = %q, want %q", parsed.Query().Get("model_id"), DefaultRealtimeTTSModel)
	}
	if parsed.Query().Get("output_format") != "mp3_44100_128" {
		t.Errorf("default output_format = %q, want mp3_44100_128", parsed.Query().Get("output_format"))
	}
}

func TestRealtimeTTSModelCompatibility(t *testing.T) {
	dialCalled := false
	p := New("test-key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
		dialCalled = true
		conn := newMockWSConn(1)
		close(conn.incoming)
		return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
	}))

	incompatible := []core.ModelID{
		ModelElevenV3,
		"eleven_v3_conversational",
		"eleven_v4",
		ModelScribeV1,
		ModelScribeV2,
		ModelScribeV2Realtime,
		"scribe_v3",
	}
	for _, model := range incompatible {
		t.Run("reject_"+string(model), func(t *testing.T) {
			dialCalled = false
			_, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{
				Model:   model,
				VoiceID: "voice-1",
			})
			if !errors.Is(err, core.ErrNotSupported) {
				t.Fatalf("model %q err = %v, want core.ErrNotSupported", model, err)
			}
			var pe *core.ProviderError
			if !errors.As(err, &pe) || pe.Code != "unsupported_model" {
				t.Errorf("model %q ProviderError = %+v, want code unsupported_model", model, pe)
			}
			if dialCalled {
				t.Errorf("dial should not be called for incompatible model %q", model)
			}
		})
	}

	compatible := []core.ModelID{
		ModelElevenFlashV25,
		ModelElevenMultilingualV2,
		"eleven_turbo_v2_5",
	}
	for _, model := range compatible {
		t.Run("allow_"+string(model), func(t *testing.T) {
			dialCalled = false
			sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{
				Model:   model,
				VoiceID: "voice-1",
			})
			if err != nil {
				t.Fatalf("model %q unexpected err = %v", model, err)
			}
			if !dialCalled {
				t.Errorf("expected dial for compatible model %q", model)
			}
			_ = sess.Close()
		})
	}
}

func TestRealtimeTTSValidationFailures(t *testing.T) {
	dialCalled := false
	p := New("test-key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
		dialCalled = true
		return newMockWSConn(1), nil, nil
	}))

	//nolint:staticcheck // intentionally testing nil context guard
	if _, err := p.StartRealtimeSynthesis(nil, core.RealtimeSynthesisConfig{VoiceID: "v1"}); err == nil {
		t.Error("expected error for nil ctx")
	}

	invalidCases := []struct {
		name string
		cfg  core.RealtimeSynthesisConfig
	}{
		{"missing voice id", core.RealtimeSynthesisConfig{}},
		{"unsupported format wav", core.RealtimeSynthesisConfig{VoiceID: "v1", Format: core.AudioFormatWAV16000}},
		{"chunk schedule too small", core.RealtimeSynthesisConfig{VoiceID: "v1", ChunkLengthSchedule: []int{49}}},
		{"chunk schedule too large", core.RealtimeSynthesisConfig{VoiceID: "v1", ChunkLengthSchedule: []int{501}}},
		{"inactivity timeout too short", core.RealtimeSynthesisConfig{VoiceID: "v1", InactivityTimeout: 500 * time.Millisecond}},
		{"inactivity timeout too long", core.RealtimeSynthesisConfig{VoiceID: "v1", InactivityTimeout: 181 * time.Second}},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := p.StartRealtimeSynthesis(context.Background(), tc.cfg); !errors.Is(err, core.ErrInvalidSpeechRequest) {
				t.Errorf("err = %v, want core.ErrInvalidSpeechRequest", err)
			}
		})
	}
	if dialCalled {
		t.Error("dialer must not be invoked on validation failures")
	}
}

func TestRealtimeTTSSendTextValidationAndClosedLifecycle(t *testing.T) {
	conn := newMockWSConn(2)
	p := New("test-key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
		return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
	}))

	sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{VoiceID: "v1"})
	if err != nil {
		t.Fatalf("StartRealtimeSynthesis() err = %v", err)
	}

	if err := sess.SendText(context.Background(), ""); !errors.Is(err, core.ErrInvalidSpeechRequest) {
		t.Errorf("empty SendText err = %v, want ErrInvalidSpeechRequest", err)
	}
	if err := sess.SendText(context.Background(), " "); err != nil {
		t.Errorf("whitespace SendText err = %v, want nil", err)
	}
	if got := len(conn.sentMessages()); got != 2 { // BOS + whitespace
		t.Errorf("sent messages = %d, want 2", got)
	}

	// Idempotent Close.
	if err := sess.Close(); err != nil {
		t.Fatalf("first Close() err = %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close() err = %v", err)
	}

	if err := sess.SendText(context.Background(), "after close"); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("SendText after Close err = %v, want ErrRealtimeSessionClosed", err)
	}
	if err := sess.Flush(context.Background()); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("Flush after Close err = %v, want ErrRealtimeSessionClosed", err)
	}
	if err := sess.Finish(context.Background()); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("Finish after Close err = %v, want ErrRealtimeSessionClosed", err)
	}
}

func TestRealtimeTTSDialAndBOSFailures(t *testing.T) {
	secret := "xi_super_secret_tts_ws_key"

	t.Run("dial 401 redacts secret", func(t *testing.T) {
		p := New(secret, withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return nil, &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Request-Id": []string{"req-ws-401"}},
				Body:       io.NopCloser(strings.NewReader(`{"detail":{"status":"invalid_api_key","message":"bad key ` + secret + `"}}`)),
			}, errors.New("handshake failed")
		}))

		_, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{VoiceID: "v1"})
		if !errors.Is(err, core.ErrUnauthorized) {
			t.Fatalf("err = %v, want ErrUnauthorized", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error leaked secret: %v", err)
		}
	})

	t.Run("BOS write failure closes socket and redacts secret", func(t *testing.T) {
		conn := newMockWSConn(1)
		conn.writeErr = errors.New("broken pipe with " + secret)
		p := New(secret, withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		_, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{VoiceID: "v1"})
		if !errors.Is(err, core.ErrNetwork) {
			t.Fatalf("BOS write err = %v, want ErrNetwork", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("BOS error leaked secret: %v", err)
		}
		conn.mu.Lock()
		closed := conn.closed
		conn.mu.Unlock()
		if !closed {
			t.Error("expected underlying WebSocket connection to be closed when BOS write fails")
		}
	})
}

func TestRealtimeTTSServerErrorsAndMalformedFrames(t *testing.T) {
	secret := "xi_secret_err_frame_key"

	cases := []struct {
		name    string
		frame   string
		wantErr error
	}{
		{
			name:    "auth error 401",
			frame:   `{"error":"invalid_api_key","code":401,"message":"unauthorized ` + secret + `"}`,
			wantErr: core.ErrUnauthorized,
		},
		{
			name:    "rate limited 429",
			frame:   `{"error":"rate_limit_exceeded","code":429,"message":"too many concurrent streams"}`,
			wantErr: core.ErrRateLimited,
		},
		{
			name:    "bad request string code",
			frame:   `{"error":"invalid_input","code":"400","message":"invalid voice settings"}`,
			wantErr: core.ErrBadRequest,
		},
		{
			name:    "server error 500",
			frame:   `{"error":"internal_error","code":500,"message":"synthesis backend error"}`,
			wantErr: core.ErrServer,
		},
		{
			name:    "malformed JSON",
			frame:   `{not-valid-json`,
			wantErr: core.ErrDecode,
		},
		{
			name:    "invalid base64 audio",
			frame:   `{"audio":"!!!not-base64!!!","isFinal":false}`,
			wantErr: core.ErrDecode,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := newMockWSConn(2)
			p := New(secret, withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
				return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
			}))

			sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{VoiceID: "v1"})
			if err != nil {
				t.Fatalf("StartRealtimeSynthesis() err = %v", err)
			}
			defer sess.Close()

			conn.push(tc.frame)
			for range sess.Audio() {
			}
			gotErr := <-sess.Err()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("err = %v, want %v", gotErr, tc.wantErr)
			}
			if strings.Contains(gotErr.Error(), secret) {
				t.Errorf("error leaked secret: %v", gotErr)
			}
		})
	}

	t.Run("unexpected network drop", func(t *testing.T) {
		conn := newMockWSConn(1)
		conn.readErr = errors.New("connection reset by peer " + secret)
		p := New(secret, withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{VoiceID: "v1"})
		if err != nil {
			t.Fatalf("StartRealtimeSynthesis() err = %v", err)
		}
		defer sess.Close()

		close(conn.incoming)
		for range sess.Audio() {
		}
		gotErr := <-sess.Err()
		if !errors.Is(gotErr, core.ErrNetwork) {
			t.Fatalf("err = %v, want ErrNetwork", gotErr)
		}
		if strings.Contains(gotErr.Error(), secret) {
			t.Errorf("network error leaked secret: %v", gotErr)
		}
	})
}

func TestRealtimeTTSCancellationBoundedBufferAndConcurrency(t *testing.T) {
	t.Run("context cancel unblocks full bounded buffer", func(t *testing.T) {
		conn := newMockWSConn(4)
		p := New("test-key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		sess, err := p.StartRealtimeSynthesis(ctx, core.RealtimeSynthesisConfig{
			VoiceID:    "v1",
			BufferSize: 1,
		})
		if err != nil {
			t.Fatalf("StartRealtimeSynthesis() err = %v", err)
		}
		defer sess.Close()

		// Fill bounded buffer (cap 1) + block readLoop on second chunk.
		b64 := base64.StdEncoding.EncodeToString([]byte("audio-bytes"))
		conn.push(`{"audio":"` + b64 + `"}`)
		conn.push(`{"audio":"` + b64 + `"}`)
		time.Sleep(15 * time.Millisecond)

		cancel()

		select {
		case gotErr := <-sess.Err():
			if !errors.Is(gotErr, context.Canceled) {
				t.Errorf("err = %v, want context.Canceled", gotErr)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for cancellation error")
		}
	})

	t.Run("Close unblocks full bounded buffer without error", func(t *testing.T) {
		conn := newMockWSConn(4)
		p := New("test-key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{
			VoiceID:    "v1",
			BufferSize: 1,
		})
		if err != nil {
			t.Fatalf("StartRealtimeSynthesis() err = %v", err)
		}

		b64 := base64.StdEncoding.EncodeToString([]byte("audio-bytes"))
		conn.push(`{"audio":"` + b64 + `"}`)
		conn.push(`{"audio":"` + b64 + `"}`)
		time.Sleep(15 * time.Millisecond)

		if err := sess.Close(); err != nil {
			t.Fatalf("Close() err = %v", err)
		}
		if err := <-sess.Err(); err != nil {
			t.Errorf("expected nil error on clean Close, got %v", err)
		}
	})

	t.Run("concurrent SendText Flush and Close", func(t *testing.T) {
		conn := newMockWSConn(16)
		p := New("test-key", withWSDialer(func(_ context.Context, _ string, _ http.Header) (wsConn, *http.Response, error) {
			return conn, &http.Response{StatusCode: http.StatusSwitchingProtocols}, nil
		}))

		sess, err := p.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{VoiceID: "v1"})
		if err != nil {
			t.Fatalf("StartRealtimeSynthesis() err = %v", err)
		}

		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					_ = sess.SendText(context.Background(), "token ")
					_ = sess.Flush(context.Background())
				}
			}()
		}
		time.Sleep(5 * time.Millisecond)
		_ = sess.Close()
		wg.Wait()
	})
}
