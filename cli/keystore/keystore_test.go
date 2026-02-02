package keystore

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileKeystoreSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// Set a key
	if err := ks.Set("openai", "sk-test-key-12345"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get it back
	value, err := ks.Get("openai")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "sk-test-key-12345" {
		t.Errorf("Get() = %q, want sk-test-key-12345", value)
	}
}

func TestFileKeystoreGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	_, err = ks.Get("nonexistent")
	if err == nil {
		t.Fatal("Get() should return error for nonexistent key")
	}

	if _, ok := err.(*ErrKeyNotFound); !ok {
		t.Errorf("Get() error type = %T, want *ErrKeyNotFound", err)
	}
}

func TestFileKeystoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// Set a key
	if err := ks.Set("anthropic", "sk-ant-test"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Delete it
	if err := ks.Delete("anthropic"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	_, err = ks.Get("anthropic")
	if _, ok := err.(*ErrKeyNotFound); !ok {
		t.Error("Get() should return ErrKeyNotFound after Delete()")
	}
}

func TestFileKeystoreDeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	err = ks.Delete("nonexistent")
	if err == nil {
		t.Fatal("Delete() should return error for nonexistent key")
	}

	if _, ok := err.(*ErrKeyNotFound); !ok {
		t.Errorf("Delete() error type = %T, want *ErrKeyNotFound", err)
	}
}

func TestFileKeystoreList(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// List empty keystore
	names, err := ks.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 0 {
		t.Errorf("List() on empty keystore returned %d items", len(names))
	}

	// Add some keys
	if err := ks.Set("openai", "key1"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := ks.Set("anthropic", "key2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := ks.Set("ollama", "key3"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// List should return sorted names
	names, err = ks.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(names) != 3 {
		t.Fatalf("List() returned %d items, want 3", len(names))
	}

	// Should be sorted
	expected := []string{"anthropic", "ollama", "openai"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("List()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestFileKeystoreOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// Set a key
	if err := ks.Set("openai", "original-key"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Overwrite it
	if err := ks.Set("openai", "updated-key"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should get the new value
	value, err := ks.Get("openai")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "updated-key" {
		t.Errorf("Get() = %q, want updated-key", value)
	}
}

func TestFileKeystorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	// Create first keystore and set a key
	ks1, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	if err := ks1.Set("openai", "persistent-key"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Create new keystore instance pointing to same file
	ks2, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// Should be able to read the key
	value, err := ks2.Get("openai")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "persistent-key" {
		t.Errorf("Get() = %q, want persistent-key", value)
	}
}

func TestFileKeystoreFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions not supported on Windows")
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// Set a key to create the file
	if err := ks.Set("test", "value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Check file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	// Should be 0600 (user read/write only)
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("File permissions = %o, want 0600", mode)
	}
}

func TestFileKeystoreEncrypted(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// Set a key with recognizable content
	secretKey := "sk-this-should-be-encrypted"
	if err := ks.Set("openai", secretKey); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Read raw file contents
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// File should not contain plaintext key
	if string(contents) == secretKey {
		t.Error("File contains plaintext key - encryption failed")
	}

	// File should not be valid JSON (it's encrypted)
	if len(contents) > 0 && contents[0] == '{' {
		t.Error("File appears to be unencrypted JSON")
	}
}

func TestFileKeystoreCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "deep", "keys.enc")

	ks, err := NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}

	// Set a key - should create directories
	if err := ks.Set("test", "value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("File not created: %v", err)
	}
}

func TestDefaultKeystorePath(t *testing.T) {
	path := DefaultKeystorePath()

	if path == "" {
		t.Error("DefaultKeystorePath() returned empty string")
	}

	// Should end with keys.enc
	if filepath.Base(path) != "keys.enc" {
		t.Errorf("DefaultKeystorePath() = %q, should end with keys.enc", path)
	}

	// Should contain .iris directory
	dir := filepath.Dir(path)
	if filepath.Base(dir) != ".iris" {
		t.Errorf("DefaultKeystorePath() = %q, should be in .iris directory", path)
	}
}

func TestErrKeyNotFoundError(t *testing.T) {
	err := &ErrKeyNotFound{Name: "openai"}
	msg := err.Error()

	if msg != "key not found: openai" {
		t.Errorf("Error() = %q, want 'key not found: openai'", msg)
	}
}
