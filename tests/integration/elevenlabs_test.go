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

func TestElevenLabs_VoiceDiscovery_And_Settings(t *testing.T) {
	skipIfNoElevenLabsKey(t)

	provider := elevenlabs.New(getElevenLabsKey(t), elevenlabs.WithTimeout(20*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := provider.ListVoices(ctx, &elevenlabs.VoiceListRequest{PageSize: 5})
	if err != nil {
		t.Fatalf("ListVoices() error = %v", err)
	}
	if len(page.Voices) == 0 {
		t.Fatal("ListVoices() returned 0 voices")
	}

	first := page.Voices[0]
	if first.VoiceID == "" || first.Name == "" {
		t.Errorf("first voice missing ID or Name: %+v", first)
	}

	voice, err := provider.GetVoice(ctx, first.VoiceID)
	if err != nil {
		t.Fatalf("GetVoice(%q) error = %v", first.VoiceID, err)
	}
	if voice.VoiceID != first.VoiceID {
		t.Errorf("GetVoice().VoiceID = %q, want %q", voice.VoiceID, first.VoiceID)
	}

	settings, err := provider.GetVoiceSettings(ctx, first.VoiceID)
	if err != nil {
		t.Fatalf("GetVoiceSettings(%q) error = %v", first.VoiceID, err)
	}
	if err := settings.Validate(); err != nil {
		t.Errorf("GetVoiceSettings() returned invalid settings: %v", err)
	}

	defaults, err := provider.GetDefaultVoiceSettings(ctx)
	if err != nil {
		t.Fatalf("GetDefaultVoiceSettings() error = %v", err)
	}
	if err := defaults.Validate(); err != nil {
		t.Errorf("GetDefaultVoiceSettings() returned invalid settings: %v", err)
	}

	t.Logf("VoiceDiscovery ok: page_count=%d has_more=%t total=%d first_voice=%q category=%q",
		len(page.Voices), page.HasMore, page.TotalCount, voice.Name, voice.Category)
}

func TestElevenLabs_RealtimeScribeSTT(t *testing.T) {
	skipIfNoElevenLabsKey(t)

	provider := elevenlabs.New(getElevenLabsKey(t), elevenlabs.WithTimeout(30*time.Second))
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Synthesize short 16kHz PCM audio to stream into real-time Scribe STT.
	ttsResp, err := client.Synthesize(ctx, &core.SpeechRequest{
		Model:   elevenlabs.ModelElevenFlashV25,
		VoiceID: elevenLabsTestVoiceID(),
		Text:    "Hello Iris realtime.",
		Format:  core.AudioFormatPCM16000,
	})
	if err != nil {
		t.Fatalf("Synthesize(PCM16000) error = %v", err)
	}

	sess, err := client.StartRealtimeTranscription(ctx, core.RealtimeTranscriptionConfig{
		Model:             elevenlabs.ModelScribeV2Realtime,
		Format:            core.AudioFormatPCM16000,
		CommitStrategy:    core.CommitStrategyManual,
		IncludeTimestamps: true,
	})
	if err != nil {
		t.Fatalf("StartRealtimeTranscription() error = %v", err)
	}
	defer sess.Close()

	// Stream audio in 8 KiB chunks (even byte boundaries for 16-bit PCM).
	const chunkSize = 8192
	pcm := ttsResp.Audio.Data
	for offset, seq := 0, int64(0); offset < len(pcm); offset, seq = offset+chunkSize, seq+1 {
		end := offset + chunkSize
		if end > len(pcm) {
			end = len(pcm)
		}
		if err := sess.SendAudio(ctx, core.AudioChunk{
			Data:     pcm[offset:end],
			Format:   core.AudioFormatPCM16000,
			Sequence: seq,
		}); err != nil {
			t.Fatalf("SendAudio(seq=%d) error = %v", seq, err)
		}
	}
	// Send 0.5s of trailing silence then commit so Scribe flushes the utterance cleanly.
	silence := make([]byte, 16000)
	if err := sess.SendAudio(ctx, core.AudioChunk{
		Data:   silence,
		Format: core.AudioFormatPCM16000,
	}); err != nil {
		t.Fatalf("SendAudio(silence) error = %v", err)
	}
	if err := sess.Commit(ctx); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	var sawStarted, sawCommitted bool
	waitDeadline := time.After(15 * time.Second)
	for !sawCommitted {
		select {
		case evt, ok := <-sess.Events():
			if !ok {
				if err := <-sess.Err(); err != nil {
					t.Fatalf("realtime STT session error: %v", err)
				}
				t.Fatal("realtime STT events channel closed before committed transcript")
			}
			switch evt.Type {
			case core.RealtimeTranscriptEventSessionStarted:
				sawStarted = true
			case core.RealtimeTranscriptEventCommitted:
				if evt.Transcript != nil && strings.TrimSpace(evt.Transcript.Text) != "" {
					sawCommitted = true
					t.Logf("RealtimeSTT committed: %s", evt.String())
				}
			}
		case err := <-sess.Err():
			if err != nil {
				t.Fatalf("realtime STT error: %v", err)
			}
		case <-waitDeadline:
			t.Fatalf("timed out waiting for committed transcript (sawStarted=%t)", sawStarted)
		}
	}
	if !sawStarted {
		t.Error("expected session_started event before committed transcript")
	}
}
