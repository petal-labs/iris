package testing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/anthropic"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/providers/voyageai"
)

func TestRecordingProvider_DelegatesID(t *testing.T) {
	mock := NewMockProvider().WithID("test-provider")
	recorder := NewRecordingProvider(mock)

	if got := recorder.ID(); got != "test-provider" {
		t.Errorf("ID() = %q, want %q", got, "test-provider")
	}
}

func TestRecordingProvider_DelegatesModels(t *testing.T) {
	models := []core.ModelInfo{
		{ID: "model-1", DisplayName: "Model 1"},
	}
	mock := NewMockProvider().WithModels(models...)
	recorder := NewRecordingProvider(mock)

	got := recorder.Models()
	if len(got) != 1 {
		t.Errorf("Models() returned %d models, want 1", len(got))
	}
	if got[0].ID != "model-1" {
		t.Errorf("Models()[0].ID = %q, want %q", got[0].ID, "model-1")
	}
}

func TestRecordingProvider_DelegatesSupports(t *testing.T) {
	mock := NewMockProvider().WithFeatures(core.FeatureChat)
	recorder := NewRecordingProvider(mock)

	if !recorder.Supports(core.FeatureChat) {
		t.Error("Supports(FeatureChat) = false, want true")
	}
	if recorder.Supports(core.FeatureToolCalling) {
		t.Error("Supports(FeatureToolCalling) = true, want false")
	}
}

func TestRecordingProvider_Chat_RecordsCall(t *testing.T) {
	ctx := context.Background()
	expectedResp := core.ChatResponse{ID: "resp-1", Output: "Hello!"}
	mock := NewMockProvider(expectedResp)
	recorder := NewRecordingProvider(mock)

	req := &core.ChatRequest{
		Model:    "test-model",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
	}

	resp, err := recorder.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Output != "Hello!" {
		t.Errorf("Chat() Output = %q, want %q", resp.Output, "Hello!")
	}

	recordings := recorder.Recordings()
	if len(recordings) != 1 {
		t.Fatalf("Recordings() = %d, want 1", len(recordings))
	}

	rec := recordings[0]
	if rec.Method != "Chat" {
		t.Errorf("Recording.Method = %q, want %q", rec.Method, "Chat")
	}
	if rec.Request.Model != "test-model" {
		t.Errorf("Recording.Request.Model = %q, want %q", rec.Request.Model, "test-model")
	}
	if rec.Response.Output != "Hello!" {
		t.Errorf("Recording.Response.Output = %q, want %q", rec.Response.Output, "Hello!")
	}
	if rec.Error != nil {
		t.Errorf("Recording.Error = %v, want nil", rec.Error)
	}
}

func TestRecordingProvider_Chat_RecordsError(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")
	mock := NewMockProvider().WithError(testErr)
	recorder := NewRecordingProvider(mock)

	_, err := recorder.Chat(ctx, &core.ChatRequest{Model: "test"})
	if err != testErr {
		t.Errorf("Chat() error = %v, want %v", err, testErr)
	}

	recordings := recorder.Recordings()
	if len(recordings) != 1 {
		t.Fatalf("Recordings() = %d, want 1", len(recordings))
	}

	if recordings[0].Error != testErr {
		t.Errorf("Recording.Error = %v, want %v", recordings[0].Error, testErr)
	}
	if recordings[0].Response != nil {
		t.Errorf("Recording.Response = %v, want nil", recordings[0].Response)
	}
}

func TestRecordingProvider_Chat_RecordsTiming(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()
	recorder := NewRecordingProvider(mock)

	before := time.Now()
	recorder.Chat(ctx, &core.ChatRequest{Model: "test"})
	after := time.Now()

	rec := recorder.LastRecording()
	if rec == nil {
		t.Fatal("LastRecording() returned nil")
	}

	if rec.StartTime.Before(before) {
		t.Error("StartTime is before the call was made")
	}
	if rec.EndTime.After(after) {
		t.Error("EndTime is after the call returned")
	}
	if rec.Duration < 0 {
		t.Errorf("Duration = %v, want >= 0", rec.Duration)
	}
	if rec.EndTime.Sub(rec.StartTime) != rec.Duration {
		t.Errorf("Duration mismatch: EndTime-StartTime=%v, Duration=%v",
			rec.EndTime.Sub(rec.StartTime), rec.Duration)
	}
}

func TestRecordingProvider_StreamChat_RecordsCall(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider().
		WithStreamingResponse([]string{"Hello"}, nil)
	recorder := NewRecordingProvider(mock)
	req := &core.ChatRequest{
		Model:    "test",
		Messages: []core.Message{{Role: core.RoleUser, Content: "stream me"}},
	}

	stream, err := recorder.StreamChat(ctx, req)
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	req.Model = "mutated-after-call"

	// Drain stream
	for range stream.Ch {
	}
	<-stream.Final

	recordings := recorder.Recordings()
	if len(recordings) != 1 {
		t.Fatalf("Recordings() = %d, want 1", len(recordings))
	}

	rec := recordings[0]
	if rec.Method != "StreamChat" {
		t.Errorf("Recording.Method = %q, want %q", rec.Method, "StreamChat")
	}
	if rec.Request == nil || rec.Request.Model != "test" || rec.Request.Messages[0].Content != "stream me" {
		t.Errorf("Recording.Request = %#v, want cloned stream request", rec.Request)
	}
	// Response should be nil for streaming (captured separately)
	if rec.Response != nil {
		t.Errorf("Recording.Response = %v, want nil for streaming", rec.Response)
	}
}

func TestRecordingProvider_RecordingCount(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()
	recorder := NewRecordingProvider(mock)
	req := &core.ChatRequest{Model: "test"}

	if recorder.RecordingCount() != 0 {
		t.Errorf("RecordingCount() = %d, want 0", recorder.RecordingCount())
	}

	recorder.Chat(ctx, req)
	recorder.Chat(ctx, req)
	recorder.Chat(ctx, req)

	if recorder.RecordingCount() != 3 {
		t.Errorf("RecordingCount() = %d, want 3", recorder.RecordingCount())
	}
}

func TestRecordingProvider_LastRecording(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()
	recorder := NewRecordingProvider(mock)

	if recorder.LastRecording() != nil {
		t.Error("LastRecording() should be nil when no calls made")
	}

	recorder.Chat(ctx, &core.ChatRequest{Model: "first"})
	recorder.Chat(ctx, &core.ChatRequest{Model: "second"})

	last := recorder.LastRecording()
	if last == nil {
		t.Fatal("LastRecording() returned nil")
	}
	if last.Request.Model != "second" {
		t.Errorf("LastRecording().Request.Model = %q, want %q", last.Request.Model, "second")
	}
}

func TestRecordingProvider_Clear(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()
	recorder := NewRecordingProvider(mock)

	recorder.Chat(ctx, &core.ChatRequest{Model: "test"})
	recorder.Chat(ctx, &core.ChatRequest{Model: "test"})

	if recorder.RecordingCount() != 2 {
		t.Errorf("RecordingCount() = %d, want 2", recorder.RecordingCount())
	}

	recorder.Clear()

	if recorder.RecordingCount() != 0 {
		t.Errorf("RecordingCount() after Clear = %d, want 0", recorder.RecordingCount())
	}
}

func TestRecordingProvider_Underlying(t *testing.T) {
	mock := NewMockProvider()
	recorder := NewRecordingProvider(mock)

	if recorder.Underlying() != mock {
		t.Error("Underlying() did not return the wrapped provider")
	}
	if recorder.Unwrap() != mock {
		t.Error("Unwrap() did not return the wrapped provider")
	}
}

func TestRecordingProvider_PreservesOptionalInterfaces(t *testing.T) {
	t.Run("OpenAI capabilities", func(t *testing.T) {
		provider := openai.New("test-key")
		recorder := NewRecordingProvider(provider)

		if got, ok := core.AsContentPartSupporter(recorder); !ok || got != provider {
			t.Error("AsContentPartSupporter() did not unwrap the recording provider")
		}
		if got, ok := core.AsEmbeddingProvider(recorder); !ok || got != provider {
			t.Error("AsEmbeddingProvider() did not unwrap the recording provider")
		}
		if got, ok := core.AsImageGenerator(recorder); !ok || got != provider {
			t.Error("AsImageGenerator() did not unwrap the recording provider")
		}
		if got, ok := core.AsBatchProvider(recorder); !ok || got != provider {
			t.Error("AsBatchProvider() did not unwrap the recording provider")
		}
	})

	t.Run("Voyage AI capabilities through nested recorders", func(t *testing.T) {
		provider := voyageai.New("test-key")
		recorder := NewRecordingProvider(NewRecordingProvider(provider))

		if got, ok := core.AsEmbeddingProvider(recorder); !ok || got != provider {
			t.Error("AsEmbeddingProvider() did not unwrap nested recording providers")
		}
		if got, ok := core.AsContextualizedEmbeddingProvider(recorder); !ok || got != provider {
			t.Error("AsContextualizedEmbeddingProvider() did not unwrap nested recording providers")
		}
		if got, ok := core.AsReranker(recorder); !ok || got != provider {
			t.Error("AsReranker() did not unwrap nested recording providers")
		}
	})

	t.Run("Anthropic token counting", func(t *testing.T) {
		provider := anthropic.New("test-key")
		recorder := NewRecordingProvider(provider)

		if got, ok := core.AsTokenCounter(recorder); !ok || got != provider {
			t.Error("AsTokenCounter() did not unwrap the recording provider")
		}
	})

	t.Run("unsupported capability stays unsupported", func(t *testing.T) {
		recorder := NewRecordingProvider(NewMockProvider())
		if got, ok := core.AsEmbeddingProvider(recorder); ok || got != nil {
			t.Errorf("AsEmbeddingProvider() = (%v, %v), want (nil, false)", got, ok)
		}
	})
}

func TestRecordingProvider_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()
	recorder := NewRecordingProvider(mock)
	req := &core.ChatRequest{Model: "test"}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			recorder.Chat(ctx, req)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if recorder.RecordingCount() != 10 {
		t.Errorf("RecordingCount() = %d, want 10", recorder.RecordingCount())
	}
}

func TestRecordingProvider_RecordingsAreCopies(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()
	recorder := NewRecordingProvider(mock)

	recorder.Chat(ctx, &core.ChatRequest{Model: "test"})

	recordings1 := recorder.Recordings()
	recordings2 := recorder.Recordings()

	// Modify first slice
	if len(recordings1) > 0 {
		recordings1[0].Method = "Modified"
	}

	// Second slice should be unaffected
	if recordings2[0].Method == "Modified" {
		t.Error("Recordings() did not return a copy")
	}
}
