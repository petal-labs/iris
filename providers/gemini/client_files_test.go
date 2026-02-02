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

func TestUploadFile(t *testing.T) {
	var initiateReceived, uploadReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/upload/v1beta/files"):
			initiateReceived = true
			// Verify headers
			if r.Header.Get("X-Goog-Upload-Protocol") != "resumable" {
				t.Errorf("Missing resumable protocol header")
			}
			if r.Header.Get("X-Goog-Upload-Command") != "start" {
				t.Errorf("Missing start command header")
			}
			// Return upload URL
			w.Header().Set("X-Goog-Upload-URL", "http://"+r.Host+"/upload-target")
			w.WriteHeader(http.StatusOK)

		case strings.HasPrefix(r.URL.Path, "/upload-target"):
			uploadReceived = true
			// Verify upload headers
			if r.Header.Get("X-Goog-Upload-Command") != "upload, finalize" {
				t.Errorf("Missing finalize command header")
			}
			// Return file response
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(fileUploadResponse{
				File: File{
					Name:     "files/test-123",
					MimeType: "text/plain",
					State:    FileStateActive,
				},
			})
		}
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	file, err := provider.UploadFile(context.Background(), &FileUploadRequest{
		File:        strings.NewReader("test content"),
		DisplayName: "test.txt",
		MimeType:    "text/plain",
	})
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	if !initiateReceived {
		t.Error("Initiate request not received")
	}
	if !uploadReceived {
		t.Error("Upload request not received")
	}
	if file.Name != "files/test-123" {
		t.Errorf("File.Name = %q, want files/test-123", file.Name)
	}
}

func TestGetFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/files/test-123") {
			t.Errorf("Path = %q, want suffix /files/test-123", r.URL.Path)
		}

		json.NewEncoder(w).Encode(File{
			Name:     "files/test-123",
			MimeType: "text/plain",
			State:    FileStateActive,
			URI:      "https://example.com/files/test-123",
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	file, err := provider.GetFile(context.Background(), "files/test-123")
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}

	if file.Name != "files/test-123" {
		t.Errorf("File.Name = %q, want files/test-123", file.Name)
	}
	if file.State != FileStateActive {
		t.Errorf("File.State = %q, want ACTIVE", file.State)
	}
}

func TestGetFile_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(geminiErrorResponse{
			Error: geminiError{
				Code:    404,
				Message: "File not found",
				Status:  "NOT_FOUND",
			},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	_, err := provider.GetFile(context.Background(), "files/nonexistent")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Expected ProviderError, got %T", err)
	}
}

func TestListFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/files") {
			t.Errorf("Path = %q, want suffix /files", r.URL.Path)
		}

		// Verify query params
		if r.URL.Query().Get("pageSize") != "10" {
			t.Errorf("pageSize = %q, want 10", r.URL.Query().Get("pageSize"))
		}

		json.NewEncoder(w).Encode(FileListResponse{
			Files: []File{
				{Name: "files/1", State: FileStateActive},
				{Name: "files/2", State: FileStateProcessing},
			},
			NextPageToken: "next-token",
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	resp, err := provider.ListFiles(context.Background(), &FileListRequest{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	if len(resp.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(resp.Files))
	}
	if resp.NextPageToken != "next-token" {
		t.Errorf("NextPageToken = %q, want next-token", resp.NextPageToken)
	}
}

func TestListFiles_WithPageToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageToken") != "some-token" {
			t.Errorf("pageToken = %q, want some-token", r.URL.Query().Get("pageToken"))
		}

		json.NewEncoder(w).Encode(FileListResponse{
			Files: []File{{Name: "files/3", State: FileStateActive}},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	resp, err := provider.ListFiles(context.Background(), &FileListRequest{
		PageToken: "some-token",
	})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	if len(resp.Files) != 1 {
		t.Errorf("len(Files) = %d, want 1", len(resp.Files))
	}
}

func TestListAllFiles(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		pageToken := r.URL.Query().Get("pageToken")

		switch pageToken {
		case "":
			// First page
			json.NewEncoder(w).Encode(FileListResponse{
				Files:         []File{{Name: "files/1"}, {Name: "files/2"}},
				NextPageToken: "page2",
			})
		case "page2":
			// Second page
			json.NewEncoder(w).Encode(FileListResponse{
				Files:         []File{{Name: "files/3"}},
				NextPageToken: "",
			})
		}
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	files, err := provider.ListAllFiles(context.Background())
	if err != nil {
		t.Fatalf("ListAllFiles() error = %v", err)
	}

	if len(files) != 3 {
		t.Errorf("len(files) = %d, want 3", len(files))
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (pagination)", callCount)
	}
}

func TestDeleteFile(t *testing.T) {
	deleteReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/files/test-123") {
			t.Errorf("Path = %q, want suffix /files/test-123", r.URL.Path)
		}
		deleteReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	err := provider.DeleteFile(context.Background(), "files/test-123")
	if err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}

	if !deleteReceived {
		t.Error("Delete request not received")
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(geminiErrorResponse{
			Error: geminiError{Code: 404, Message: "Not found", Status: "NOT_FOUND"},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	err := provider.DeleteFile(context.Background(), "files/nonexistent")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestWaitForFileActive_AlreadyActive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(File{
			Name:  "files/test-123",
			State: FileStateActive,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	file, err := provider.WaitForFileActive(ctx, "files/test-123")
	if err != nil {
		t.Fatalf("WaitForFileActive() error = %v", err)
	}

	if file.State != FileStateActive {
		t.Errorf("State = %q, want ACTIVE", file.State)
	}
}

func TestWaitForFileActive_BecomesActive(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		state := FileStateProcessing
		if callCount >= 2 {
			state = FileStateActive
		}
		json.NewEncoder(w).Encode(File{
			Name:  "files/test-123",
			State: state,
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	file, err := provider.WaitForFileActive(ctx, "files/test-123")
	if err != nil {
		t.Fatalf("WaitForFileActive() error = %v", err)
	}

	if file.State != FileStateActive {
		t.Errorf("State = %q, want ACTIVE", file.State)
	}
	if callCount < 2 {
		t.Errorf("callCount = %d, want >= 2", callCount)
	}
}

func TestWaitForFileActive_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(File{
			Name:  "files/test-123",
			State: FileStateFailed,
			Error: &FileError{Code: 500, Message: "Processing failed"},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.WaitForFileActive(ctx, "files/test-123")
	if err == nil {
		t.Fatal("Expected error for failed file")
	}

	if !errors.Is(err, ErrFileFailed) {
		t.Errorf("Expected ErrFileFailed, got %v", err)
	}
}
