// Example: Iris-Managed Real-Time Voice Pipeline
//
// This example composes ElevenLabs real-time STT (Scribe v2 Realtime) and
// incremental WebSocket TTS (Flash v2.5) with an Iris LLM provider (OpenAI by
// default) inside voice.Pipeline. It demonstrates:
//   - Provider-agnostic core.VoiceSession consumption
//   - Streaming LLM tokens directly into incremental WebSocket TTS
//   - Stale-audio suppression via core.VoiceTurnTracker
//   - Privacy-safe latency telemetry via core.VoiceTelemetryHook
//   - Running either a text turn (-prompt) or a raw 16kHz mono PCM audio turn (-pcm-in)
//
// Run with:
//
//	export ELEVENLABS_API_KEY=your-key
//	export OPENAI_API_KEY=your-key
//	go run ./examples/voice/pipeline -prompt "Give me a one-sentence fun fact about Go." -out reply.pcm
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/providers/elevenlabs"
	"github.com/petal-labs/iris/v2/providers/openai"
	"github.com/petal-labs/iris/v2/voice"
)

const defaultVoiceID = "JBFqnCBsd6RMkjVDRZzb" // ElevenLabs public "George" voice

type pipelineExampleConfig struct {
	Prompt    string
	PCMInPath string
	OutPath   string
	VoiceID   string
	LLMModel  core.ModelID
}

type latencyRecorder struct {
	core.NoopVoiceTelemetryHook
	mu           sync.Mutex
	firstAudio   time.Duration
	turnDuration time.Duration
}

func (r *latencyRecorder) OnVoiceTurnEnd(_ context.Context, e core.VoiceTurnEndEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.FirstAudioLatency > 0 {
		r.firstAudio = e.FirstAudioLatency
	}
	if d := e.Duration(); d > 0 {
		r.turnDuration = d
	}
}

func (r *latencyRecorder) snapshot() (firstAudio, turnDuration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.firstAudio, r.turnDuration
}

func runVoicePipeline(
	ctx context.Context,
	stt core.RealtimeSpeechRecognizer,
	llm core.Provider,
	rtTTS core.RealtimeSpeechSynthesizer,
	cfg pipelineExampleConfig,
	stdout io.Writer,
) error {
	if strings.TrimSpace(cfg.Prompt) == "" && strings.TrimSpace(cfg.PCMInPath) == "" {
		return fmt.Errorf("either -prompt or -pcm-in is required")
	}

	metrics := &latencyRecorder{}
	pipeline, err := voice.NewPipeline(voice.PipelineConfig{
		STT:          stt,
		STTModel:     elevenlabs.ModelScribeV2Realtime,
		LLM:          llm,
		LLMModel:     cfg.LLMModel,
		RealtimeTTS:  rtTTS,
		TTSModel:     elevenlabs.ModelElevenFlashV25,
		VoiceID:      cfg.VoiceID,
		SystemPrompt: "You are a concise, friendly voice assistant. Answer in one or two short sentences.",
		Telemetry:    metrics,
	})
	if err != nil {
		return fmt.Errorf("create voice pipeline: %w", err)
	}

	sess, err := pipeline.StartVoiceSession(ctx, core.VoiceSessionConfig{
		InputFormat:  core.AudioFormatPCM16000,
		OutputFormat: core.AudioFormatPCM16000,
	})
	if err != nil {
		return fmt.Errorf("start voice session: %w", err)
	}
	defer func() { _ = sess.Close() }()

	if cfg.PCMInPath != "" {
		pcmBytes, readErr := os.ReadFile(cfg.PCMInPath)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", cfg.PCMInPath, readErr)
		}
		const chunkSize = 8192
		var seq int64
		for off := 0; off < len(pcmBytes); off += chunkSize {
			end := off + chunkSize
			if end > len(pcmBytes) {
				end = len(pcmBytes)
			}
			if sendErr := sess.SendAudio(ctx, core.AudioChunk{
				Data:     pcmBytes[off:end],
				Format:   core.AudioFormatPCM16000,
				Sequence: seq,
				IsFinal:  end == len(pcmBytes),
			}); sendErr != nil {
				return fmt.Errorf("send audio chunk %d: %w", seq, sendErr)
			}
			seq++
		}
	} else {
		if sendErr := sess.SendText(ctx, cfg.Prompt); sendErr != nil {
			return fmt.Errorf("send text: %w", sendErr)
		}
	}

	outFile, err := os.OpenFile(cfg.OutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", cfg.OutPath, err)
	}
	defer outFile.Close()

	tracker := core.NewVoiceTurnTracker("consumer")
	var (
		audioBytes    int64
		assistantText string
		userText      string
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-sess.Events():
			if !ok {
				if sessErr := <-sess.Err(); sessErr != nil {
					return fmt.Errorf("voice session failed: %w", sessErr)
				}
				return fmt.Errorf("voice session closed before turn completed")
			}
			tracker.ObserveEvent(evt)

			switch evt.Type {
			case core.VoiceEventUserTranscriptCommitted:
				if evt.Transcript != nil {
					userText = evt.Transcript.Text
				}
			case core.VoiceEventAgentTextCommitted:
				assistantText = evt.Text
			case core.VoiceEventAudioOutput:
				if evt.Audio != nil && len(evt.Audio.Data) > 0 && tracker.ShouldPlayAudio(evt) {
					n, wErr := outFile.Write(evt.Audio.Data)
					if wErr != nil {
						return fmt.Errorf("write audio output: %w", wErr)
					}
					audioBytes += int64(n)
				}
			case core.VoiceEventError:
				if evt.Err != nil {
					return fmt.Errorf("voice turn error: %w", evt.Err)
				}
			case core.VoiceEventTurnCompleted:
				firstAudio, turnDur := metrics.snapshot()
				fmt.Fprintf(stdout, "Turn %s completed (user_chars=%d, assistant_chars=%d, audio_bytes=%d, first_audio=%s, duration=%s)\n",
					evt.TurnID, len(userText), len(assistantText), audioBytes,
					firstAudio.Round(time.Millisecond), turnDur.Round(time.Millisecond))
				fmt.Fprintf(stdout, "Assistant: %s\n", assistantText)
				fmt.Fprintf(stdout, "Saved PCM 16kHz mono audio to %s\n", cfg.OutPath)
				return nil
			}
		}
	}
}

func resolveVoice(flagVoice string) string {
	if v := strings.TrimSpace(flagVoice); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID")); v != "" {
		return v
	}
	return defaultVoiceID
}

func main() {
	prompt := flag.String("prompt", "Give me a one-sentence fun fact about Go.", "Text prompt for a voice turn (ignored when -pcm-in is set)")
	pcmIn := flag.String("pcm-in", "", "Optional 16kHz mono signed 16-bit PCM file to stream through Scribe v2 Realtime STT")
	outPath := flag.String("out", "reply.pcm", "Output 16kHz mono PCM audio file path")
	voiceID := flag.String("voice", "", "ElevenLabs voice ID (defaults to ELEVENLABS_VOICE_ID or George)")
	model := flag.String("model", string(openai.ModelGPT4oMini), "OpenAI LLM model ID")
	flag.Parse()

	el, err := elevenlabs.NewFromEnv(elevenlabs.WithTimeout(60 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	llm, err := openai.NewFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var stt core.RealtimeSpeechRecognizer
	if strings.TrimSpace(*pcmIn) != "" {
		stt = el
	}

	cfg := pipelineExampleConfig{
		Prompt:    *prompt,
		PCMInPath: *pcmIn,
		OutPath:   *outPath,
		VoiceID:   resolveVoice(*voiceID),
		LLMModel:  core.ModelID(*model),
	}
	if err := runVoicePipeline(ctx, stt, llm, el, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
