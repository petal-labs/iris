//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/providers/gemini"
)

func TestGemini_Files_UploadAndDelete(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Upload a test file
	content := "Hello, this is a test file for Gemini Files API integration testing."
	file, err := provider.UploadFile(ctx, &gemini.FileUploadRequest{
		File:        strings.NewReader(content),
		DisplayName: "test-integration.txt",
		MimeType:    "text/plain",
	})
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	// Verify file metadata
	if file.Name == "" {
		t.Error("File name is empty")
	}
	if !strings.HasPrefix(file.Name, "files/") {
		t.Errorf("File name = %q, want prefix 'files/'", file.Name)
	}

	t.Logf("Uploaded file: Name=%s, State=%s", file.Name, file.State)

	// Cleanup: delete the file
	defer func() {
		delErr := provider.DeleteFile(ctx, file.Name)
		if delErr != nil {
			t.Logf("Warning: failed to delete test file %s: %v", file.Name, delErr)
		} else {
			t.Logf("Deleted test file: %s", file.Name)
		}
	}()

	// Wait for file to become active
	activeFile, err := provider.WaitForFileActive(ctx, file.Name)
	if err != nil {
		t.Fatalf("WaitForFileActive() error = %v", err)
	}

	if activeFile.State != gemini.FileStateActive {
		t.Errorf("File state = %q, want ACTIVE", activeFile.State)
	}

	t.Logf("File is now active: Name=%s, URI=%s", activeFile.Name, activeFile.URI)
}

func TestGemini_Files_ListFiles(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// List files
	resp, err := provider.ListFiles(ctx, &gemini.FileListRequest{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	t.Logf("Listed %d files", len(resp.Files))

	for i, f := range resp.Files {
		if i < 3 {
			t.Logf("  File %d: Name=%s, State=%s", i, f.Name, f.State)
		}
	}
}

func TestGemini_Files_GetNotFound(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to get a non-existent file
	_, err := provider.GetFile(ctx, "files/nonexistent-test-id-12345")
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	t.Logf("Got expected error for non-existent file: %v", err)
}

func TestGemini_Files_ListAllFiles(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// ListAllFiles handles pagination automatically
	files, err := provider.ListAllFiles(ctx)
	if err != nil {
		t.Fatalf("ListAllFiles() error = %v", err)
	}

	t.Logf("ListAllFiles returned %d files", len(files))

	for i, f := range files {
		if i < 3 {
			t.Logf("  File %d: Name=%s, State=%s", i, f.Name, f.State)
		}
	}
}
