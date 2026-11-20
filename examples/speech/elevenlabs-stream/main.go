// Example: ElevenLabs HTTP Streaming Text-to-Speech
//
// This example demonstrates consuming incremental audio chunks from ElevenLabs
// via core.Client.StreamSynthesize, measuring first-chunk latency and writing
// chunks directly to disk without buffering the full audio payload in memory.
//
// Run with:
//
//	export ELEVENLABS_API_KEY=your-key
//	go run ./examples/speech/elevenlabs-stream -text "Streaming audio from Iris." -out stream.mp3
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
)

const defaultVoiceID = "JBFqnCBsd6RMkjVDRZzb" // ElevenLabs public "George" voice

type streamConfig struct {
	Text    string
	VoiceID string
	Model   core.ModelID
	OutPath string
}

func runStream(ctx context.Context, client *core.Client, cfg streamConfig, stdout io.Writer) error {
	start := time.Now()
	stream, err := client.StreamSynthesize(ctx, &core.SpeechRequest{
		Model:   cfg.Model,
		VoiceID: cfg.VoiceID,
		Text:    cfg.Text,
		Format:  core.AudioFormatMP3_44100_128,
	})
	if err != nil {
		return fmt.Errorf("start speech stream: %w", err)
	}

	outFile, err := os.OpenFile(cfg.OutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", cfg.OutPath, err)
	}
	defer outFile.Close()

	var (
		chunks     int
		totalBytes int64
		firstChunk time.Duration
	)
	for chunk := range stream.Ch {
		if len(chunk.Data) == 0 {
			continue
		}
		if chunks == 0 {
			firstChunk = time.Since(start)
		}
		n, wErr := outFile.Write(chunk.Data)
		if wErr != nil {
			return fmt.Errorf("write chunk %d: %w", chunk.Sequence, wErr)
		}
		chunks++
		totalBytes += int64(n)
	}

	if err := <-stream.Err; err != nil {
		return fmt.Errorf("stream error: %w", err)
	}
	finalResp := <-stream.Final

	reqID := ""
	if finalResp != nil {
		reqID = finalResp.ID
	}
	fmt.Fprintf(stdout, "Streamed %d chunks (%d bytes) to %s (first_chunk=%s, total=%s, request_id=%s)\n",
		chunks, totalBytes, cfg.OutPath, firstChunk.Round(time.Millisecond), time.Since(start).Round(time.Millisecond), reqID)
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
	text := flag.String("text", "Streaming audio chunks from Iris and ElevenLabs.", "Text to stream as speech")
	voice := flag.String("voice", "", "ElevenLabs voice ID (defaults to ELEVENLABS_VOICE_ID or George)")
	model := flag.String("model", string(elevenlabs.ModelElevenFlashV25), "ElevenLabs TTS model ID")
	outPath := flag.String("out", "stream.mp3", "Output audio file path")
	flag.Parse()

	provider, err := elevenlabs.NewFromEnv(elevenlabs.WithTimeout(60 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := core.NewClient(provider)
	cfg := streamConfig{
		Text:    *text,
		VoiceID: resolveVoice(*voice),
		Model:   core.ModelID(*model),
		OutPath: *outPath,
	}

	if err := runStream(ctx, client, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
