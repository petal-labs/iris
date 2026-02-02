package anthropic

import (
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

func TestBuildFilesHeaders(t *testing.T) {
	p := New("test-key")
	headers := p.buildFilesHeaders()

	if headers.Get("x-api-key") != "test-key" {
		t.Errorf("expected x-api-key 'test-key', got %q", headers.Get("x-api-key"))
	}
	if headers.Get("anthropic-version") != DefaultVersion {
		t.Errorf("expected anthropic-version %q, got %q", DefaultVersion, headers.Get("anthropic-version"))
	}
	if headers.Get("anthropic-beta") != DefaultFilesAPIBeta {
		t.Errorf("expected anthropic-beta %q, got %q", DefaultFilesAPIBeta, headers.Get("anthropic-beta"))
	}
}

func TestBuildFilesHeadersCustomBeta(t *testing.T) {
	p := New("test-key", WithFilesAPIBeta("custom-beta-version"))
	headers := p.buildFilesHeaders()

	if headers.Get("anthropic-beta") != "custom-beta-version" {
		t.Errorf("expected anthropic-beta 'custom-beta-version', got %q", headers.Get("anthropic-beta"))
	}
}

func TestBuildFilesHeadersPreservesCustomHeaders(t *testing.T) {
	p := New("test-key", WithHeader("X-Custom", "value"))
	headers := p.buildFilesHeaders()

	if headers.Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom 'value', got %q", headers.Get("X-Custom"))
	}
}

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
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key 'test-key', got %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-beta") != DefaultFilesAPIBeta {
			t.Errorf("expected anthropic-beta %q, got %s", DefaultFilesAPIBeta, r.Header.Get("anthropic-beta"))
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse form: %v", err)
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
			ID:           "file_011CNha8iCJcU1wXNR6q4V8w",
			Type:         "file",
			Filename:     "test.txt",
			MimeType:     "text/plain",
			SizeBytes:    11,
			CreatedAt:    "2025-04-14T12:00:00Z",
			Downloadable: false,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	result, err := provider.UploadFile(context.Background(), &FileUploadRequest{
		File:     strings.NewReader("hello world"),
		Filename: "test.txt",
	})
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	if result.ID != "file_011CNha8iCJcU1wXNR6q4V8w" {
		t.Errorf("expected ID 'file_011CNha8iCJcU1wXNR6q4V8w', got %q", result.ID)
	}
	if result.Downloadable != false {
		t.Errorf("expected Downloadable false, got %v", result.Downloadable)
	}
}

func TestGetFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files/file_011CNha8iCJcU1wXNR6q4V8w" {
			t.Errorf("expected /v1/files/file_011CNha8iCJcU1wXNR6q4V8w, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(File{
			ID:           "file_011CNha8iCJcU1wXNR6q4V8w",
			Type:         "file",
			Filename:     "test.pdf",
			MimeType:     "application/pdf",
			SizeBytes:    1024,
			CreatedAt:    "2025-04-14T12:00:00Z",
			Downloadable: false,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	result, err := provider.GetFile(context.Background(), "file_011CNha8iCJcU1wXNR6q4V8w")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}

	if result.ID != "file_011CNha8iCJcU1wXNR6q4V8w" {
		t.Errorf("expected ID 'file_011CNha8iCJcU1wXNR6q4V8w', got %q", result.ID)
	}
	if result.SizeBytes != 1024 {
		t.Errorf("expected SizeBytes 1024, got %d", result.SizeBytes)
	}
}

func TestGetFileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "not_found_error",
				"message": "File not found",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

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

func TestListFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files" {
			t.Errorf("expected /v1/files, got %s", r.URL.Path)
		}

		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("after_id") != "file_prev" {
			t.Errorf("expected after_id=file_prev, got %s", r.URL.Query().Get("after_id"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(FileListResponse{
			Data: []File{
				{ID: "file_1", Type: "file", Filename: "a.txt"},
				{ID: "file_2", Type: "file", Filename: "b.txt"},
			},
			FirstID: "file_1",
			LastID:  "file_2",
			HasMore: false,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	limit := 10
	afterID := "file_prev"
	result, err := provider.ListFiles(context.Background(), &FileListRequest{
		Limit:   &limit,
		AfterID: &afterID,
	})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(result.Data) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Data))
	}
	if result.Data[0].ID != "file_1" {
		t.Errorf("expected file_1, got %s", result.Data[0].ID)
	}
}

func TestListFilesNilRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(FileListResponse{
			Data: []File{},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	result, err := provider.ListFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if result.Data == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestListAllFiles(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			json.NewEncoder(w).Encode(FileListResponse{
				Data:    []File{{ID: "file_1"}, {ID: "file_2"}},
				FirstID: "file_1",
				LastID:  "file_2",
				HasMore: true,
			})
		} else {
			json.NewEncoder(w).Encode(FileListResponse{
				Data:    []File{{ID: "file_3"}},
				FirstID: "file_3",
				LastID:  "file_3",
				HasMore: false,
			})
		}
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	files, err := provider.ListAllFiles(context.Background())
	if err != nil {
		t.Fatalf("ListAllFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestDownloadFile(t *testing.T) {
	content := []byte("file content here")
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: GetFile for pre-check
			if r.URL.Path != "/v1/files/file_downloadable" {
				t.Errorf("expected /v1/files/file_downloadable, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(File{
				ID:           "file_downloadable",
				Type:         "file",
				Downloadable: true,
			})
		} else {
			// Second call: actual download
			if r.URL.Path != "/v1/files/file_downloadable/content" {
				t.Errorf("expected /v1/files/file_downloadable/content, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(content)
		}
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	reader, err := provider.DownloadFile(context.Background(), "file_downloadable")
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	defer reader.Close()

	downloaded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}

	if string(downloaded) != string(content) {
		t.Errorf("expected %q, got %q", content, downloaded)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (pre-check + download), got %d", callCount)
	}
}

func TestDownloadFileNotDownloadable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(File{
			ID:           "file_uploaded",
			Type:         "file",
			Downloadable: false,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	_, err := provider.DownloadFile(context.Background(), "file_uploaded")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrFileNotDownloadable) {
		t.Errorf("expected ErrFileNotDownloadable, got %v", err)
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Code != "file_not_downloadable" {
		t.Errorf("expected code 'file_not_downloadable', got %q", provErr.Code)
	}
}

func TestDeleteFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/files/file_011CNha8iCJcU1wXNR6q4V8w" {
			t.Errorf("expected /v1/files/file_011CNha8iCJcU1wXNR6q4V8w, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(FileDeleteResponse{
			ID:   "file_011CNha8iCJcU1wXNR6q4V8w",
			Type: "file_deleted",
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	err := provider.DeleteFile(context.Background(), "file_011CNha8iCJcU1wXNR6q4V8w")
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
}

func TestDeleteFileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "not_found_error",
				"message": "File not found",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

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
