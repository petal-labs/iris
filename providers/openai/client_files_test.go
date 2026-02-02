package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestUploadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files" {
			t.Errorf("expected /v1/files, got %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart/form-data, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}

		purpose := r.FormValue("purpose")
		if purpose != "user_data" {
			t.Errorf("expected purpose 'user_data', got %q", purpose)
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("failed to get file: %v", err)
		}
		defer file.Close()

		if header.Filename != "test.txt" {
			t.Errorf("expected filename 'test.txt', got %q", header.Filename)
		}

		content, _ := io.ReadAll(file)
		if string(content) != "hello world" {
			t.Errorf("expected content 'hello world', got %q", content)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(File{
			ID:        "file-abc123",
			Object:    "file",
			Bytes:     11,
			CreatedAt: 1677610602,
			Filename:  "test.txt",
			Purpose:   FilePurposeUserData,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.UploadFile(context.Background(), &FileUploadRequest{
		File:     strings.NewReader("hello world"),
		Filename: "test.txt",
		Purpose:  FilePurposeUserData,
	})
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	if result.ID != "file-abc123" {
		t.Errorf("expected ID 'file-abc123', got %q", result.ID)
	}
	if result.Purpose != FilePurposeUserData {
		t.Errorf("expected Purpose 'user_data', got %q", result.Purpose)
	}
}

func TestUploadFileWithExpiration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}

		anchor := r.FormValue("expires_after[anchor]")
		seconds := r.FormValue("expires_after[seconds]")

		if anchor != "created_at" {
			t.Errorf("expected anchor 'created_at', got %q", anchor)
		}
		if seconds != "2592000" {
			t.Errorf("expected seconds '2592000', got %q", seconds)
		}

		w.Header().Set("Content-Type", "application/json")
		expiresAt := int64(1680202602)
		json.NewEncoder(w).Encode(File{
			ID:        "file-abc123",
			Object:    "file",
			Bytes:     11,
			CreatedAt: 1677610602,
			ExpiresAt: &expiresAt,
			Filename:  "test.txt",
			Purpose:   FilePurposeUserData,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.UploadFile(context.Background(), &FileUploadRequest{
		File:     strings.NewReader("hello world"),
		Filename: "test.txt",
		Purpose:  FilePurposeUserData,
		ExpiresAfter: &ExpiresAfter{
			Anchor:  "created_at",
			Seconds: 2592000,
		},
	})
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	if result.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestUploadFileError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid file purpose",
				"type":    "invalid_request_error",
				"code":    "invalid_purpose",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.UploadFile(context.Background(), &FileUploadRequest{
		File:     strings.NewReader("hello"),
		Filename: "test.txt",
		Purpose:  "invalid",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if !errors.Is(provErr, core.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", provErr.Err)
	}
}

func TestListFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files" {
			t.Errorf("expected /v1/files, got %s", r.URL.Path)
		}

		// Check query parameters
		if r.URL.Query().Get("purpose") != "user_data" {
			t.Errorf("expected purpose=user_data, got %s", r.URL.Query().Get("purpose"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(FileListResponse{
			Object: "list",
			Data: []File{
				{ID: "file-1", Filename: "a.txt", Purpose: FilePurposeUserData},
				{ID: "file-2", Filename: "b.txt", Purpose: FilePurposeUserData},
			},
			HasMore: false,
			FirstID: "file-1",
			LastID:  "file-2",
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	purpose := FilePurposeUserData
	limit := 10
	result, err := provider.ListFiles(context.Background(), &FileListRequest{
		Purpose: &purpose,
		Limit:   &limit,
	})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(result.Data) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Data))
	}
	if result.Data[0].ID != "file-1" {
		t.Errorf("expected file-1, got %s", result.Data[0].ID)
	}
}

func TestListFilesNilRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(FileListResponse{
			Object: "list",
			Data:   []File{},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.ListFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if result.Data == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestGetFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files/file-abc123" {
			t.Errorf("expected /v1/files/file-abc123, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(File{
			ID:        "file-abc123",
			Object:    "file",
			Bytes:     120000,
			CreatedAt: 1677610602,
			Filename:  "mydata.jsonl",
			Purpose:   FilePurposeFineTune,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	result, err := provider.GetFile(context.Background(), "file-abc123")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}

	if result.ID != "file-abc123" {
		t.Errorf("expected ID 'file-abc123', got %q", result.ID)
	}
	if result.Bytes != 120000 {
		t.Errorf("expected Bytes 120000, got %d", result.Bytes)
	}
}

func TestGetFileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "No such file",
				"type":    "invalid_request_error",
				"code":    "not_found",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.GetFile(context.Background(), "file-nonexistent")
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

func TestDownloadFile(t *testing.T) {
	content := []byte("file content here")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files/file-abc123/content" {
			t.Errorf("expected /v1/files/file-abc123/content, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(content)
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	reader, err := provider.DownloadFile(context.Background(), "file-abc123")
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	defer reader.Close()

	downloaded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}

	if !bytes.Equal(downloaded, content) {
		t.Errorf("expected %q, got %q", content, downloaded)
	}
}

func TestDownloadFileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "No such file",
				"code":    "not_found",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.DownloadFile(context.Background(), "file-nonexistent")
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

func TestDeleteFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files/file-abc123" {
			t.Errorf("expected /v1/files/file-abc123, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(FileDeleteResponse{
			ID:      "file-abc123",
			Object:  "file",
			Deleted: true,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	err := provider.DeleteFile(context.Background(), "file-abc123")
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
}

func TestDeleteFileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "No such file",
				"code":    "not_found",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	err := provider.DeleteFile(context.Background(), "file-nonexistent")
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
