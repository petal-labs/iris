package anthropic

import (
	"encoding/json"
	"testing"
)

func TestFileUnmarshal(t *testing.T) {
	data := []byte(`{
		"id": "file_011CNha8iCJcU1wXNR6q4V8w",
		"type": "file",
		"filename": "test.pdf",
		"mime_type": "application/pdf",
		"size_bytes": 1024,
		"created_at": "2025-04-14T12:00:00Z",
		"downloadable": false
	}`)

	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if f.ID != "file_011CNha8iCJcU1wXNR6q4V8w" {
		t.Errorf("expected ID 'file_011CNha8iCJcU1wXNR6q4V8w', got %q", f.ID)
	}
	if f.Type != "file" {
		t.Errorf("expected Type 'file', got %q", f.Type)
	}
	if f.Filename != "test.pdf" {
		t.Errorf("expected Filename 'test.pdf', got %q", f.Filename)
	}
	if f.MimeType != "application/pdf" {
		t.Errorf("expected MimeType 'application/pdf', got %q", f.MimeType)
	}
	if f.SizeBytes != 1024 {
		t.Errorf("expected SizeBytes 1024, got %d", f.SizeBytes)
	}
	if f.CreatedAt != "2025-04-14T12:00:00Z" {
		t.Errorf("expected CreatedAt '2025-04-14T12:00:00Z', got %q", f.CreatedAt)
	}
	if f.Downloadable != false {
		t.Errorf("expected Downloadable false, got %v", f.Downloadable)
	}
}

func TestFileListResponseUnmarshal(t *testing.T) {
	data := []byte(`{
		"data": [
			{"id": "file_1", "type": "file", "filename": "a.txt", "mime_type": "text/plain", "size_bytes": 100, "created_at": "2025-04-14T12:00:00Z", "downloadable": false}
		],
		"first_id": "file_1",
		"last_id": "file_1",
		"has_more": false
	}`)

	var resp FileListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 file, got %d", len(resp.Data))
	}
	if resp.FirstID != "file_1" {
		t.Errorf("expected FirstID 'file_1', got %q", resp.FirstID)
	}
	if resp.HasMore != false {
		t.Errorf("expected HasMore false, got %v", resp.HasMore)
	}
}

func TestFileDeleteResponseUnmarshal(t *testing.T) {
	data := []byte(`{
		"id": "file_011CNha8iCJcU1wXNR6q4V8w",
		"type": "file_deleted"
	}`)

	var resp FileDeleteResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ID != "file_011CNha8iCJcU1wXNR6q4V8w" {
		t.Errorf("expected ID 'file_011CNha8iCJcU1wXNR6q4V8w', got %q", resp.ID)
	}
	if resp.Type != "file_deleted" {
		t.Errorf("expected Type 'file_deleted', got %q", resp.Type)
	}
}
