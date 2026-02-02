//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/anthropic"
)

func TestAnthropic_Files_UploadAndDelete(t *testing.T) {
	skipIfNoAnthropicKey(t)

	apiKey := getAnthropicKey(t)
	provider := anthropic.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Upload a test file
	content := "Hello, this is a test file for Anthropic Files API integration testing."
	file, err := provider.UploadFile(ctx, &anthropic.FileUploadRequest{
		File:     strings.NewReader(content),
		Filename: "test-integration.txt",
	})
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	// Verify file metadata
	if file.ID == "" {
		t.Error("File ID is empty")
	}
	if file.Filename != "test-integration.txt" {
		t.Errorf("Filename = %q, want %q", file.Filename, "test-integration.txt")
	}
	if file.Type != "file" {
		t.Errorf("Type = %q, want %q", file.Type, "file")
	}
	if file.SizeBytes == 0 {
		t.Error("SizeBytes is 0")
	}
	if file.Downloadable {
		t.Error("Expected Downloadable to be false for user-uploaded files")
	}

	t.Logf("Uploaded file: ID=%s, Size=%d bytes, MimeType=%s", file.ID, file.SizeBytes, file.MimeType)

	// Cleanup: delete the file
	defer func() {
		delErr := provider.DeleteFile(ctx, file.ID)
		if delErr != nil {
			t.Logf("Warning: failed to delete test file %s: %v", file.ID, delErr)
		} else {
			t.Logf("Deleted test file: %s", file.ID)
		}
	}()

	// Get file metadata
	retrieved, err := provider.GetFile(ctx, file.ID)
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}

	if retrieved.ID != file.ID {
		t.Errorf("GetFile ID = %q, want %q", retrieved.ID, file.ID)
	}
	if retrieved.Filename != file.Filename {
		t.Errorf("GetFile Filename = %q, want %q", retrieved.Filename, file.Filename)
	}

	t.Logf("Retrieved file metadata: ID=%s, Filename=%s", retrieved.ID, retrieved.Filename)
}

func TestAnthropic_Files_ListFiles(t *testing.T) {
	skipIfNoAnthropicKey(t)

	apiKey := getAnthropicKey(t)
	provider := anthropic.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Upload a test file first so we have something to list
	content := "Test file for list operation."
	file, err := provider.UploadFile(ctx, &anthropic.FileUploadRequest{
		File:     strings.NewReader(content),
		Filename: "test-list-integration.txt",
	})
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	// Cleanup: delete the file after test
	defer func() {
		delErr := provider.DeleteFile(ctx, file.ID)
		if delErr != nil {
			t.Logf("Warning: failed to delete test file %s: %v", file.ID, delErr)
		}
	}()

	t.Logf("Uploaded test file: %s", file.ID)

	// List files with pagination
	limit := 10
	resp, err := provider.ListFiles(ctx, &anthropic.FileListRequest{
		Limit: &limit,
	})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	if resp.Data == nil {
		t.Error("ListFiles Data is nil")
	}

	t.Logf("Listed %d files (HasMore=%v)", len(resp.Data), resp.HasMore)

	// Verify our uploaded file is in the list
	found := false
	for _, f := range resp.Data {
		if f.ID == file.ID {
			found = true
			break
		}
	}

	if !found {
		t.Logf("Note: Uploaded file %s not found in first page of results (may be paginated)", file.ID)
	}
}

func TestAnthropic_Files_GetNotFound(t *testing.T) {
	skipIfNoAnthropicKey(t)

	apiKey := getAnthropicKey(t)
	provider := anthropic.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to get a non-existent file
	_, err := provider.GetFile(ctx, "file_nonexistent_test_id")
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Expected ProviderError, got %T: %v", err, err)
	}

	if !errors.Is(provErr, core.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", provErr.Err)
	}

	t.Logf("Got expected error for non-existent file: %v", err)
}

func TestAnthropic_Files_DownloadNotDownloadable(t *testing.T) {
	skipIfNoAnthropicKey(t)

	apiKey := getAnthropicKey(t)
	provider := anthropic.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Upload a test file
	content := "Test file for download check."
	file, err := provider.UploadFile(ctx, &anthropic.FileUploadRequest{
		File:     strings.NewReader(content),
		Filename: "test-download-check.txt",
	})
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	// Cleanup: delete the file after test
	defer func() {
		delErr := provider.DeleteFile(ctx, file.ID)
		if delErr != nil {
			t.Logf("Warning: failed to delete test file %s: %v", file.ID, delErr)
		}
	}()

	// Try to download a user-uploaded file (should fail with ErrFileNotDownloadable)
	_, err = provider.DownloadFile(ctx, file.ID)
	if err == nil {
		t.Fatal("Expected error when downloading user-uploaded file, got nil")
	}

	if !errors.Is(err, anthropic.ErrFileNotDownloadable) {
		t.Errorf("Expected ErrFileNotDownloadable, got %v", err)
	}

	t.Logf("Got expected error for non-downloadable file: %v", err)
}

func TestAnthropic_Files_DeleteNotFound(t *testing.T) {
	skipIfNoAnthropicKey(t)

	apiKey := getAnthropicKey(t)
	provider := anthropic.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to delete a non-existent file
	err := provider.DeleteFile(ctx, "file_nonexistent_delete_test")
	if err == nil {
		t.Fatal("Expected error for deleting non-existent file, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Expected ProviderError, got %T: %v", err, err)
	}

	if !errors.Is(provErr, core.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", provErr.Err)
	}

	t.Logf("Got expected error for deleting non-existent file: %v", err)
}

func TestAnthropic_Files_ListAllFiles(t *testing.T) {
	skipIfNoAnthropicKey(t)

	apiKey := getAnthropicKey(t)
	provider := anthropic.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// ListAllFiles is a convenience helper that handles pagination
	files, err := provider.ListAllFiles(ctx)
	if err != nil {
		t.Fatalf("ListAllFiles() error = %v", err)
	}

	// We don't know how many files exist, but the call should succeed
	t.Logf("ListAllFiles returned %d files", len(files))

	// If there are files, verify they have valid structure
	for i, f := range files {
		if f.ID == "" {
			t.Errorf("File %d has empty ID", i)
		}
		if i < 3 {
			t.Logf("  File %d: ID=%s, Filename=%s", i, f.ID, f.Filename)
		}
	}
	if len(files) > 3 {
		t.Logf("  ... and %d more files", len(files)-3)
	}
}
