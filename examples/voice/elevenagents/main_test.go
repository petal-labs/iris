package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/providers/elevenlabs"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type mockVoiceSession struct {
	events chan core.VoiceEvent
	errCh  chan error
}

func (m *mockVoiceSession) ID() string                                       { return "agent-sess-1" }
func (m *mockVoiceSession) State() core.VoiceSessionState                    { return core.VoiceSessionStateActive }
func (m *mockVoiceSession) SendAudio(context.Context, core.AudioChunk) error { return nil }
func (m *mockVoiceSession) SendText(_ context.Context, _ string) error {
	m.events <- core.VoiceEvent{Type: core.VoiceEventTurnStarted, TurnID: "turn-1"}
	m.events <- core.VoiceEvent{Type: core.VoiceEventAgentTextCommitted, TurnID: "turn-1", Text: "Hello from ElevenAgents!"}
	m.events <- core.VoiceEvent{Type: core.VoiceEventAudioOutput, TurnID: "turn-1", Audio: &core.AudioChunk{Data: []byte("pcm-agent")}}
	m.events <- core.VoiceEvent{Type: core.VoiceEventTurnCompleted, TurnID: "turn-1"}
	return nil
}
func (m *mockVoiceSession) SubmitToolResult(context.Context, core.ToolResult) error { return nil }
func (m *mockVoiceSession) Interrupt(context.Context) error                         { return nil }
func (m *mockVoiceSession) Events() <-chan core.VoiceEvent                          { return m.events }
func (m *mockVoiceSession) Err() <-chan error                                       { return m.errCh }
func (m *mockVoiceSession) Close() error                                            { return nil }

func TestRunElevenAgentsSignedURLAndSession(t *testing.T) {
	t.Run("signed-url-only mints URL and keeps secret redacted in output", func(t *testing.T) {
		const secretToken = "sig_very_secret_token_xyz"
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/v1/convai/conversation/get-signed-url" {
					t.Errorf("unexpected path: %s", req.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"signed_url":"wss://api.elevenlabs.io/v1/convai/conversation?agent_id=agent_1&agent_signature=` + secretToken + `"}`)),
				}, nil
			}),
		}

		p := elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient))
		var out bytes.Buffer
		err := runElevenAgents(context.Background(), p, elevenAgentsExampleConfig{
			AgentID:       "agent_1",
			SignedURLOnly: true,
		}, &out)
		if err != nil {
			t.Fatalf("runElevenAgents(signed-url-only) err = %v", err)
		}
		if strings.Contains(out.String(), secretToken) {
			t.Errorf("stdout leaked raw signed URL token: %s", out.String())
		}
		if !strings.Contains(out.String(), "non_empty=true") {
			t.Errorf("unexpected output: %s", out.String())
		}
	})

	t.Run("consumes single agent turn and writes audio", func(t *testing.T) {
		sess := &mockVoiceSession{
			events: make(chan core.VoiceEvent, 4),
			errCh:  make(chan error, 1),
		}
		outPath := filepath.Join(t.TempDir(), "agent.pcm")
		var out bytes.Buffer
		if err := consumeSingleAgentTurn(context.Background(), sess, "Hi", outPath, &out); err != nil {
			t.Fatalf("consumeSingleAgentTurn() err = %v", err)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("ReadFile() err = %v", err)
		}
		if string(data) != "pcm-agent" {
			t.Errorf("audio = %q, want pcm-agent", data)
		}
		if !strings.Contains(out.String(), "Hello from ElevenAgents!") {
			t.Errorf("missing agent text in stdout: %s", out.String())
		}
	})

	t.Run("missing agent ID fails fast", func(t *testing.T) {
		p := elevenlabs.New("test-key")
		if err := runElevenAgents(context.Background(), p, elevenAgentsExampleConfig{}, io.Discard); err == nil {
			t.Fatal("expected error for missing agent ID")
		}
	})

	t.Run("cancelled context returns context.Canceled", func(t *testing.T) {
		sess := &mockVoiceSession{
			events: make(chan core.VoiceEvent),
			errCh:  make(chan error, 1),
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := consumeSingleAgentTurn(ctx, sess, "", filepath.Join(t.TempDir(), "agent.pcm"), io.Discard)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
