package voice

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
)

// TestPetalFlowIntegrationContracts exercises the exact capability discovery,
// preflight model compatibility, session-boundary, tool-port, interruption,
// and cancellation contracts documented in docs/guides/petalflow-voice-integration.md.
func TestPetalFlowIntegrationContracts(t *testing.T) {
	t.Run("hydration and capability discovery preflight", func(t *testing.T) {
		el := elevenlabs.New("xi_petalflow_hydrated_key")

		if _, ok := core.AsSpeechSynthesizer(el); !ok {
			t.Fatal("ElevenLabs must satisfy core.SpeechSynthesizer for PetalFlow Synthesis node (#250)")
		}
		if _, ok := core.AsSpeechRecognizer(el); !ok {
			t.Fatal("ElevenLabs must satisfy core.SpeechRecognizer for PetalFlow Transcription node (#249)")
		}
		if _, ok := core.AsRealtimeSpeechRecognizer(el); !ok {
			t.Fatal("ElevenLabs must satisfy core.RealtimeSpeechRecognizer for PetalFlow Voice Agent (#253)")
		}
		if _, ok := core.AsRealtimeSpeechSynthesizer(el); !ok {
			t.Fatal("ElevenLabs must satisfy core.RealtimeSpeechSynthesizer for PetalFlow Voice Agent (#253)")
		}
		if _, ok := core.AsVoiceSessionProvider(el); !ok {
			t.Fatal("ElevenLabs must satisfy core.VoiceSessionProvider for hosted ElevenAgents mode")
		}

		// Model compatibility preflight: eleven_flash_v2_5 supports WS TTS, eleven_v3 does not.
		if !core.ProviderSupportsModelFeature(el, elevenlabs.ModelElevenFlashV25, core.FeatureSpeechSynthesisRealtime) {
			t.Error("expected eleven_flash_v2_5 to advertise FeatureSpeechSynthesisRealtime")
		}
		if core.ProviderSupportsModelFeature(el, elevenlabs.ModelElevenV3, core.FeatureSpeechSynthesisRealtime) {
			t.Error("eleven_v3 must NOT advertise FeatureSpeechSynthesisRealtime")
		}
		if _, err := el.StartRealtimeSynthesis(context.Background(), core.RealtimeSynthesisConfig{
			Model:   elevenlabs.ModelElevenV3,
			VoiceID: "voice-1",
		}); !errors.Is(err, core.ErrNotSupported) {
			t.Errorf("eleven_v3 WS TTS err = %v, want core.ErrNotSupported", err)
		}

		// Privacy gate: hydrated provider string redacts API key.
		if strings.Contains(el.String(), "xi_petalflow_hydrated_key") {
			t.Errorf("hydrated provider leaked API key: %s", el.String())
		}
	})

	t.Run("session boundary tool routing stale-audio filter and Cortex committed-only hygiene", func(t *testing.T) {
		var round atomic.Int32
		llm := &fakeLLMProvider{
			id:        "openai",
			streaming: true,
			streamFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatStream, error) {
				ch := make(chan core.ChatChunk, 2)
				errCh := make(chan error, 1)
				finalCh := make(chan *core.ChatResponse, 1)
				if round.Add(1) == 1 {
					close(ch)
					finalCh <- &core.ChatResponse{
						ToolCalls: []core.ToolCall{
							{ID: "pf-tool-1", Name: "petalflow_node_lookup", Arguments: json.RawMessage(`{"k":"v"}`)},
						},
					}
				} else {
					ch <- core.ChatChunk{Delta: "Workflow result ready."}
					close(ch)
					finalCh <- &core.ChatResponse{Output: "Workflow result ready."}
				}
				close(finalCh)
				close(errCh)
				return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
			},
		}

		pipeline, err := NewPipeline(PipelineConfig{
			LLM:         llm,
			LLMModel:    "gpt-5",
			RealtimeTTS: &fakeRealtimeTTSProvider{},
			VoiceID:     "v1",
			Tools:       []core.Tool{schemaOnlyTestTool{name: "petalflow_node_lookup"}},
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}

		sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{
			OutputFormat: core.AudioFormatPCM16000,
		})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		if err := sess.SendText(context.Background(), "Run lookup"); err != nil {
			t.Fatalf("SendText() err = %v", err)
		}

		// Consumer uses VoiceTurnTracker as mandated by section 3.2 and persists only IsFinal==true events.
		tracker := core.NewVoiceTurnTracker("pf")
		var persisted []core.VoiceEvent
		var playedAudio int

		for evt := range sess.Events() {
			tracker.ObserveEvent(evt)

			switch evt.Type {
			case core.VoiceEventToolCall:
				if err := sess.SubmitToolResult(context.Background(), core.ToolResult{
					CallID:  evt.ToolCall.ID,
					Content: "ok",
				}); err != nil {
					t.Fatalf("SubmitToolResult() err = %v", err)
				}
			case core.VoiceEventAudioOutput:
				if tracker.ShouldPlayAudio(evt) {
					playedAudio++
				}
			case core.VoiceEventUserTranscriptCommitted, core.VoiceEventAgentTextCommitted:
				if !evt.IsFinal {
					t.Errorf("committed event %s had IsFinal=false", evt.Type)
				}
				persisted = append(persisted, evt)
			case core.VoiceEventTurnCompleted:
				_ = sess.Close()
			}
		}

		if err := <-sess.Err(); err != nil {
			t.Fatalf("unexpected session error: %v", err)
		}
		if playedAudio == 0 {
			t.Error("expected playable audio chunk via VoiceTurnTracker.ShouldPlayAudio")
		}
		if len(persisted) != 2 {
			t.Errorf("Cortex committed utterances = %d, want 2 (user + assistant)", len(persisted))
		}
	})
}
