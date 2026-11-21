// Example: ElevenLabs Synchronous Text-to-Speech
//
// This example demonstrates generating audio from text with ElevenLabs using
// core.Client.Synthesize and saving the resulting audio file to disk.
//
// Run with:
//
//	export ELEVENLABS_API_KEY=your-key
//	go run ./examples/speech/elevenlabs-synthesize -text "Hello from Iris." -out speech.mp3
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/providers/elevenlabs"
)

const defaultVoiceID = "JBFqnCBsd6RMkjVDRZzb" // ElevenLabs public "George" voice

type synthesizeConfig struct {
	Text    string
	VoiceID string
	Model   core.ModelID
	OutPath string
}

func runSynthesize(ctx context.Context, client *core.Client, cfg synthesizeConfig, stdout io.Writer) error {
	resp, err := client.Synthesize(ctx, &core.SpeechRequest{
		Model:   cfg.Model,
		VoiceID: cfg.VoiceID,
		Text:    cfg.Text,
		Format:  core.AudioFormatMP3_44100_128,
	})
	if err != nil {
		return fmt.Errorf("synthesize speech: %w", err)
	}

	audioBytes, err := resp.Audio.GetBytes()
	if err != nil {
		return fmt.Errorf("decode audio bytes: %w", err)
	}

	if err := os.WriteFile(cfg.OutPath, audioBytes, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", cfg.OutPath, err)
	}

	fmt.Fprintf(stdout, "Saved %d bytes (%s) to %s (model=%s, request_id=%s, character_cost=%d)\n",
		len(audioBytes), resp.Audio.Format.String(), cfg.OutPath, resp.Model, resp.ID, resp.Usage.InputCharacters)
	return nil
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
	text := flag.String("text", "Hello from Iris and ElevenLabs.", "Text to synthesize into speech")
	voice := flag.String("voice", "", "ElevenLabs voice ID (defaults to ELEVENLABS_VOICE_ID or George)")
	model := flag.String("model", string(elevenlabs.ModelElevenFlashV25), "ElevenLabs TTS model ID")
	outPath := flag.String("out", "speech.mp3", "Output audio file path")
	flag.Parse()

	provider, err := elevenlabs.NewFromEnv(elevenlabs.WithTimeout(60 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := core.NewClient(provider)
	cfg := synthesizeConfig{
		Text:    *text,
		VoiceID: resolveVoice(*voice),
		Model:   core.ModelID(*model),
		OutPath: *outPath,
	}

	if err := runSynthesize(ctx, client, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
