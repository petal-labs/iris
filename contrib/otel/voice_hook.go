package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/petal-labs/iris/core"
)

// Compile-time check that VoiceHook implements core.VoiceTelemetryHook.
var _ core.VoiceTelemetryHook = (*VoiceHook)(nil)

// VoiceConfig configures [VoiceHook] tracing and metrics instrumentation.
type VoiceConfig struct {
	// ScopeName identifies the OTel tracer and meter instrumentation scope.
	// Defaults to "github.com/petal-labs/iris/voice".
	ScopeName string

	// TracerProvider supplies the OpenTelemetry tracer provider.
	// Defaults to otel.GetTracerProvider().
	TracerProvider trace.TracerProvider

	// MeterProvider supplies the OpenTelemetry meter provider.
	// Defaults to otel.GetMeterProvider().
	MeterProvider metric.MeterProvider

	// RecordError controls whether error details are attached to spans.
	// Defaults to true.
	RecordError bool

	// AdditionalAttributes appends static attributes to emitted spans and metrics.
	AdditionalAttributes []attribute.KeyValue
}

// VoiceOption configures a [VoiceHook].
type VoiceOption func(*VoiceConfig)

// WithVoiceTracerProvider sets the OpenTelemetry tracer provider for voice spans.
func WithVoiceTracerProvider(tp trace.TracerProvider) VoiceOption {
	return func(c *VoiceConfig) {
		c.TracerProvider = tp
	}
}

// WithVoiceMeterProvider sets the OpenTelemetry meter provider for voice metrics.
func WithVoiceMeterProvider(mp metric.MeterProvider) VoiceOption {
	return func(c *VoiceConfig) {
		c.MeterProvider = mp
	}
}

// WithVoiceScopeName sets the instrumentation scope name for voice spans and metrics.
func WithVoiceScopeName(name string) VoiceOption {
	return func(c *VoiceConfig) {
		c.ScopeName = name
	}
}

// WithVoiceRecordError controls whether errors are recorded on voice spans.
func WithVoiceRecordError(record bool) VoiceOption {
	return func(c *VoiceConfig) {
		c.RecordError = record
	}
}

// WithVoiceAttributes adds static attributes to all voice spans and metrics.
func WithVoiceAttributes(attrs ...attribute.KeyValue) VoiceOption {
	return func(c *VoiceConfig) {
		c.AdditionalAttributes = append(c.AdditionalAttributes, attrs...)
	}
}

// VoiceHook implements [core.VoiceTelemetryHook] with OpenTelemetry tracing
// spans and metrics for speech requests, first-chunk latency, voice sessions,
// and conversational turns.
//
// # Privacy
//
// VoiceHook records operational metadata only (provider, model, operation,
// timings, byte/character counts, turn/interruption/tool counts, error status).
// Raw audio bytes, transcripts, prompts, assistant text, and API keys are never
// recorded in spans, span events, or metric attributes.
type VoiceHook struct {
	tracer trace.Tracer
	config VoiceConfig

	requestsCounter   metric.Int64Counter
	errorsCounter     metric.Int64Counter
	requestDurationMS metric.Float64Histogram
	firstChunkMS      metric.Float64Histogram
	sessionsCounter   metric.Int64Counter
	sessionDurationMS metric.Float64Histogram
	turnsCounter      metric.Int64Counter
	interruptsCounter metric.Int64Counter
	audioBytesCounter metric.Int64Counter
	inputCharsCounter metric.Int64Counter
}

// NewVoiceHook creates a new OpenTelemetry [VoiceHook].
func NewVoiceHook(opts ...VoiceOption) *VoiceHook {
	cfg := VoiceConfig{
		ScopeName:   "github.com/petal-labs/iris/voice",
		RecordError: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	tp := cfg.TracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	mp := cfg.MeterProvider
	if mp == nil {
		mp = otel.GetMeterProvider()
	}

	meter := mp.Meter(cfg.ScopeName)
	reqCounter, _ := meter.Int64Counter("iris.voice.requests",
		metric.WithDescription("Total speech and voice requests completed"))
	errCounter, _ := meter.Int64Counter("iris.voice.errors",
		metric.WithDescription("Total speech and voice errors"))
	reqDur, _ := meter.Float64Histogram("iris.voice.request.duration_ms",
		metric.WithUnit("ms"),
		metric.WithDescription("End-to-end duration of speech requests in milliseconds"))
	firstChunk, _ := meter.Float64Histogram("iris.voice.first_chunk.latency_ms",
		metric.WithUnit("ms"),
		metric.WithDescription("Time to first audio chunk in milliseconds"))
	sessCounter, _ := meter.Int64Counter("iris.voice.sessions",
		metric.WithDescription("Total full-duplex voice sessions completed"))
	sessDur, _ := meter.Float64Histogram("iris.voice.session.duration_ms",
		metric.WithUnit("ms"),
		metric.WithDescription("Duration of full-duplex voice sessions in milliseconds"))
	turnsCounter, _ := meter.Int64Counter("iris.voice.turns",
		metric.WithDescription("Total conversational voice turns completed"))
	intrCounter, _ := meter.Int64Counter("iris.voice.interruptions",
		metric.WithDescription("Total interrupted conversational turns"))
	audioBytes, _ := meter.Int64Counter("iris.voice.audio.bytes",
		metric.WithUnit("By"),
		metric.WithDescription("Total audio bytes transferred"))
	inputChars, _ := meter.Int64Counter("iris.voice.input_characters",
		metric.WithDescription("Total input characters synthesized"))

	return &VoiceHook{
		tracer:            tp.Tracer(cfg.ScopeName),
		config:            cfg,
		requestsCounter:   reqCounter,
		errorsCounter:     errCounter,
		requestDurationMS: reqDur,
		firstChunkMS:      firstChunk,
		sessionsCounter:   sessCounter,
		sessionDurationMS: sessDur,
		turnsCounter:      turnsCounter,
		interruptsCounter: intrCounter,
		audioBytesCounter: audioBytes,
		inputCharsCounter: inputChars,
	}
}

// OnVoiceRequestStart starts a client span for a speech operation.
func (h *VoiceHook) OnVoiceRequestStart(ctx context.Context, e core.VoiceRequestStartEvent) context.Context {
	attrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		IrisVoiceOperation.String(string(e.Operation)),
	}
	if e.Model != "" {
		attrs = append(attrs, GenAIRequestModel.String(string(e.Model)))
	}
	if e.Format.Encoding != "" {
		attrs = append(attrs, IrisVoiceAudioEncoding.String(string(e.Format.Encoding)))
	}
	if e.Format.SampleRate > 0 {
		attrs = append(attrs, IrisVoiceAudioSampleRate.Int(e.Format.SampleRate))
	}
	attrs = append(attrs, h.config.AdditionalAttributes...)

	ctx, _ = h.tracer.Start(ctx, "voice "+string(e.Operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	return ctx
}

// OnVoiceFirstChunk records first-audio latency as a span event and histogram sample.
func (h *VoiceHook) OnVoiceFirstChunk(ctx context.Context, e core.VoiceFirstChunkEvent) {
	latencyMS := durationMS(e.FirstChunkLatency())

	attrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		IrisVoiceOperation.String(string(e.Operation)),
	}
	if e.Model != "" {
		attrs = append(attrs, GenAIRequestModel.String(string(e.Model)))
	}
	attrs = append(attrs, h.config.AdditionalAttributes...)

	h.firstChunkMS.Record(ctx, latencyMS, metric.WithAttributes(attrs...))

	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		eventAttrs := []attribute.KeyValue{
			IrisVoiceFirstChunkMS.Float64(latencyMS),
			IrisVoiceOutputBytes.Int(e.ChunkBytes),
		}
		if e.TurnID != "" {
			eventAttrs = append(eventAttrs, IrisVoiceTurnID.String(e.TurnID))
		}
		span.AddEvent("first_chunk", trace.WithAttributes(eventAttrs...))
	}
}

// OnVoiceRequestEnd finishes the speech request span and records usage/duration/error metrics.
func (h *VoiceHook) OnVoiceRequestEnd(ctx context.Context, e core.VoiceRequestEndEvent) {
	metricAttrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		IrisVoiceOperation.String(string(e.Operation)),
	}
	if e.Model != "" {
		metricAttrs = append(metricAttrs, GenAIRequestModel.String(string(e.Model)))
	}
	metricAttrs = append(metricAttrs, h.config.AdditionalAttributes...)

	h.requestsCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	h.requestDurationMS.Record(ctx, durationMS(e.Duration()), metric.WithAttributes(metricAttrs...))

	if e.InputBytes > 0 {
		inAttrs := append(append([]attribute.KeyValue{}, metricAttrs...), IrisVoiceAudioDirection.String("input"))
		h.audioBytesCounter.Add(ctx, e.InputBytes, metric.WithAttributes(inAttrs...))
	}
	if e.OutputBytes > 0 {
		outAttrs := append(append([]attribute.KeyValue{}, metricAttrs...), IrisVoiceAudioDirection.String("output"))
		h.audioBytesCounter.Add(ctx, e.OutputBytes, metric.WithAttributes(outAttrs...))
	}
	if e.InputCharacters > 0 {
		h.inputCharsCounter.Add(ctx, int64(e.InputCharacters), metric.WithAttributes(metricAttrs...))
	}
	if e.Err != nil {
		h.errorsCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	spanAttrs := []attribute.KeyValue{
		IrisVoiceInputBytes.Int64(e.InputBytes),
		IrisVoiceOutputBytes.Int64(e.OutputBytes),
		IrisVoiceInputCharacters.Int(e.InputCharacters),
		IrisVoiceAudioChunks.Int(e.Chunks),
	}
	if e.FirstChunkLatency > 0 {
		spanAttrs = append(spanAttrs, IrisVoiceFirstChunkMS.Float64(durationMS(e.FirstChunkLatency)))
	}
	span.SetAttributes(spanAttrs...)

	if e.Err != nil {
		if h.config.RecordError {
			span.RecordError(e.Err)
		}
		span.SetStatus(codes.Error, e.Err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// OnVoiceSessionStart opens a parent span for a full-duplex voice session.
func (h *VoiceHook) OnVoiceSessionStart(ctx context.Context, e core.VoiceSessionStartEvent) context.Context {
	attrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		IrisVoiceOperation.String(string(core.VoiceOperationVoiceSession)),
		IrisVoiceMode.String(e.Mode),
	}
	if e.SessionID != "" {
		attrs = append(attrs, IrisVoiceSessionID.String(e.SessionID))
	}
	if e.Model != "" {
		attrs = append(attrs, GenAIRequestModel.String(string(e.Model)))
	}
	attrs = append(attrs, h.config.AdditionalAttributes...)

	ctx, _ = h.tracer.Start(ctx, "voice session",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	return ctx
}

// OnVoiceTurnEnd records a child span and turn metrics for a conversational turn.
func (h *VoiceHook) OnVoiceTurnEnd(ctx context.Context, e core.VoiceTurnEndEvent) {
	metricAttrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		IrisVoiceOperation.String(string(core.VoiceOperationVoiceTurn)),
	}
	if e.Model != "" {
		metricAttrs = append(metricAttrs, GenAIRequestModel.String(string(e.Model)))
	}
	metricAttrs = append(metricAttrs, h.config.AdditionalAttributes...)

	h.turnsCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	if e.Interrupted {
		h.interruptsCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	}
	if e.Err != nil {
		h.errorsCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	}

	spanAttrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		IrisVoiceTurnID.String(e.TurnID),
		IrisVoiceOutputBytes.Int64(e.OutputAudioBytes),
		IrisVoiceAudioChunks.Int(e.AudioChunks),
		IrisVoiceToolCalls.Int(e.ToolCalls),
		IrisVoiceInterrupted.Bool(e.Interrupted),
	}
	if e.SessionID != "" {
		spanAttrs = append(spanAttrs, IrisVoiceSessionID.String(e.SessionID))
	}
	if e.FirstAudioLatency > 0 {
		spanAttrs = append(spanAttrs, IrisVoiceFirstChunkMS.Float64(durationMS(e.FirstAudioLatency)))
	}
	if e.InterruptionReason != "" {
		spanAttrs = append(spanAttrs, IrisVoiceInterruptionReason.String(string(e.InterruptionReason)))
	}
	spanAttrs = append(spanAttrs, h.config.AdditionalAttributes...)

	_, turnSpan := h.tracer.Start(ctx, "voice turn",
		trace.WithTimestamp(e.Start),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(spanAttrs...),
	)
	if e.Err != nil {
		if h.config.RecordError {
			turnSpan.RecordError(e.Err)
		}
		turnSpan.SetStatus(codes.Error, e.Err.Error())
	} else {
		turnSpan.SetStatus(codes.Ok, "")
	}
	turnSpan.End(trace.WithTimestamp(e.End))
}

// OnVoiceSessionEnd finishes the voice session span and records aggregate session metrics.
func (h *VoiceHook) OnVoiceSessionEnd(ctx context.Context, e core.VoiceSessionEndEvent) {
	metricAttrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		IrisVoiceMode.String(e.Mode),
	}
	metricAttrs = append(metricAttrs, h.config.AdditionalAttributes...)

	h.sessionsCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	h.sessionDurationMS.Record(ctx, durationMS(e.Duration()), metric.WithAttributes(metricAttrs...))
	if e.InputAudioBytes > 0 {
		inAttrs := append(append([]attribute.KeyValue{}, metricAttrs...), IrisVoiceAudioDirection.String("input"))
		h.audioBytesCounter.Add(ctx, e.InputAudioBytes, metric.WithAttributes(inAttrs...))
	}
	if e.OutputAudioBytes > 0 {
		outAttrs := append(append([]attribute.KeyValue{}, metricAttrs...), IrisVoiceAudioDirection.String("output"))
		h.audioBytesCounter.Add(ctx, e.OutputAudioBytes, metric.WithAttributes(outAttrs...))
	}
	if e.Err != nil {
		h.errorsCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	spanAttrs := []attribute.KeyValue{
		IrisVoiceTurns.Int(e.Turns),
		IrisVoiceInterruptions.Int(e.Interruptions),
		IrisVoiceToolCalls.Int(e.ToolCalls),
		IrisVoiceInputBytes.Int64(e.InputAudioBytes),
		IrisVoiceOutputBytes.Int64(e.OutputAudioBytes),
		IrisVoiceErrorCount.Int(e.ErrorCount),
	}
	if e.SessionID != "" {
		spanAttrs = append(spanAttrs, IrisVoiceSessionID.String(e.SessionID))
	}
	span.SetAttributes(spanAttrs...)

	if e.Err != nil {
		if h.config.RecordError {
			span.RecordError(e.Err)
		}
		span.SetStatus(codes.Error, e.Err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
