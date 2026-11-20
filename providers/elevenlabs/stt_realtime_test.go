package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/petal-labs/iris/core"
)

type mockWSConn struct {
	mu       sync.Mutex
	incoming chan []byte
	readErr  error
	writeErr error
	sent     [][]byte
	closed   bool
	closeCh  chan struct{}
}

func newMockWSConn(buf int) *mockWSConn {
	return &mockWSConn{
		incoming: make(chan []byte, buf),
		closeCh:  make(chan struct{}),
	}
}

func (m *mockWSConn) push(msg string) {
	m.incoming <- []byte(msg)
}

func (m *mockWSConn) ReadText(ctx context.Context) ([]byte, error) {
	select {
	case data, ok := <-m.incoming:
		if !ok {
			m.mu.Lock()
			err := m.readErr
			m.mu.Unlock()
			if err != nil {
				return nil, err
			}
			return nil, io.EOF
		}
		return data, nil
	case <-m.closeCh:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockWSConn) WriteText(ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("connection closed")
	}
	if m.writeErr != nil {
		return m.writeErr
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	m.sent = append(m.sent, cp)
	return nil
}

func (m *mockWSConn) Close(_ websocket.StatusCode, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.closeCh)
	}
	return nil
}

func (m *mockWSConn) sentMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, len(m.sent))
	copy(out, m.sent)
	return out
}

func withWSDialer(d wsDialer) Option {
	return func(c *Config) {
		c.wsDialer = d
	}
}

func TestRealtimeSTTFormatResolutionAndWSURL(t *testing.T) {
	wire, resolved, err := resolveRealtimeSTTFormat(core.AudioFormatPCM16000)
	if err != nil || wire != "pcm_16000" || resolved.SampleRate != 16000 {
		t.Fatalf("pcm16000 = (%q, %+v, %v)", wire, resolved, err)
	}
	wire, resolved, err = resolveRealtimeSTTFormat(core.AudioFormatULaw8000)
	if err != nil || wire != "ulaw_8000" || resolved.SampleRate != 8000 {
		t.Fatalf("ulaw8000 = (%q, %+v, %v)", wire, resolved, err)
	}

	for _, bad := range []core.AudioFormat{
		core.AudioFormatMP3_44100_128,
		core.AudioFormatWAV16000,
		{Encoding: core.AudioEncodingPCM, SampleRate: 16000, Channels: 2},
		{Encoding: core.AudioEncodingULaw, SampleRate: 16000},
		{Encoding: core.AudioEncodingPCM, SampleRate: 11025},
	} {
		if _, _, err := resolveRealtimeSTTFormat(bad); !errors.Is(err, core.ErrInvalidAudioFormat) {
			t.Errorf("resolveRealtimeSTTFormat(%+v) err = %v, want ErrInvalidAudioFormat", bad, err)
		}
	}

	pHTTPS := New("k", WithBaseURL("https://api.elevenlabs.io"))
	if u, err := pHTTPS.wsEndpointURL("/v1/speech-to-text/realtime"); err != nil || u != "wss://api.elevenlabs.io/v1/speech-to-text/realtime" {
		t.Errorf("https->wss = (%q, %v)", u, err)
	}
	pHTTP := New("k", WithBaseURL("http://127.0.0.1:8080"))
	if u, err := pHTTP.wsEndpointURL("/v1/speech-to-text/realtime"); err != nil || u != "ws://127.0.0.1:8080/v1/speech-to-text/realtime" {
		t.Errorf("http->ws = (%q, %v)", u, err)
	}
}

func TestRealtimeScribeSessionPartialCommittedAndTimestamps(t *testing.T) {
	conn := newMockWSConn(8)
	var capturedURL string
	var capturedKey string

	p := New("sk_rt_key", withWSDialer(func(ctx context.Context, urlStr string, headers http.Header) (wsConn, *http.Response, error) {
		capturedURL = urlStr
		capturedKey = headers.Get("xi-api-key")
		return conn, nil, nil
	}))

	client := core.NewClient(p)
	sess, err := client.StartRealtimeTranscription(context.Background(), core.RealtimeTranscriptionConfig{
		Model:             ModelScribeV2Realtime,
		Format:            core.AudioFormatPCM16000,
		Language:          "en",
		CommitStrategy:    core.CommitStrategyManual,
		IncludeTimestamps: true,
		MaxChunkBytes:     1024,
		EventBufferSize:   8,
	})
	if err != nil {
		t.Fatalf("StartRealtimeTranscription() error = %v", err)
	}
	defer sess.Close()

	if capturedKey != "sk_rt_key" {
		t.Errorf("xi-api-key = %q, want sk_rt_key", capturedKey)
	}
	for _, want := range []string{
		"wss://api.elevenlabs.io/v1/speech-to-text/realtime?",
		"model_id=scribe_v2_realtime",
		"audio_format=pcm_16000",
		"language_code=en",
		"commit_strategy=manual",
		"include_timestamps=true",
	} {
		if !strings.Contains(capturedURL, want) {
			t.Errorf("dial URL %q missing %q", capturedURL, want)
		}
	}

	if cap(sess.Events()) != 8 {
		t.Errorf("cap(Events()) = %d, want 8", cap(sess.Events()))
	}

	// Send audio chunk and manual commit.
	pcmData := []byte{0x01, 0x02, 0x03, 0x04}
	if err := sess.SendAudio(context.Background(), core.AudioChunk{
		Data:     pcmData,
		Format:   core.AudioFormatPCM16000,
		Sequence: 0,
	}); err != nil {
		t.Fatalf("SendAudio() error = %v", err)
	}
	if err := sess.Commit(context.Background()); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	sent := conn.sentMessages()
	if len(sent) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(sent))
	}
	var chunkMsg realtimeAudioChunkMessage
	if err := json.Unmarshal(sent[0], &chunkMsg); err != nil {
		t.Fatalf("unmarshal sent[0]: %v", err)
	}
	if chunkMsg.MessageType != "input_audio_chunk" || chunkMsg.AudioBase64 != base64.StdEncoding.EncodeToString(pcmData) || chunkMsg.SampleRate != 16000 || chunkMsg.Commit {
		t.Errorf("sent[0] = %+v", chunkMsg)
	}
	var commitMsg realtimeAudioChunkMessage
	if err := json.Unmarshal(sent[1], &commitMsg); err != nil {
		t.Fatalf("unmarshal sent[1]: %v", err)
	}
	if commitMsg.MessageType != "input_audio_chunk" || !commitMsg.Commit || commitMsg.AudioBase64 != "" {
		t.Errorf("sent[1] = %+v", commitMsg)
	}

	// Simulate server events: session_started -> partial -> committed -> committed_with_timestamps -> unknown ignored -> EOF.
	conn.push(`{"message_type":"session_started","session_id":"rt_sess_1"}`)
	conn.push(`{"message_type":"partial_transcript","text":"Hello"}`)
	conn.push(`{"message_type":"committed_transcript","text":"Hello world","language_code":"en"}`)
	conn.push(`{"message_type":"committed_transcript_with_timestamps","text":"Hello world","language_code":"en","words":[{"text":"Hello","start":0.0,"end":0.4,"type":"word"}]}`)
	conn.push(`{"message_type":"keepalive_ping"}`)
	close(conn.incoming)

	var events []core.RealtimeTranscriptEvent
	for evt := range sess.Events() {
		events = append(events, evt)
	}
	if err := <-sess.Err(); err != nil {
		t.Fatalf("unexpected session error: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("events = %d, want 4: %+v", len(events), events)
	}
	if events[0].Type != core.RealtimeTranscriptEventSessionStarted || events[0].SessionID != "rt_sess_1" {
		t.Errorf("events[0] = %+v", events[0])
	}
	if events[1].Type != core.RealtimeTranscriptEventPartial || events[1].IsFinal || events[1].Transcript.Text != "Hello" || events[1].Transcript.IsFinal {
		t.Errorf("events[1] = %+v", events[1])
	}
	if events[2].Type != core.RealtimeTranscriptEventCommitted || !events[2].IsFinal || events[2].Transcript.Text != "Hello world" || !events[2].Transcript.IsFinal {
		t.Errorf("events[2] = %+v", events[2])
	}
	if events[3].Type != core.RealtimeTranscriptEventCommitted || !events[3].IsFinal || len(events[3].Transcript.Words) != 1 || events[3].Transcript.Words[0].End != 400*time.Millisecond {
		t.Errorf("events[3] = %+v", events[3])
	}
}

func TestRealtimeScribeChunkValidation(t *testing.T) {
	conn := newMockWSConn(4)
	p := New("k", withWSDialer(func(context.Context, string, http.Header) (wsConn, *http.Response, error) {
		return conn, nil, nil
	}))

	sess, err := p.StartRealtimeTranscription(context.Background(), core.RealtimeTranscriptionConfig{
		Format:        core.AudioFormatPCM16000,
		MaxChunkBytes: 8,
	})
	if err != nil {
		t.Fatalf("StartRealtimeTranscription() error = %v", err)
	}
	defer sess.Close()

	badChunks := []struct {
		name  string
		chunk core.AudioChunk
	}{
		{"empty non-final", core.AudioChunk{Sequence: 0}},
		{"exceeds max chunk bytes", core.AudioChunk{Data: make([]byte, 10), Sequence: 0}},
		{"mismatched encoding", core.AudioChunk{Data: []byte{1, 2}, Format: core.AudioFormatULaw8000}},
		{"mismatched sample rate", core.AudioChunk{Data: []byte{1, 2}, Format: core.AudioFormatPCM24000}},
		{"odd pcm byte length", core.AudioChunk{Data: []byte{1, 2, 3}}},
	}

	for _, tc := range badChunks {
		t.Run(tc.name, func(t *testing.T) {
			if err := sess.SendAudio(context.Background(), tc.chunk); !errors.Is(err, core.ErrInvalidAudioChunk) {
				t.Errorf("SendAudio(%s) err = %v, want ErrInvalidAudioChunk", tc.name, err)
			}
		})
	}
	if len(conn.sentMessages()) != 0 {
		t.Error("invalid chunks must not be written to WebSocket")
	}

	// After Close(), SendAudio and Commit must return ErrRealtimeSessionClosed.
	if err := sess.Close(); err != nil {
		t.Fatalf("Close() err = %v", err)
	}
	if err := sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2}}); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("SendAudio after close err = %v, want ErrRealtimeSessionClosed", err)
	}
	if err := sess.Commit(context.Background()); !errors.Is(err, core.ErrRealtimeSessionClosed) {
		t.Errorf("Commit after close err = %v, want ErrRealtimeSessionClosed", err)
	}
}

func TestRealtimeScribeErrorsCancellationAndBoundedBufferCleanup(t *testing.T) {
	t.Run("preflight validation and dial HTTP error", func(t *testing.T) {
		p := New("k", withWSDialer(func(context.Context, string, http.Header) (wsConn, *http.Response, error) {
			return nil, &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Request-Id": []string{"req-ws-401"}},
				Body:       io.NopCloser(strings.NewReader(`{"detail":{"status":"invalid_api_key","message":"bad key k"}}`)),
			}, errors.New("handshake failed")
		}))

		// Unsupported format fails before dial.
		if _, err := p.StartRealtimeTranscription(context.Background(), core.RealtimeTranscriptionConfig{
			Format: core.AudioFormatMP3_44100_128,
		}); !errors.Is(err, core.ErrInvalidAudioFormat) {
			t.Errorf("unsupported format err = %v, want ErrInvalidAudioFormat", err)
		}

		// Dial 401 maps to ErrUnauthorized.
		_, err := p.StartRealtimeTranscription(context.Background(), core.RealtimeTranscriptionConfig{
			Format: core.AudioFormatPCM16000,
		})
		if !errors.Is(err, core.ErrUnauthorized) {
			t.Fatalf("dial 401 err = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("server error message types map to typed ProviderError", func(t *testing.T) {
		cases := []struct {
			msgType string
			want    error
		}{
			{"auth_error", core.ErrUnauthorized},
			{"rate_limited", core.ErrRateLimited},
			{"chunk_size_exceeded", core.ErrBadRequest},
			{"transcriber_error", core.ErrServer},
		}

		for _, tc := range cases {
			conn := newMockWSConn(2)
			p := New("sk_secret", withWSDialer(func(context.Context, string, http.Header) (wsConn, *http.Response, error) {
				return conn, nil, nil
			}))
			sess, err := p.StartRealtimeTranscription(context.Background(), core.RealtimeTranscriptionConfig{
				Format: core.AudioFormatPCM16000,
			})
			if err != nil {
				t.Fatalf("start err = %v", err)
			}
			conn.push(`{"message_type":"` + tc.msgType + `","error":"problem with sk_secret"}`)
			for range sess.Events() {
			}
			gotErr := <-sess.Err()
			if !errors.Is(gotErr, tc.want) {
				t.Errorf("msgType %q err = %v, want %v", tc.msgType, gotErr, tc.want)
			}
			if strings.Contains(gotErr.Error(), "sk_secret") {
				t.Errorf("error leaked secret: %v", gotErr)
			}
			_ = sess.Close()
		}
	})

	t.Run("cancellation unblocks full bounded buffer and closes cleanly", func(t *testing.T) {
		conn := newMockWSConn(10)
		p := New("k", withWSDialer(func(context.Context, string, http.Header) (wsConn, *http.Response, error) {
			return conn, nil, nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		sess, err := p.StartRealtimeTranscription(ctx, core.RealtimeTranscriptionConfig{
			Format:          core.AudioFormatPCM16000,
			EventBufferSize: 1,
		})
		if err != nil {
			t.Fatalf("start err = %v", err)
		}

		// Fill buffer (1 slot) plus one pending event so readLoop blocks on send.
		conn.push(`{"message_type":"partial_transcript","text":"one"}`)
		conn.push(`{"message_type":"partial_transcript","text":"two"}`)
		time.Sleep(15 * time.Millisecond)

		cancel()

		select {
		case gotErr := <-sess.Err():
			if !errors.Is(gotErr, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", gotErr)
			}
		case <-time.After(time.Second):
			t.Fatal("session did not exit promptly when cancelled with full bounded buffer")
		}
		_ = sess.Close()
	})
}
