package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestCreateEmbeddings(t *testing.T) {
	dimensions := 3
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEmbeddingRequest(t, r, dimensions)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"embeddings": []any{
				map[string]any{"values": []float32{0.1, 0.2, 0.3}},
				map[string]any{"values": []float32{0.4, 0.5, 0.6}},
			},
			"usageMetadata": map[string]any{"promptTokenCount": 7},
		})
	}))
	defer server.Close()

	metadata := map[string]string{"source": "test"}
	request := &core.EmbeddingRequest{
		Model:      ModelGeminiEmbedding001,
		Input:      []core.EmbeddingInput{{Text: "hello", ID: "first", Metadata: metadata}, {Text: "world", ID: "second"}},
		Dimensions: &dimensions,
		InputType:  core.InputTypeQuery,
	}
	provider := New("test-key", WithBaseURL(server.URL))
	response, err := provider.CreateEmbeddings(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if response.Model != ModelGeminiEmbedding001 {
		t.Errorf("Model = %q, want %q", response.Model, ModelGeminiEmbedding001)
	}
	if len(response.Vectors) != 2 {
		t.Fatalf("len(Vectors) = %d, want 2", len(response.Vectors))
	}
	if response.Vectors[0].Index != 0 || response.Vectors[0].ID != "first" {
		t.Errorf("Vectors[0] identity = (%d, %q), want (0, first)", response.Vectors[0].Index, response.Vectors[0].ID)
	}
	if got := response.Vectors[1].Vector; len(got) != 3 || got[2] != 0.6 {
		t.Errorf("Vectors[1].Vector = %v, want [0.4 0.5 0.6]", got)
	}
	if response.Usage.PromptTokens != 7 || response.Usage.TotalTokens != 7 {
		t.Errorf("Usage = %+v, want prompt/total tokens 7", response.Usage)
	}

	response.Vectors[0].Metadata["source"] = "changed"
	if metadata["source"] != "test" {
		t.Error("response metadata aliases request metadata")
	}
}

func TestCreateEmbeddingsOmitsUnsetOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Requests []map[string]json.RawMessage `json:"requests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Requests) != 1 {
			t.Fatalf("len(requests) = %d, want 1", len(body.Requests))
		}
		if _, ok := body.Requests[0]["taskType"]; ok {
			t.Error("taskType should be omitted when no input type is set")
		}
		if _, ok := body.Requests[0]["outputDimensionality"]; ok {
			t.Error("outputDimensionality should be omitted when dimensions are unset")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"embeddings":    []any{map[string]any{"values": []float32{0.1}}},
			"usageMetadata": map[string]any{"promptTokenCount": 1},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))
	_, err := provider.CreateEmbeddings(context.Background(), validGeminiEmbeddingRequest())
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}
}

func TestCreateEmbeddingsValidatesRequest(t *testing.T) {
	dimensions := 0
	truncate := true
	tests := []struct {
		name            string
		request         *core.EmbeddingRequest
		wantUnsupported bool
	}{
		{name: "nil request", request: nil},
		{name: "missing model", request: &core.EmbeddingRequest{Input: []core.EmbeddingInput{{Text: "hello"}}}},
		{name: "unsafe model", request: &core.EmbeddingRequest{Model: "models/../unsafe", Input: []core.EmbeddingInput{{Text: "hello"}}}},
		{name: "missing input", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001}},
		{name: "blank input", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001, Input: []core.EmbeddingInput{{Text: "  "}}}},
		{name: "invalid dimensions", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001, Input: []core.EmbeddingInput{{Text: "hello"}}, Dimensions: &dimensions}},
		{name: "invalid input type", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001, Input: []core.EmbeddingInput{{Text: "hello"}}, InputType: core.InputType("invalid")}},
		{name: "user identifier", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001, Input: []core.EmbeddingInput{{Text: "hello"}}, User: "user-123"}, wantUnsupported: true},
		{name: "truncation", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001, Input: []core.EmbeddingInput{{Text: "hello"}}, Truncation: &truncate}, wantUnsupported: true},
		{name: "base64 encoding", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001, Input: []core.EmbeddingInput{{Text: "hello"}}, EncodingFormat: core.EncodingFormatBase64}, wantUnsupported: true},
		{name: "int8 output", request: &core.EmbeddingRequest{Model: ModelGeminiEmbedding001, Input: []core.EmbeddingInput{{Text: "hello"}}, OutputDType: core.OutputDTypeInt8}, wantUnsupported: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
			defer server.Close()

			provider := New("test-key", WithBaseURL(server.URL))
			_, err := provider.CreateEmbeddings(context.Background(), tt.request)
			if err == nil {
				t.Fatal("CreateEmbeddings() should reject invalid request")
			}
			if tt.wantUnsupported && !errors.Is(err, core.ErrNotSupported) {
				t.Errorf("error = %v, want ErrNotSupported", err)
			}
			if hit {
				t.Error("invalid request should fail before HTTP")
			}
		})
	}
}

func TestCreateEmbeddingsRejectsNilContext(t *testing.T) {
	provider := New("test-key")
	//nolint:staticcheck // Intentionally verify that the public API rejects a nil context.
	_, err := provider.CreateEmbeddings(nil, validGeminiEmbeddingRequest())
	if err == nil || !strings.Contains(err.Error(), "context cannot be nil") {
		t.Fatalf("error = %v, want nil context error", err)
	}
}

func TestCreateEmbeddingsEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer server.Close()

	provider := New("", WithBaseURL(server.URL))
	_, err := provider.CreateEmbeddings(context.Background(), validGeminiEmbeddingRequest())
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("error = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("empty key should fail before HTTP")
	}
}

func TestCreateEmbeddingsNormalizesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 429, "message": "quota exhausted", "status": "RESOURCE_EXHAUSTED"},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))
	_, err := provider.CreateEmbeddings(context.Background(), validGeminiEmbeddingRequest())
	if !errors.Is(err, core.ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
}

func TestCreateEmbeddingsRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"embeddings":`))
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))
	_, err := provider.CreateEmbeddings(context.Background(), validGeminiEmbeddingRequest())
	if !errors.Is(err, core.ErrDecode) {
		t.Fatalf("error = %v, want ErrDecode", err)
	}
}

func TestCreateEmbeddingsRejectsMismatchedResponseCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"embeddings": []any{}})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))
	_, err := provider.CreateEmbeddings(context.Background(), validGeminiEmbeddingRequest())
	if err == nil || !strings.Contains(err.Error(), "response count") {
		t.Fatalf("error = %v, want response count mismatch", err)
	}
}

func TestCreateEmbeddingsHonorsProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(func() { go server.Close() })

	provider := New("test-key", WithBaseURL(server.URL), WithTimeout(20*time.Millisecond))
	_, err := provider.CreateEmbeddings(context.Background(), validGeminiEmbeddingRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
}

func validGeminiEmbeddingRequest() *core.EmbeddingRequest {
	return &core.EmbeddingRequest{
		Model: ModelGeminiEmbedding001,
		Input: []core.EmbeddingInput{{Text: "hello"}},
	}
}

func assertEmbeddingRequest(t *testing.T, r *http.Request, dimensions int) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", r.Method)
	}
	if r.URL.Path != "/v1beta/models/gemini-embedding-001:batchEmbedContents" {
		t.Errorf("path = %q, want Gemini batch embeddings path", r.URL.Path)
	}
	if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
		t.Errorf("x-goog-api-key = %q, want test-key", got)
	}
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body struct {
		Requests []struct {
			Model   string `json:"model"`
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			TaskType             string `json:"taskType"`
			OutputDimensionality int    `json:"outputDimensionality"`
		} `json:"requests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(body.Requests) != 2 {
		t.Fatalf("len(requests) = %d, want 2", len(body.Requests))
	}
	for i, request := range body.Requests {
		if request.Model != "models/gemini-embedding-001" {
			t.Errorf("requests[%d].model = %q", i, request.Model)
		}
		if request.TaskType != "RETRIEVAL_QUERY" {
			t.Errorf("requests[%d].taskType = %q, want RETRIEVAL_QUERY", i, request.TaskType)
		}
		if request.OutputDimensionality != dimensions {
			t.Errorf("requests[%d].outputDimensionality = %d, want %d", i, request.OutputDimensionality, dimensions)
		}
	}
	if got := body.Requests[0].Content.Parts[0].Text; got != "hello" {
		t.Errorf("first input = %q, want hello", got)
	}
}
