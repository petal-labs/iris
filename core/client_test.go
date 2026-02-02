package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockProvider is a test implementation of Provider.
type mockProvider struct {
	id          string
	chatFunc    func(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	streamFunc  func(ctx context.Context, req *ChatRequest) (*ChatStream, error)
	callCount   int
	lastRequest *ChatRequest
	mu          sync.Mutex
}

func (m *mockProvider) ID() string {
	return m.id
}

func (m *mockProvider) Models() []ModelInfo {
	return []ModelInfo{
		{ID: "mock-model", DisplayName: "Mock Model", Capabilities: []Feature{FeatureChat, FeatureChatStreaming}},
	}
}

func (m *mockProvider) Supports(feature Feature) bool {
	return feature == FeatureChat || feature == FeatureChatStreaming
}

func (m *mockProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	m.mu.Lock()
	m.callCount++
	m.lastRequest = req
	m.mu.Unlock()

	if m.chatFunc != nil {
		return m.chatFunc(ctx, req)
	}
	return &ChatResponse{
		ID:     "resp-1",
		Model:  req.Model,
		Output: "Hello!",
		Usage:  TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}, nil
}

func (m *mockProvider) StreamChat(ctx context.Context, req *ChatRequest) (*ChatStream, error) {
	m.mu.Lock()
	m.callCount++
	m.lastRequest = req
	m.mu.Unlock()

	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}

	ch := make(chan ChatChunk, 1)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	go func() {
		ch <- ChatChunk{Delta: "Hello"}
		close(ch)
		finalCh <- &ChatResponse{
			ID:     "resp-1",
			Model:  req.Model,
			Output: "Hello!",
			Usage:  TokenUsage{TotalTokens: 15},
		}
		close(finalCh)
		close(errCh)
	}()

	return &ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

// mockTelemetryHook records telemetry events for testing.
type mockTelemetryHook struct {
	startEvents []RequestStartEvent
	endEvents   []RequestEndEvent
	mu          sync.Mutex
}

func (h *mockTelemetryHook) OnRequestStart(e RequestStartEvent) {
	h.mu.Lock()
	h.startEvents = append(h.startEvents, e)
	h.mu.Unlock()
}

func (h *mockTelemetryHook) OnRequestEnd(e RequestEndEvent) {
	h.mu.Lock()
	h.endEvents = append(h.endEvents, e)
	h.mu.Unlock()
}

func TestNewClient(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.provider != p {
		t.Error("provider not set correctly")
	}
}

func TestNewClientWithOptions(t *testing.T) {
	p := &mockProvider{id: "test"}
	hook := &mockTelemetryHook{}
	retry := NewRetryPolicy(RetryConfig{MaxRetries: 5})

	c := NewClient(p,
		WithTelemetry(hook),
		WithRetryPolicy(retry),
	)

	if c.telemetry != hook {
		t.Error("telemetry hook not set")
	}
	if c.retry != retry {
		t.Error("retry policy not set")
	}
}

func TestChatBuilderFluentAPI(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	builder := c.Chat("gpt-4").
		System("You are helpful").
		User("Hello").
		Assistant("Hi there").
		User("How are you?").
		Temperature(0.7).
		MaxTokens(100)

	if builder.req.Model != "gpt-4" {
		t.Errorf("Model = %v, want gpt-4", builder.req.Model)
	}
	if len(builder.req.Messages) != 4 {
		t.Errorf("len(Messages) = %d, want 4", len(builder.req.Messages))
	}
	if *builder.req.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", *builder.req.Temperature)
	}
	if *builder.req.MaxTokens != 100 {
		t.Errorf("MaxTokens = %v, want 100", *builder.req.MaxTokens)
	}
}

func TestChatBuilderMessageOrder(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	builder := c.Chat("gpt-4").
		System("System").
		User("User1").
		Assistant("Assistant1").
		User("User2")

	expected := []struct {
		role    Role
		content string
	}{
		{RoleSystem, "System"},
		{RoleUser, "User1"},
		{RoleAssistant, "Assistant1"},
		{RoleUser, "User2"},
	}

	if len(builder.req.Messages) != len(expected) {
		t.Fatalf("len(Messages) = %d, want %d", len(builder.req.Messages), len(expected))
	}

	for i, exp := range expected {
		msg := builder.req.Messages[i]
		if msg.Role != exp.role {
			t.Errorf("Messages[%d].Role = %v, want %v", i, msg.Role, exp.role)
		}
		if msg.Content != exp.content {
			t.Errorf("Messages[%d].Content = %v, want %v", i, msg.Content, exp.content)
		}
	}
}

func TestGetResponseValidationModelRequired(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	_, err := c.Chat(""). // empty model
				User("Hello").
				GetResponse(context.Background())

	if !errors.Is(err, ErrModelRequired) {
		t.Errorf("err = %v, want ErrModelRequired", err)
	}
}

func TestGetResponseValidationNoMessages(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	_, err := c.Chat("gpt-4"). // no messages
					GetResponse(context.Background())

	if !errors.Is(err, ErrNoMessages) {
		t.Errorf("err = %v, want ErrNoMessages", err)
	}
}

func TestGetResponseSuccess(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	resp, err := c.Chat("gpt-4").
		User("Hello").
		GetResponse(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.Output != "Hello!" {
		t.Errorf("Output = %v, want Hello!", resp.Output)
	}
}

func TestGetResponseTelemetry(t *testing.T) {
	p := &mockProvider{id: "test-provider"}
	hook := &mockTelemetryHook{}
	c := NewClient(p, WithTelemetry(hook))

	_, err := c.Chat("gpt-4").
		User("Hello").
		GetResponse(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hook.startEvents) != 1 {
		t.Errorf("expected 1 start event, got %d", len(hook.startEvents))
	}
	if len(hook.endEvents) != 1 {
		t.Errorf("expected 1 end event, got %d", len(hook.endEvents))
	}

	if hook.startEvents[0].Provider != "test-provider" {
		t.Error("start event should have correct provider")
	}
	if hook.endEvents[0].Provider != "test-provider" {
		t.Error("end event should have correct provider")
	}
	if hook.endEvents[0].Err != nil {
		t.Error("end event should have nil error on success")
	}
}

func TestGetResponseRetryOnRetryableError(t *testing.T) {
	callCount := 0
	p := &mockProvider{
		id: "test",
		chatFunc: func(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
			callCount++
			if callCount < 3 {
				return nil, ErrNetwork // retryable
			}
			return &ChatResponse{Output: "Success"}, nil
		},
	}

	// Use fast retry for testing
	retry := NewRetryPolicy(RetryConfig{
		MaxRetries: 5,
		BaseDelay:  time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Jitter:     0,
	})
	c := NewClient(p, WithRetryPolicy(retry))

	resp, err := c.Chat("gpt-4").
		User("Hello").
		GetResponse(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3 (initial + 2 retries)", callCount)
	}
	if resp.Output != "Success" {
		t.Errorf("Output = %v, want Success", resp.Output)
	}
}

func TestGetResponseNoRetryOnNonRetryableError(t *testing.T) {
	callCount := 0
	p := &mockProvider{
		id: "test",
		chatFunc: func(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
			callCount++
			return nil, ErrUnauthorized // not retryable
		},
	}

	c := NewClient(p)

	_, err := c.Chat("gpt-4").
		User("Hello").
		GetResponse(context.Background())

	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (no retries)", callCount)
	}
}

func TestGetResponseContextCancellation(t *testing.T) {
	p := &mockProvider{
		id: "test",
		chatFunc: func(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
			return nil, ErrNetwork // would retry, but context cancelled
		},
	}

	retry := NewRetryPolicy(RetryConfig{
		MaxRetries: 5,
		BaseDelay:  time.Second, // long delay
	})
	c := NewClient(p, WithRetryPolicy(retry))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.Chat("gpt-4").
		User("Hello").
		GetResponse(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestStreamValidation(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	// No model
	_, err := c.Chat("").User("Hello").Stream(context.Background())
	if !errors.Is(err, ErrModelRequired) {
		t.Errorf("err = %v, want ErrModelRequired", err)
	}

	// No messages
	_, err = c.Chat("gpt-4").Stream(context.Background())
	if !errors.Is(err, ErrNoMessages) {
		t.Errorf("err = %v, want ErrNoMessages", err)
	}
}

func TestStreamSuccess(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	stream, err := c.Chat("gpt-4").
		User("Hello").
		Stream(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream == nil {
		t.Fatal("stream is nil")
	}

	// Read chunks
	for chunk := range stream.Ch {
		if chunk.Delta != "Hello" {
			t.Errorf("Delta = %v, want Hello", chunk.Delta)
		}
	}
}

func TestStreamTelemetry(t *testing.T) {
	p := &mockProvider{id: "test-provider"}
	hook := &mockTelemetryHook{}
	c := NewClient(p, WithTelemetry(hook))

	stream, err := c.Chat("gpt-4").
		User("Hello").
		Stream(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Start event should be immediate
	if len(hook.startEvents) != 1 {
		t.Errorf("expected 1 start event, got %d", len(hook.startEvents))
	}

	// Drain the stream to trigger end event
	for range stream.Ch {
	}
	<-stream.Final

	// Give goroutine time to emit end event
	time.Sleep(10 * time.Millisecond)

	hook.mu.Lock()
	endCount := len(hook.endEvents)
	hook.mu.Unlock()

	if endCount != 1 {
		t.Errorf("expected 1 end event, got %d", endCount)
	}
}

func TestClientConcurrentUse(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Chat("gpt-4").
				User("Hello").
				GetResponse(context.Background())
			if err != nil {
				t.Errorf("concurrent call failed: %v", err)
			}
		}()
	}
	wg.Wait()

	p.mu.Lock()
	count := p.callCount
	p.mu.Unlock()

	if count != 10 {
		t.Errorf("callCount = %d, want 10", count)
	}
}

func TestChatBuilderTools(t *testing.T) {
	p := &mockProvider{id: "test"}
	c := NewClient(p)

	// Create mock tools
	tool1 := &mockTool{name: "tool1"}
	tool2 := &mockTool{name: "tool2"}

	builder := c.Chat("gpt-4").
		User("Hello").
		Tools(tool1, tool2)

	if len(builder.req.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(builder.req.Tools))
	}
}

// mockTool is a test implementation of Tool.
type mockTool struct {
	name string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock tool" }

func TestImageGeneratorInterface(t *testing.T) {
	// Verify the interface is defined
	var _ ImageGenerator = (*mockImageGenerator)(nil)
}

type mockImageGenerator struct{}

func (m *mockImageGenerator) GenerateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error) {
	return nil, nil
}

func (m *mockImageGenerator) EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error) {
	return nil, nil
}

func (m *mockImageGenerator) StreamImage(ctx context.Context, req *ImageGenerateRequest) (*ImageStream, error) {
	return nil, nil
}

func TestFileSearchWithVectorStoreIDs(t *testing.T) {
	p := &mockProvider{id: "test"}
	client := NewClient(p)

	builder := client.Chat("gpt-4.1-mini").
		User("Search my docs").
		FileSearch("vs_abc123", "vs_def456")

	// Access the internal request to verify
	if builder.req.ToolResources == nil {
		t.Fatal("expected ToolResources to be set")
	}
	if builder.req.ToolResources.FileSearch == nil {
		t.Fatal("expected FileSearch resources to be set")
	}
	if len(builder.req.ToolResources.FileSearch.VectorStoreIDs) != 2 {
		t.Errorf("expected 2 vector store IDs, got %d", len(builder.req.ToolResources.FileSearch.VectorStoreIDs))
	}
	if builder.req.ToolResources.FileSearch.VectorStoreIDs[0] != "vs_abc123" {
		t.Errorf("expected vs_abc123, got %s", builder.req.ToolResources.FileSearch.VectorStoreIDs[0])
	}

	// Verify file_search tool was added
	found := false
	for _, tool := range builder.req.BuiltInTools {
		if tool.Type == "file_search" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file_search tool to be added")
	}
}

func TestFileSearchWithoutVectorStoreIDs(t *testing.T) {
	p := &mockProvider{id: "test"}
	client := NewClient(p)

	builder := client.Chat("gpt-4.1-mini").
		User("Search").
		FileSearch()

	// Should add the tool but no resources
	found := false
	for _, tool := range builder.req.BuiltInTools {
		if tool.Type == "file_search" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file_search tool to be added")
	}
	if builder.req.ToolResources != nil {
		t.Error("expected no ToolResources when no vector store IDs provided")
	}
}

func TestMessageBuilder(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserMultimodal().
		Text("What's in this image?").
		ImageURL("https://example.com/cat.jpg").
		Done()

	if len(builder.req.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(builder.req.Messages))
	}

	msg := builder.req.Messages[0]
	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want user", msg.Role)
	}
	if len(msg.Parts) != 2 {
		t.Fatalf("Parts length = %d, want 2", len(msg.Parts))
	}

	// Verify first part is text
	text, ok := msg.Parts[0].(*InputText)
	if !ok {
		t.Fatalf("Parts[0] is not *InputText, got %T", msg.Parts[0])
	}
	if text.Text != "What's in this image?" {
		t.Errorf("Text = %q, want %q", text.Text, "What's in this image?")
	}

	// Verify second part is image
	img, ok := msg.Parts[1].(*InputImage)
	if !ok {
		t.Fatalf("Parts[1] is not *InputImage, got %T", msg.Parts[1])
	}
	if img.ImageURL != "https://example.com/cat.jpg" {
		t.Errorf("ImageURL = %q, want %q", img.ImageURL, "https://example.com/cat.jpg")
	}
}

func TestMessageBuilderImageWithDetail(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserMultimodal().
		Text("Analyze this").
		ImageURLWithDetail("https://example.com/doc.jpg", ImageDetailHigh).
		Done()

	if len(builder.req.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(builder.req.Messages))
	}

	img, ok := builder.req.Messages[0].Parts[1].(*InputImage)
	if !ok {
		t.Fatalf("Parts[1] is not *InputImage, got %T", builder.req.Messages[0].Parts[1])
	}
	if img.Detail != ImageDetailHigh {
		t.Errorf("Detail = %q, want high", img.Detail)
	}
}

func TestMessageBuilderFile(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserMultimodal().
		Text("Summarize").
		FileID("file-abc123").
		Done()

	if len(builder.req.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(builder.req.Messages))
	}

	file, ok := builder.req.Messages[0].Parts[1].(*InputFile)
	if !ok {
		t.Fatalf("Parts[1] is not *InputFile, got %T", builder.req.Messages[0].Parts[1])
	}
	if file.FileID != "file-abc123" {
		t.Errorf("FileID = %q, want file-abc123", file.FileID)
	}
}

func TestMessageBuilderImageFileID(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserMultimodal().
		ImageFileID("file-img123").
		Done()

	if len(builder.req.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(builder.req.Messages))
	}

	img, ok := builder.req.Messages[0].Parts[0].(*InputImage)
	if !ok {
		t.Fatalf("Parts[0] is not *InputImage, got %T", builder.req.Messages[0].Parts[0])
	}
	if img.FileID != "file-img123" {
		t.Errorf("FileID = %q, want file-img123", img.FileID)
	}
}

func TestMessageBuilderImageFileIDWithDetail(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserMultimodal().
		ImageFileIDWithDetail("file-img456", ImageDetailLow).
		Done()

	img, ok := builder.req.Messages[0].Parts[0].(*InputImage)
	if !ok {
		t.Fatalf("Parts[0] is not *InputImage, got %T", builder.req.Messages[0].Parts[0])
	}
	if img.FileID != "file-img456" {
		t.Errorf("FileID = %q, want file-img456", img.FileID)
	}
	if img.Detail != ImageDetailLow {
		t.Errorf("Detail = %q, want low", img.Detail)
	}
}

func TestMessageBuilderFileURL(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserMultimodal().
		FileURL("https://example.com/document.pdf").
		Done()

	file, ok := builder.req.Messages[0].Parts[0].(*InputFile)
	if !ok {
		t.Fatalf("Parts[0] is not *InputFile, got %T", builder.req.Messages[0].Parts[0])
	}
	if file.FileURL != "https://example.com/document.pdf" {
		t.Errorf("FileURL = %q, want https://example.com/document.pdf", file.FileURL)
	}
}

func TestMessageBuilderFileBase64(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserMultimodal().
		FileBase64("report.pdf", "JVBERi0xLjQK").
		Done()

	file, ok := builder.req.Messages[0].Parts[0].(*InputFile)
	if !ok {
		t.Fatalf("Parts[0] is not *InputFile, got %T", builder.req.Messages[0].Parts[0])
	}
	if file.Filename != "report.pdf" {
		t.Errorf("Filename = %q, want report.pdf", file.Filename)
	}
	if file.FileData != "JVBERi0xLjQK" {
		t.Errorf("FileData = %q, want JVBERi0xLjQK", file.FileData)
	}
}

func TestMessageBuilderChaining(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	// Test that MessageBuilder chains correctly back to ChatBuilder
	builder := client.Chat("test-model").
		System("You are helpful").
		UserMultimodal().
		Text("Look at these").
		ImageURL("https://example.com/img1.jpg").
		ImageURLWithDetail("https://example.com/img2.jpg", ImageDetailHigh).
		Done().
		Temperature(0.7)

	if len(builder.req.Messages) != 2 {
		t.Fatalf("Messages length = %d, want 2", len(builder.req.Messages))
	}
	if builder.req.Messages[0].Role != RoleSystem {
		t.Errorf("Messages[0].Role = %q, want system", builder.req.Messages[0].Role)
	}
	if builder.req.Messages[1].Role != RoleUser {
		t.Errorf("Messages[1].Role = %q, want user", builder.req.Messages[1].Role)
	}
	if len(builder.req.Messages[1].Parts) != 3 {
		t.Fatalf("Messages[1].Parts length = %d, want 3", len(builder.req.Messages[1].Parts))
	}
	if builder.req.Temperature == nil || *builder.req.Temperature != 0.7 {
		t.Errorf("Temperature not set correctly")
	}
}

func TestUserWithImageURL(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserWithImageURL("Describe this", "https://example.com/img.jpg")

	req := builder.req
	if len(req.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(req.Messages))
	}

	msg := req.Messages[0]
	if len(msg.Parts) != 2 {
		t.Fatalf("Parts length = %d, want 2", len(msg.Parts))
	}

	text := msg.Parts[0].(*InputText)
	if text.Text != "Describe this" {
		t.Errorf("Text = %q, want %q", text.Text, "Describe this")
	}

	img := msg.Parts[1].(*InputImage)
	if img.ImageURL != "https://example.com/img.jpg" {
		t.Errorf("ImageURL = %q, want expected URL", img.ImageURL)
	}
}

func TestUserWithFileURL(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserWithFileURL("Summarize this", "https://example.com/doc.pdf")

	req := builder.req
	msg := req.Messages[0]

	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want user", msg.Role)
	}

	text := msg.Parts[0].(*InputText)
	if text.Text != "Summarize this" {
		t.Errorf("Text = %q, want %q", text.Text, "Summarize this")
	}

	file := msg.Parts[1].(*InputFile)
	if file.FileURL != "https://example.com/doc.pdf" {
		t.Errorf("FileURL = %q, want expected URL", file.FileURL)
	}
}

func TestUserWithImageFileID(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserWithImageFileID("Describe this", "file-img123")

	req := builder.req
	msg := req.Messages[0]

	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want user", msg.Role)
	}

	text := msg.Parts[0].(*InputText)
	if text.Text != "Describe this" {
		t.Errorf("Text = %q, want %q", text.Text, "Describe this")
	}

	img := msg.Parts[1].(*InputImage)
	if img.FileID != "file-img123" {
		t.Errorf("FileID = %q, want file-img123", img.FileID)
	}
}

func TestUserWithFileID(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	builder := client.Chat("test-model").
		UserWithFileID("Summarize this", "file-doc456")

	req := builder.req
	msg := req.Messages[0]

	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want user", msg.Role)
	}

	text := msg.Parts[0].(*InputText)
	if text.Text != "Summarize this" {
		t.Errorf("Text = %q, want %q", text.Text, "Summarize this")
	}

	file := msg.Parts[1].(*InputFile)
	if file.FileID != "file-doc456" {
		t.Errorf("FileID = %q, want file-doc456", file.FileID)
	}
}

func TestBackwardCompatibilitySimpleUser(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	// Old-style .User() should still work
	builder := client.Chat("test-model").
		User("Hello, world!")

	req := builder.req
	if len(req.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(req.Messages))
	}

	msg := req.Messages[0]
	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", msg.Content, "Hello, world!")
	}
	if len(msg.Parts) != 0 {
		t.Errorf("Parts should be empty for simple messages, got %d", len(msg.Parts))
	}
}

func TestValidateMultimodalMessage(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	// Message with Parts should be valid
	builder := client.Chat("test-model").
		UserMultimodal().
		Text("Hello").
		Done()

	err := builder.validate()
	if err != nil {
		t.Errorf("validate() = %v, want nil", err)
	}
}

func TestValidateEmptyMessage(t *testing.T) {
	provider := &mockProvider{id: "test"}
	client := NewClient(provider)

	// Direct manipulation to test edge case - message with neither Content nor Parts
	builder := client.Chat("test-model")
	builder.req.Messages = append(builder.req.Messages, Message{
		Role: RoleUser,
		// Both Content and Parts are empty
	})

	err := builder.validate()
	if err == nil {
		t.Error("validate() should fail for empty message")
	}
}
