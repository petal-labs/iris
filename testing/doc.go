// Package testing provides test utilities for the Iris SDK.
//
// This package contains mock implementations and test helpers that enable
// unit testing of code that uses the Iris SDK without making real API calls.
//
// # Mock Provider
//
// MockProvider implements core.Provider and allows you to define canned responses:
//
//	provider := testing.NewMockProvider(
//		core.ChatResponse{Output: "Hello!"},
//		core.ChatResponse{Output: "How can I help?"},
//	)
//	client := core.NewClient(provider)
//
//	// First call returns "Hello!"
//	resp, _ := client.Chat("gpt-4o").User("Hi").GetResponse(ctx)
//
//	// Second call returns "How can I help?"
//	resp, _ = client.Chat("gpt-4o").User("What's up?").GetResponse(ctx)
//
// A bare mock advertises chat, streaming, tool calling, structured output, and
// web search so chat-builder capability gates do not obstruct ordinary tests.
// The first WithFeatures call replaces those defaults; later calls add to the
// customized set.
//
// # Error Simulation
//
// Queue errors to test error handling:
//
//	provider := testing.NewMockProvider().
//		WithError(core.ErrRateLimited).
//		WithResponse(core.ChatResponse{Output: "Success after retry"})
//
// # Call Recording
//
// Verify that expected calls were made:
//
//	provider := testing.NewMockProvider(core.ChatResponse{Output: "test"})
//	client := core.NewClient(provider)
//
//	client.Chat("gpt-4o").User("Hello").GetResponse(ctx)
//
//	calls := provider.Calls()
//	if len(calls) != 1 {
//		t.Errorf("expected 1 call, got %d", len(calls))
//	}
//	if calls[0].Request.Messages[0].Content != "Hello" {
//		t.Error("unexpected message content")
//	}
//
// # Streaming Support
//
// Mock streaming responses:
//
//	provider := testing.NewMockProvider().
//		WithStreamingResponse([]string{"Hello", " ", "world", "!"}, nil)
//
// # Recording Provider
//
// Wrap a real provider to record calls for replay testing:
//
//	realProvider := openai.New(apiKey)
//	recorder := testing.NewRecordingProvider(realProvider)
//	client := core.NewClient(recorder)
//
//	// Make calls...
//
//	// Save recordings for later replay
//	recordings := recorder.Recordings()
//
// RecordingProvider implements core.ProviderUnwrapper, so optional operations
// remain discoverable through helpers such as core.AsEmbeddingProvider.
package testing
