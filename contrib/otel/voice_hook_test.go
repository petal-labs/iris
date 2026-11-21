package otel

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/petal-labs/iris/v2/core"
)

type recordingMeterProvider struct {
	noop.MeterProvider
	mu      sync.Mutex
	metrics map[string]int
}

func newRecordingMeterProvider() *recordingMeterProvider {
	return &recordingMeterProvider{metrics: make(map[string]int)}
}

func (p *recordingMeterProvider) record(name string) {
	p.mu.Lock()
	p.metrics[name]++
	p.mu.Unlock()
}

func (p *recordingMeterProvider) Meter(_ string, _ ...metric.MeterOption) metric.Meter {
	return &recordingMeter{provider: p}
}

type recordingMeter struct {
	noop.Meter
	provider *recordingMeterProvider
}

func (m *recordingMeter) Int64Counter(name string, _ ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return &recordingInt64Counter{name: name, provider: m.provider}, nil
}

func (m *recordingMeter) Float64Histogram(name string, _ ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return &recordingFloat64Histogram{name: name, provider: m.provider}, nil
}

type recordingInt64Counter struct {
	noop.Int64Counter
	name     string
	provider *recordingMeterProvider
}

func (c *recordingInt64Counter) Add(_ context.Context, _ int64, _ ...metric.AddOption) {
	c.provider.record(c.name)
}

type recordingFloat64Histogram struct {
	noop.Float64Histogram
	name     string
	provider *recordingMeterProvider
}

func (h *recordingFloat64Histogram) Record(_ context.Context, _ float64, _ ...metric.RecordOption) {
	h.provider.record(h.name)
}

func TestVoiceHookSpansMetricsAndPrivacy(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	mp := newRecordingMeterProvider()

	hook := NewVoiceHook(
		WithVoiceTracerProvider(tp),
		WithVoiceMeterProvider(mp),
		WithVoiceScopeName("iris.voice.test"),
		WithVoiceRecordError(true),
		WithVoiceAttributes(attribute.String("env", "test")),
	)

	// 1. Streaming synthesis request with first-chunk latency & usage.
	reqStart := time.Now().Add(-50 * time.Millisecond)
	ctx := hook.OnVoiceRequestStart(context.Background(), core.VoiceRequestStartEvent{
		Operation: core.VoiceOperationStreamSynthesize,
		Provider:  "elevenlabs",
		Model:     "eleven_flash_v2_5",
		VoiceID:   "voice-1",
		Format:    core.AudioFormatPCM16000,
		Start:     reqStart,
	})
	hook.OnVoiceFirstChunk(ctx, core.VoiceFirstChunkEvent{
		Operation:    core.VoiceOperationStreamSynthesize,
		Provider:     "elevenlabs",
		Model:        "eleven_flash_v2_5",
		Start:        reqStart,
		FirstChunkAt: reqStart.Add(15 * time.Millisecond),
		ChunkBytes:   4096,
	})
	hook.OnVoiceRequestEnd(ctx, core.VoiceRequestEndEvent{
		Operation:         core.VoiceOperationStreamSynthesize,
		Provider:          "elevenlabs",
		Model:             "eleven_flash_v2_5",
		Start:             reqStart,
		End:               time.Now(),
		FirstChunkLatency: 15 * time.Millisecond,
		OutputBytes:       16384,
		InputCharacters:   42,
		Chunks:            4,
	})

	// 2. Full-duplex voice session with turn + interruption + error.
	sessStart := time.Now().Add(-200 * time.Millisecond)
	sessCtx := hook.OnVoiceSessionStart(context.Background(), core.VoiceSessionStartEvent{
		SessionID: "sess-99",
		Provider:  "openai",
		Mode:      "pipeline",
		Model:     "gpt-5",
		Start:     sessStart,
	})
	hook.OnVoiceTurnEnd(sessCtx, core.VoiceTurnEndEvent{
		SessionID:          "sess-99",
		TurnID:             "turn-1",
		Provider:           "openai",
		Model:              "gpt-5",
		Start:              sessStart.Add(10 * time.Millisecond),
		End:                sessStart.Add(100 * time.Millisecond),
		FirstAudioLatency:  20 * time.Millisecond,
		OutputAudioBytes:   8192,
		AudioChunks:        2,
		ToolCalls:          1,
		Interrupted:        true,
		InterruptionReason: core.VoiceInterruptionBargeIn,
	})
	hook.OnVoiceSessionEnd(sessCtx, core.VoiceSessionEndEvent{
		SessionID:        "sess-99",
		Provider:         "openai",
		Mode:             "pipeline",
		Start:            sessStart,
		End:              time.Now(),
		Turns:            1,
		Interruptions:    1,
		ToolCalls:        1,
		InputAudioBytes:  3200,
		OutputAudioBytes: 8192,
		ErrorCount:       1,
		Err:              errors.New("upstream closed"),
	})

	// Verify spans.
	spans := sr.Ended()
	if len(spans) != 3 {
		t.Fatalf("ended spans = %d, want 3 (request, turn, session)", len(spans))
	}

	var sawReqSpan, sawTurnSpan, sawSessSpan bool
	for _, s := range spans {
		switch s.Name() {
		case "voice stream_synthesize":
			sawReqSpan = true
			if s.Status().Code != codes.Ok {
				t.Errorf("request span status = %v, want Ok", s.Status().Code)
			}
			if len(s.Events()) != 1 || s.Events()[0].Name != "first_chunk" {
				t.Errorf("expected first_chunk span event, got %+v", s.Events())
			}
		case "voice turn":
			sawTurnSpan = true
			assertAttribute(t, s.Attributes(), "iris.voice.turn_id", "turn-1")
			assertAttribute(t, s.Attributes(), "iris.voice.interruption_reason", "barge_in")
		case "voice session":
			sawSessSpan = true
			if s.Status().Code != codes.Error {
				t.Errorf("session span status = %v, want Error", s.Status().Code)
			}
		}

		// Privacy check: no audio/transcript/prompt attribute keys exist.
		for _, a := range s.Attributes() {
			k := string(a.Key)
			if strings.Contains(k, "transcript") || strings.Contains(k, "prompt") || strings.Contains(k, "audio.data") {
				t.Errorf("unexpected sensitive attribute key %q on span %q", k, s.Name())
			}
		}
	}
	if !sawReqSpan || !sawTurnSpan || !sawSessSpan {
		t.Fatalf("missing spans: req=%t turn=%t sess=%t", sawReqSpan, sawTurnSpan, sawSessSpan)
	}

	// Verify all expected OTel metrics were recorded.
	mp.mu.Lock()
	defer mp.mu.Unlock()
	for _, want := range []string{
		"iris.voice.requests",
		"iris.voice.request.duration_ms",
		"iris.voice.first_chunk.latency_ms",
		"iris.voice.sessions",
		"iris.voice.session.duration_ms",
		"iris.voice.turns",
		"iris.voice.interruptions",
		"iris.voice.audio.bytes",
		"iris.voice.input_characters",
		"iris.voice.errors",
	} {
		if mp.metrics[want] == 0 {
			t.Errorf("expected metric %q to be recorded; got %v", want, mp.metrics)
		}
	}
}
