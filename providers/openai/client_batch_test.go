package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestCreateBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Track file upload and batch creation
		var uploadedContent string
		fileUploaded := false
		batchCreated := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/files" && r.Method == http.MethodPost:
				// File upload
				fileUploaded = true
				r.ParseMultipartForm(10 << 20)
				file, _, _ := r.FormFile("file")
				if file != nil {
					content, _ := io.ReadAll(file)
					uploadedContent = string(content)
					file.Close()
				}
				json.NewEncoder(w).Encode(File{
					ID:       "file-abc123",
					Filename: "batch_input.jsonl",
					Purpose:  FilePurposeBatch,
				})

			case r.URL.Path == "/batches" && r.Method == http.MethodPost:
				// Batch creation
				batchCreated = true
				var req openAIBatchCreateRequest
				json.NewDecoder(r.Body).Decode(&req)

				if req.InputFileID != "file-abc123" {
					t.Errorf("InputFileID = %q, want %q", req.InputFileID, "file-abc123")
				}

				json.NewEncoder(w).Encode(openAIBatch{
					ID:       "batch_xyz789",
					Status:   "validating",
					Endpoint: batchEndpoint,
				})

			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))

		requests := []core.BatchRequest{
			{
				CustomID: "req-1",
				Request: core.ChatRequest{
					Model: "gpt-4",
					Messages: []core.Message{
						{Role: core.RoleUser, Content: "Hello"},
					},
				},
			},
			{
				CustomID: "req-2",
				Request: core.ChatRequest{
					Model: "gpt-4",
					Messages: []core.Message{
						{Role: core.RoleUser, Content: "World"},
					},
				},
			},
		}

		batchID, err := p.CreateBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("CreateBatch() error: %v", err)
		}

		if !fileUploaded {
			t.Error("File was not uploaded")
		}
		if !batchCreated {
			t.Error("Batch was not created")
		}
		if batchID != "batch_xyz789" {
			t.Errorf("BatchID = %q, want %q", batchID, "batch_xyz789")
		}

		// Verify JSONL format
		lines := strings.Split(strings.TrimSpace(uploadedContent), "\n")
		if len(lines) != 2 {
			t.Errorf("Expected 2 lines in JSONL, got %d", len(lines))
		}

		var line1 openAIBatchRequestLine
		if err := json.Unmarshal([]byte(lines[0]), &line1); err != nil {
			t.Errorf("Failed to parse line 1: %v", err)
		}
		if line1.CustomID != "req-1" {
			t.Errorf("Line 1 CustomID = %q, want %q", line1.CustomID, "req-1")
		}
		if line1.Method != "POST" {
			t.Errorf("Line 1 Method = %q, want %q", line1.Method, "POST")
		}
	})

	t.Run("empty requests", func(t *testing.T) {
		p := New("test-key")
		_, err := p.CreateBatch(context.Background(), []core.BatchRequest{})
		if err == nil {
			t.Error("CreateBatch() should return error for empty requests")
		}
	})
}

func TestGetBatchStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		completedAt := int64(1700000100)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/batches/batch_123" || r.Method != http.MethodGet {
				http.NotFound(w, r)
				return
			}

			json.NewEncoder(w).Encode(openAIBatch{
				ID:           "batch_123",
				Status:       "completed",
				Endpoint:     batchEndpoint,
				CreatedAt:    1700000000,
				CompletedAt:  &completedAt,
				OutputFileID: "file-output",
				RequestCounts: openAIBatchCounts{
					Total:     10,
					Completed: 8,
					Failed:    2,
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))

		info, err := p.GetBatchStatus(context.Background(), "batch_123")
		if err != nil {
			t.Fatalf("GetBatchStatus() error: %v", err)
		}

		if info.ID != "batch_123" {
			t.Errorf("ID = %q, want %q", info.ID, "batch_123")
		}
		if info.Status != core.BatchStatusCompleted {
			t.Errorf("Status = %q, want %q", info.Status, core.BatchStatusCompleted)
		}
		if info.Total != 10 {
			t.Errorf("Total = %d, want %d", info.Total, 10)
		}
		if info.Completed != 8 {
			t.Errorf("Completed = %d, want %d", info.Completed, 8)
		}
		if info.Failed != 2 {
			t.Errorf("Failed = %d, want %d", info.Failed, 2)
		}
		if info.OutputFileID != "file-output" {
			t.Errorf("OutputFileID = %q, want %q", info.OutputFileID, "file-output")
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"message": "batch not found",
					"code":    "not_found",
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))

		_, err := p.GetBatchStatus(context.Background(), "nonexistent")
		if err != core.ErrBatchNotFound {
			t.Errorf("GetBatchStatus() error = %v, want ErrBatchNotFound", err)
		}
	})
}

func TestGetBatchResults(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// JSONL output content
		outputContent := `{"id":"resp_1","custom_id":"req-1","response":{"status_code":200,"request_id":"req_abc","body":{"id":"chatcmpl-1","model":"gpt-4","choices":[{"message":{"content":"Hello!"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}}}
{"id":"resp_2","custom_id":"req-2","response":{"status_code":200,"request_id":"req_def","body":{"id":"chatcmpl-2","model":"gpt-4","choices":[{"message":{"content":"World!"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}}}`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/batches/batch_123" && r.Method == http.MethodGet:
				json.NewEncoder(w).Encode(openAIBatch{
					ID:           "batch_123",
					Status:       "completed",
					OutputFileID: "file-output",
				})

			case r.URL.Path == "/files/file-output/content" && r.Method == http.MethodGet:
				w.Write([]byte(outputContent))

			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))

		results, err := p.GetBatchResults(context.Background(), "batch_123")
		if err != nil {
			t.Fatalf("GetBatchResults() error: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2", len(results))
		}

		// Check first result
		if results[0].CustomID != "req-1" {
			t.Errorf("results[0].CustomID = %q, want %q", results[0].CustomID, "req-1")
		}
		if !results[0].IsSuccess() {
			t.Error("results[0] should be success")
		}
		if results[0].Response.Output != "Hello!" {
			t.Errorf("results[0].Response.Output = %q, want %q", results[0].Response.Output, "Hello!")
		}

		// Check second result
		if results[1].CustomID != "req-2" {
			t.Errorf("results[1].CustomID = %q, want %q", results[1].CustomID, "req-2")
		}
		if results[1].Response.Output != "World!" {
			t.Errorf("results[1].Response.Output = %q, want %q", results[1].Response.Output, "World!")
		}
	})

	t.Run("with errors", func(t *testing.T) {
		outputContent := `{"id":"resp_1","custom_id":"req-1","error":{"code":"rate_limit_exceeded","message":"Too many requests"}}`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/batches/batch_123":
				json.NewEncoder(w).Encode(openAIBatch{
					ID:           "batch_123",
					Status:       "completed",
					OutputFileID: "file-output",
				})
			case "/files/file-output/content":
				w.Write([]byte(outputContent))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))

		results, err := p.GetBatchResults(context.Background(), "batch_123")
		if err != nil {
			t.Fatalf("GetBatchResults() error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(results))
		}

		if results[0].IsSuccess() {
			t.Error("results[0] should not be success")
		}
		if results[0].Error == nil {
			t.Fatal("results[0].Error should not be nil")
		}
		if results[0].Error.Code != "rate_limit_exceeded" {
			t.Errorf("Error.Code = %q, want %q", results[0].Error.Code, "rate_limit_exceeded")
		}
	})
}

func TestCancelBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cancelled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/batches/batch_123/cancel" && r.Method == http.MethodPost {
				cancelled = true
				json.NewEncoder(w).Encode(openAIBatch{
					ID:     "batch_123",
					Status: "cancelling",
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))

		err := p.CancelBatch(context.Background(), "batch_123")
		if err != nil {
			t.Fatalf("CancelBatch() error: %v", err)
		}
		if !cancelled {
			t.Error("Cancel endpoint was not called")
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{"message": "not found", "code": "not_found"},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))

		err := p.CancelBatch(context.Background(), "nonexistent")
		if err != core.ErrBatchNotFound {
			t.Errorf("CancelBatch() error = %v, want ErrBatchNotFound", err)
		}
	})
}

func TestListBatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/batches" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		// Check limit query param
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "5")
		}

		json.NewEncoder(w).Encode(openAIBatchListResponse{
			Object: "list",
			Data: []openAIBatch{
				{ID: "batch_1", Status: "completed"},
				{ID: "batch_2", Status: "in_progress"},
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	batches, err := p.ListBatches(context.Background(), 5)
	if err != nil {
		t.Fatalf("ListBatches() error: %v", err)
	}

	if len(batches) != 2 {
		t.Fatalf("len(batches) = %d, want 2", len(batches))
	}
	if batches[0].ID != "batch_1" {
		t.Errorf("batches[0].ID = %q, want %q", batches[0].ID, "batch_1")
	}
	if batches[0].Status != core.BatchStatusCompleted {
		t.Errorf("batches[0].Status = %q, want %q", batches[0].Status, core.BatchStatusCompleted)
	}
}

func TestMapBatchStatus(t *testing.T) {
	p := &OpenAI{}

	tests := []struct {
		input    string
		expected core.BatchStatus
	}{
		{"validating", core.BatchStatusPending},
		{"pending", core.BatchStatusPending},
		{"in_progress", core.BatchStatusInProgress},
		{"finalizing", core.BatchStatusInProgress},
		{"completed", core.BatchStatusCompleted},
		{"failed", core.BatchStatusFailed},
		{"cancelled", core.BatchStatusCancelled},
		{"cancelling", core.BatchStatusCancelled},
		{"expired", core.BatchStatusExpired},
		{"unknown", core.BatchStatusPending},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := p.mapBatchStatus(tc.input)
			if got != tc.expected {
				t.Errorf("mapBatchStatus(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestBuildBatchJSONL(t *testing.T) {
	p := &OpenAI{}

	requests := []core.BatchRequest{
		{
			CustomID: "req-1",
			Request: core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: core.RoleSystem, Content: "You are helpful"},
					{Role: core.RoleUser, Content: "Hello"},
				},
			},
		},
	}

	data, err := p.buildBatchJSONL(requests)
	if err != nil {
		t.Fatalf("buildBatchJSONL() error: %v", err)
	}

	var line openAIBatchRequestLine
	if err := json.Unmarshal(data[:len(data)-1], &line); err != nil { // -1 to remove newline
		t.Fatalf("Failed to parse JSONL: %v", err)
	}

	if line.CustomID != "req-1" {
		t.Errorf("CustomID = %q, want %q", line.CustomID, "req-1")
	}
	if line.Method != "POST" {
		t.Errorf("Method = %q, want %q", line.Method, "POST")
	}
	if line.URL != batchEndpoint {
		t.Errorf("URL = %q, want %q", line.URL, batchEndpoint)
	}
	if line.Body.Model != "gpt-4" {
		t.Errorf("Body.Model = %q, want %q", line.Body.Model, "gpt-4")
	}
	if len(line.Body.Messages) != 2 {
		t.Errorf("len(Body.Messages) = %d, want 2", len(line.Body.Messages))
	}
}
