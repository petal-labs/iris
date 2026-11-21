// Example: ElevenLabs File-Based Scribe Transcription
//
// This example demonstrates transcribing an audio file with ElevenLabs Scribe
// via core.Client.Transcribe, including optional speaker diarization, word
// timestamps, and non-speech audio event tagging.
//
// Run with:
//
//	export ELEVENLABS_API_KEY=your-key
//	go run ./examples/speech/elevenlabs-transcribe -file speech.mp3 -diarize -timestamps word
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/providers/elevenlabs"
)

type transcribeConfig struct {
	FilePath       string
	Model          core.ModelID
	Language       string
	Diarize        bool
	Timestamps     core.TranscriptionTimestampGranularity
	TagAudioEvents bool
}

func runTranscribe(ctx context.Context, client *core.Client, cfg transcribeConfig, stdout io.Writer) error {
	if cfg.FilePath == "" {
		return fmt.Errorf("input audio -file path is required")
	}

	f, err := os.Open(cfg.FilePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", cfg.FilePath, err)
	}
	defer f.Close()

	resp, err := client.Transcribe(ctx, &core.TranscriptionRequest{
		Model:          cfg.Model,
		Reader:         f,
		Filename:       filepath.Base(cfg.FilePath),
		Language:       cfg.Language,
		Diarize:        cfg.Diarize,
		Timestamps:     cfg.Timestamps,
		TagAudioEvents: cfg.TagAudioEvents,
	})
	if err != nil {
		return fmt.Errorf("transcribe audio: %w", err)
	}

	fmt.Fprintf(stdout, "Transcription ID: %s (model=%s, language=%s, duration=%s, final=%t, words=%d)\n",
		resp.ID, resp.Model, resp.Language, resp.Duration, resp.IsFinal, len(resp.Words))
	fmt.Fprintf(stdout, "Transcript: %s\n", resp.Text)

	for _, w := range resp.Words {
		if w.Type == "spacing" {
			continue
		}
		if w.SpeakerID != "" {
			fmt.Fprintf(stdout, "  [%s - %s] (%s) %s\n", w.Start, w.End, w.SpeakerID, w.Text)
		} else {
			fmt.Fprintf(stdout, "  [%s - %s] %s\n", w.Start, w.End, w.Text)
		}
	}
	return nil
}

func main() {
	filePath := flag.String("file", "", "Path to an audio file (mp3, wav, m4a, flac, ogg, webm)")
	model := flag.String("model", string(elevenlabs.ModelScribeV2), "ElevenLabs Scribe model ID")
	language := flag.String("language", "", "Optional ISO-639 language code (empty = auto-detect)")
	diarize := flag.Bool("diarize", false, "Enable speaker diarization")
	timestamps := flag.String("timestamps", string(core.TimestampGranularityWord), "Timestamp granularity: none, word, or character")
	tagEvents := flag.Bool("events", false, "Tag non-speech audio events (laughter, applause, etc.)")
	flag.Parse()

	provider, err := elevenlabs.NewFromEnv(elevenlabs.WithTimeout(120 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ts := core.TranscriptionTimestampGranularity(*timestamps)
	if *timestamps == "none" {
		ts = core.TimestampGranularityNone
	}

	client := core.NewClient(provider)
	cfg := transcribeConfig{
		FilePath:       *filePath,
		Model:          core.ModelID(*model),
		Language:       *language,
		Diarize:        *diarize,
		Timestamps:     ts,
		TagAudioEvents: *tagEvents,
	}

	if err := runTranscribe(ctx, client, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
