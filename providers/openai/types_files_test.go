package openai

import (
	"encoding/json"
	"testing"
)

func TestFilePurposeConstants(t *testing.T) {
	purposes := []FilePurpose{
		FilePurposeAssistants,
		FilePurposeBatch,
		FilePurposeFineTune,
		FilePurposeVision,
		FilePurposeUserData,
		FilePurposeEvals,
	}

	expected := []string{"assistants", "batch", "fine-tune", "vision", "user_data", "evals"}
	for i, p := range purposes {
		if string(p) != expected[i] {
			t.Errorf("expected %q, got %q", expected[i], p)
		}
	}
}

func TestFileJSONUnmarshal(t *testing.T) {
	data := `{
		"id": "file-abc123",
		"object": "file",
		"bytes": 120000,
		"created_at": 1677610602,
		"expires_at": 1677614202,
		"filename": "mydata.jsonl",
		"purpose": "fine-tune"
	}`

	var f File
	if err := json.Unmarshal([]byte(data), &f); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if f.ID != "file-abc123" {
		t.Errorf("expected ID 'file-abc123', got %q", f.ID)
	}
	if f.Bytes != 120000 {
		t.Errorf("expected Bytes 120000, got %d", f.Bytes)
	}
	if f.Purpose != FilePurposeFineTune {
		t.Errorf("expected Purpose 'fine-tune', got %q", f.Purpose)
	}
	if f.ExpiresAt == nil || *f.ExpiresAt != 1677614202 {
		t.Errorf("expected ExpiresAt 1677614202, got %v", f.ExpiresAt)
	}
}

func TestExpiresAfterJSONMarshal(t *testing.T) {
	ea := ExpiresAfter{
		Anchor:  "created_at",
		Seconds: 2592000,
	}

	data, err := json.Marshal(ea)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	expected := `{"anchor":"created_at","seconds":2592000}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, data)
	}
}

func TestFileListResponseJSONUnmarshal(t *testing.T) {
	data := `{
		"object": "list",
		"data": [
			{"id": "file-1", "object": "file", "bytes": 100, "created_at": 1000, "filename": "a.txt", "purpose": "user_data"},
			{"id": "file-2", "object": "file", "bytes": 200, "created_at": 2000, "filename": "b.txt", "purpose": "batch"}
		],
		"has_more": true,
		"first_id": "file-1",
		"last_id": "file-2"
	}`

	var resp FileListResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 files, got %d", len(resp.Data))
	}
	if !resp.HasMore {
		t.Error("expected HasMore to be true")
	}
	if resp.FirstID != "file-1" {
		t.Errorf("expected FirstID 'file-1', got %q", resp.FirstID)
	}
}

func TestVectorStoreStatusConstants(t *testing.T) {
	statuses := []VectorStoreStatus{
		VectorStoreStatusExpired,
		VectorStoreStatusInProgress,
		VectorStoreStatusCompleted,
	}

	expected := []string{"expired", "in_progress", "completed"}
	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected %q, got %q", expected[i], s)
		}
	}
}

func TestVectorStoreJSONUnmarshal(t *testing.T) {
	data := `{
		"id": "vs_abc123",
		"object": "vector_store",
		"created_at": 1699061776,
		"name": "Support FAQ",
		"usage_bytes": 139920,
		"file_counts": {
			"in_progress": 0,
			"completed": 3,
			"failed": 0,
			"cancelled": 0,
			"total": 3
		},
		"status": "completed"
	}`

	var vs VectorStore
	if err := json.Unmarshal([]byte(data), &vs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if vs.ID != "vs_abc123" {
		t.Errorf("expected ID 'vs_abc123', got %q", vs.ID)
	}
	if vs.Name != "Support FAQ" {
		t.Errorf("expected Name 'Support FAQ', got %q", vs.Name)
	}
	if vs.Status != VectorStoreStatusCompleted {
		t.Errorf("expected Status 'completed', got %q", vs.Status)
	}
	if vs.FileCounts.Completed != 3 {
		t.Errorf("expected FileCounts.Completed 3, got %d", vs.FileCounts.Completed)
	}
}

func TestVectorStoreFileJSONUnmarshal(t *testing.T) {
	data := `{
		"id": "file-abc123",
		"object": "vector_store.file",
		"created_at": 1699061776,
		"usage_bytes": 1234,
		"vector_store_id": "vs_abcd",
		"status": "completed",
		"last_error": null,
		"chunking_strategy": {
			"type": "static",
			"static": {
				"max_chunk_size_tokens": 800,
				"chunk_overlap_tokens": 400
			}
		}
	}`

	var vsf VectorStoreFile
	if err := json.Unmarshal([]byte(data), &vsf); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if vsf.ID != "file-abc123" {
		t.Errorf("expected ID 'file-abc123', got %q", vsf.ID)
	}
	if vsf.Status != VectorStoreFileStatusCompleted {
		t.Errorf("expected Status 'completed', got %q", vsf.Status)
	}
	if vsf.ChunkingStrategy == nil || vsf.ChunkingStrategy.Type != "static" {
		t.Error("expected ChunkingStrategy.Type 'static'")
	}
	if vsf.ChunkingStrategy.Static.MaxChunkSizeTokens != 800 {
		t.Errorf("expected MaxChunkSizeTokens 800, got %d", vsf.ChunkingStrategy.Static.MaxChunkSizeTokens)
	}
}

func TestChunkingStrategyJSONMarshal(t *testing.T) {
	cs := &ChunkingStrategy{
		Type: "static",
		Static: &StaticChunkingOpts{
			MaxChunkSizeTokens: 800,
			ChunkOverlapTokens: 400,
		},
	}

	data, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	expected := `{"type":"static","static":{"max_chunk_size_tokens":800,"chunk_overlap_tokens":400}}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, data)
	}
}
