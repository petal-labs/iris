package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestCreateVectorStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/vector_stores" {
			t.Errorf("expected /v1/vector_stores, got %s", r.URL.Path)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header, got %q", r.Header.Get("OpenAI-Beta"))
		}

		var req VectorStoreCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Name != "test-store" {
			t.Errorf("expected name 'test-store', got %q", req.Name)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStore{
			ID:        "vs_abc123",
			Object:    "vector_store",
			Name:      "test-store",
			Status:    VectorStoreStatusCompleted,
			CreatedAt: 1699061776,
			FileCounts: VectorStoreFileCounts{
				Total: 0,
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.CreateVectorStore(context.Background(), &VectorStoreCreateRequest{
		Name: "test-store",
	})
	if err != nil {
		t.Fatalf("CreateVectorStore failed: %v", err)
	}

	if result.ID != "vs_abc123" {
		t.Errorf("expected ID 'vs_abc123', got %q", result.ID)
	}
	if result.Name != "test-store" {
		t.Errorf("expected Name 'test-store', got %q", result.Name)
	}
}

func TestListVectorStores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStoreListResponse{
			Object: "list",
			Data: []VectorStore{
				{ID: "vs_1", Name: "Store 1", Status: VectorStoreStatusCompleted},
				{ID: "vs_2", Name: "Store 2", Status: VectorStoreStatusInProgress},
			},
			HasMore: false,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.ListVectorStores(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListVectorStores failed: %v", err)
	}

	if len(result.Data) != 2 {
		t.Errorf("expected 2 stores, got %d", len(result.Data))
	}
}

func TestGetVectorStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vector_stores/vs_abc123" {
			t.Errorf("expected /v1/vector_stores/vs_abc123, got %s", r.URL.Path)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header 'assistants=v2', got %q", r.Header.Get("OpenAI-Beta"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStore{
			ID:     "vs_abc123",
			Name:   "test-store",
			Status: VectorStoreStatusCompleted,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.GetVectorStore(context.Background(), "vs_abc123")
	if err != nil {
		t.Fatalf("GetVectorStore failed: %v", err)
	}

	if result.ID != "vs_abc123" {
		t.Errorf("expected ID 'vs_abc123', got %q", result.ID)
	}
}

func TestDeleteVectorStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/vector_stores/vs_abc123" {
			t.Errorf("expected /v1/vector_stores/vs_abc123, got %s", r.URL.Path)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header 'assistants=v2', got %q", r.Header.Get("OpenAI-Beta"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStoreDeleteResponse{
			ID:      "vs_abc123",
			Object:  "vector_store.deleted",
			Deleted: true,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	err := provider.DeleteVectorStore(context.Background(), "vs_abc123")
	if err != nil {
		t.Fatalf("DeleteVectorStore failed: %v", err)
	}
}

func TestGetVectorStoreNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "No such vector store",
				"code":    "not_found",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.GetVectorStore(context.Background(), "vs_nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if !errors.Is(provErr, core.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", provErr.Err)
	}
}

func TestAddFileToVectorStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/vector_stores/vs_abc123/files" {
			t.Errorf("expected /v1/vector_stores/vs_abc123/files, got %s", r.URL.Path)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header 'assistants=v2', got %q", r.Header.Get("OpenAI-Beta"))
		}

		var req VectorStoreFileAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.FileID != "file-xyz789" {
			t.Errorf("expected file_id 'file-xyz789', got %q", req.FileID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStoreFile{
			ID:            "file-xyz789",
			Object:        "vector_store.file",
			VectorStoreID: "vs_abc123",
			Status:        VectorStoreFileStatusInProgress,
			CreatedAt:     1699061776,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.AddFileToVectorStore(context.Background(), "vs_abc123", &VectorStoreFileAddRequest{
		FileID: "file-xyz789",
	})
	if err != nil {
		t.Fatalf("AddFileToVectorStore failed: %v", err)
	}

	if result.ID != "file-xyz789" {
		t.Errorf("expected ID 'file-xyz789', got %q", result.ID)
	}
	if result.Status != VectorStoreFileStatusInProgress {
		t.Errorf("expected Status 'in_progress', got %q", result.Status)
	}
}

func TestListVectorStoreFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/vector_stores/vs_abc123/files" {
			t.Errorf("expected /v1/vector_stores/vs_abc123/files, got %s", r.URL.Path)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header 'assistants=v2', got %q", r.Header.Get("OpenAI-Beta"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStoreFileListResponse{
			Object: "list",
			Data: []VectorStoreFile{
				{ID: "file-1", VectorStoreID: "vs_abc123", Status: VectorStoreFileStatusCompleted},
				{ID: "file-2", VectorStoreID: "vs_abc123", Status: VectorStoreFileStatusCompleted},
			},
			HasMore: false,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.ListVectorStoreFiles(context.Background(), "vs_abc123", nil)
	if err != nil {
		t.Fatalf("ListVectorStoreFiles failed: %v", err)
	}

	if len(result.Data) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Data))
	}
}

func TestGetVectorStoreFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vector_stores/vs_abc123/files/file-xyz789" {
			t.Errorf("expected /v1/vector_stores/vs_abc123/files/file-xyz789, got %s", r.URL.Path)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header 'assistants=v2', got %q", r.Header.Get("OpenAI-Beta"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStoreFile{
			ID:            "file-xyz789",
			VectorStoreID: "vs_abc123",
			Status:        VectorStoreFileStatusCompleted,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.GetVectorStoreFile(context.Background(), "vs_abc123", "file-xyz789")
	if err != nil {
		t.Fatalf("GetVectorStoreFile failed: %v", err)
	}

	if result.ID != "file-xyz789" {
		t.Errorf("expected ID 'file-xyz789', got %q", result.ID)
	}
}

func TestDeleteVectorStoreFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/vector_stores/vs_abc123/files/file-xyz789" {
			t.Errorf("expected /v1/vector_stores/vs_abc123/files/file-xyz789, got %s", r.URL.Path)
		}
		if r.Header.Get("OpenAI-Beta") != "assistants=v2" {
			t.Errorf("expected OpenAI-Beta header 'assistants=v2', got %q", r.Header.Get("OpenAI-Beta"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStoreFileDeleteResponse{
			ID:      "file-xyz789",
			Object:  "vector_store.file.deleted",
			Deleted: true,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	err := provider.DeleteVectorStoreFile(context.Background(), "vs_abc123", "file-xyz789")
	if err != nil {
		t.Fatalf("DeleteVectorStoreFile failed: %v", err)
	}
}

func TestPollVectorStoreUntilReady(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)

		status := VectorStoreStatusInProgress
		if count >= 3 {
			status = VectorStoreStatusCompleted
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStore{
			ID:     "vs_abc123",
			Name:   "test-store",
			Status: status,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := provider.PollVectorStoreUntilReady(ctx, "vs_abc123", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("PollVectorStoreUntilReady failed: %v", err)
	}

	if result.Status != VectorStoreStatusCompleted {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 calls, got %d", callCount)
	}
}

func TestPollVectorStoreUntilReadyAlreadyComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStore{
			ID:     "vs_abc123",
			Status: VectorStoreStatusCompleted,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.PollVectorStoreUntilReady(context.Background(), "vs_abc123", time.Second)
	if err != nil {
		t.Fatalf("PollVectorStoreUntilReady failed: %v", err)
	}

	if result.Status != VectorStoreStatusCompleted {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
}

func TestPollVectorStoreUntilReadyExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStore{
			ID:     "vs_abc123",
			Status: VectorStoreStatusExpired,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.PollVectorStoreUntilReady(context.Background(), "vs_abc123", time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Code != "vector_store_expired" {
		t.Errorf("expected code 'vector_store_expired', got %q", provErr.Code)
	}
}

func TestPollVectorStoreUntilReadyContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VectorStore{
			ID:     "vs_abc123",
			Status: VectorStoreStatusInProgress,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := provider.PollVectorStoreUntilReady(ctx, "vs_abc123", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}
