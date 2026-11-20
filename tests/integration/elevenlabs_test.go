//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
)

// defaultElevenLabsVoiceID is ElevenLabs' public "George" voice, overridable via ELEVENLABS_VOICE_ID.
const defaultElevenLabsVoiceID = "JBFqnCBsd6RMkjVDRZzb"

func elevenLabsTestVoiceID() string {
	if v := strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID")); v != "" {
		return v
	}
	return defaultElevenLabsVoiceID
}

func TestElevenLabs_Synthesize_And_Transcribe_RoundTrip(t *testing.T) {
	skipIfNoElevenLabsKey(t)

	provider := elevenlabs.New(getElevenLabsKey(t), elevenlabs.WithTimeout(30*time.Second))
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	ttsResp, err := client.Synthesize(ctx, &core.SpeechRequest{
		Model:   elevenlabs.ModelElevenFlashV25,
		VoiceID: elevenLabsTestVoiceID(),
		Text:    "Hello from Iris.",
		Format:  core.AudioFormatMP3_44100_128,
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if err := ttsResp.Audio.Validate(); err != nil {
		t.Fatalf("Synthesize() returned invalid AudioData: %v", err)
	}
	if len(ttsResp.Audio.Data) == 0 {
		t.Fatal("Synthesize() returned empty audio bytes")
	}
	t.Logf("Synthesize ok: bytes=%d format=%s char_cost=%d", len(ttsResp.Audio.Data), ttsResp.Audio.Format.String(), ttsResp.Usage.InputCharacters)

	sttResp, err := client.Transcribe(ctx, &core.TranscriptionRequest{
		Model:      elevenlabs.ModelScribeV2,
		Audio:      &ttsResp.Audio,
		Filename:   "roundtrip.mp3",
		Timestamps: core.TimestampGranularityWord,
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if err := sttResp.Validate(); err != nil {
		t.Fatalf("Transcribe() returned invalid response: %v", err)
	}
	if !sttResp.IsFinal {
		t.Error("expected IsFinal == true for file-based Scribe transcription")
	}
	if strings.TrimSpace(sttResp.Text) == "" {
		t.Error("Transcribe() returned empty transcript text")
	}
	if sttResp.Language == "" {
		t.Error("Transcribe() returned empty detected language")
	}
	if len(sttResp.Words) == 0 {
		t.Error("Transcribe() returned no word timestamps")
	}
	// Privacy: log metadata counts only, never transcript text or raw audio.
	t.Logf("Transcribe ok: %s", sttResp.String())
}

func TestElevenLabs_StreamSynthesize(t *testing.T) {
	skipIfNoElevenLabsKey(t)

	provider := elevenlabs.New(getElevenLabsKey(t), elevenlabs.WithTimeout(30*time.Second))
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.StreamSynthesize(ctx, &core.SpeechRequest{
		Model:   elevenlabs.ModelElevenFlashV25,
		VoiceID: elevenLabsTestVoiceID(),
		Text:    "Streaming check.",
		Format:  core.AudioFormatMP3_44100_128,
	})
	if err != nil {
		t.Fatalf("StreamSynthesize() error = %v", err)
	}

	var dataChunks int
	var totalBytes int
	var sawFinalMarker bool
	var expectedSeq int64

	for chunk := range stream.Ch {
		if chunk.Sequence != expectedSeq {
			t.Errorf("chunk.Sequence = %d, want %d", chunk.Sequence, expectedSeq)
		}
		expectedSeq++
		if chunk.IsFinal {
			sawFinalMarker = true
		} else {
			dataChunks++
			totalBytes += len(chunk.Data)
		}
	}
	if err := <-stream.Err; err != nil {
		t.Fatalf("stream.Err = %v", err)
	}
	finalResp := <-stream.Final
	if finalResp == nil {
		t.Fatal("expected non-nil Final SpeechResponse")
	}
	if dataChunks == 0 || totalBytes == 0 {
		t.Fatalf("expected non-empty streamed chunks, got chunks=%d bytes=%d", dataChunks, totalBytes)
	}
	if !sawFinalMarker {
		t.Error("expected terminal IsFinal chunk")
	}
	if finalResp.Usage.OutputBytes != int64(totalBytes) {
		t.Errorf("Final.Usage.OutputBytes = %d, want %d", finalResp.Usage.OutputBytes, totalBytes)
	}
	t.Logf("StreamSynthesize ok: chunks=%d bytes=%d format=%s", dataChunks, totalBytes, finalResp.Audio.Format.String())
}

func TestElevenLabs_InvalidAPIKey_ReturnsUnauthorized(t *testing.T) {
	skipIfNoElevenLabsKey(t)

	badKey := "xi_definitely_invalid_test_key_12345"
	provider := elevenlabs.New(badKey, elevenlabs.WithTimeout(15*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := provider.Synthesize(ctx, &core.SpeechRequest{
		Model:   elevenlabs.ModelElevenFlashV25,
		VoiceID: elevenLabsTestVoiceID(),
		Text:    "Unauthorized test.",
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("Synthesize(invalid key) err = %v, want core.ErrUnauthorized", err)
	}
	if strings.Contains(err.Error(), badKey) {
		t.Errorf("error leaked API key: %v", err)
	}
}
