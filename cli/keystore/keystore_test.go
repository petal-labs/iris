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

// --- v2 Format Tests ---

// staticMasterKeySource provides a fixed master key for testing.
type staticMasterKeySource struct {
	key []byte
}

func (s *staticMasterKeySource) GetMasterKey() ([]byte, error) {
	return s.key, nil
}

func TestKeystoreV2Format(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	source := &staticMasterKeySource{key: []byte("test-master-password-123")}
	ks, err := NewFileKeystoreWithSource(path, source)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}

	// Set a key
	if err := ks.Set("openai", "sk-test-key-12345"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Read raw file to check v2 format
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Should start with "IRIS" magic header
	if len(contents) < 5 {
		t.Fatalf("File too short: %d bytes", len(contents))
	}

	if string(contents[:4]) != "IRIS" {
		t.Errorf("File doesn't start with IRIS magic header")
	}

	// Version should be 0x02
	if contents[4] != 0x02 {
		t.Errorf("File version = %d, want 2", contents[4])
	}

	// Verify IsV2Format
	isV2, err := ks.IsV2Format()
	if err != nil {
		t.Fatalf("IsV2Format() error = %v", err)
	}
	if !isV2 {
		t.Error("IsV2Format() = false, want true")
	}
}

func TestKeystoreV2DifferentMasterKeys(t *testing.T) {
	tmpDir := t.TempDir()

	// Create keystore with first master key
	path1 := filepath.Join(tmpDir, "keys1.enc")
	source1 := &staticMasterKeySource{key: []byte("master-key-one")}
	ks1, err := NewFileKeystoreWithSource(path1, source1)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}
	if err := ks1.Set("test", "secret-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Create keystore with different master key
	path2 := filepath.Join(tmpDir, "keys2.enc")
	source2 := &staticMasterKeySource{key: []byte("master-key-two")}
	ks2, err := NewFileKeystoreWithSource(path2, source2)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}
	if err := ks2.Set("test", "secret-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Try to read ks1's file with ks2's key - should fail
	wrongSource := &staticMasterKeySource{key: []byte("master-key-two")}
	ksWrong, err := NewFileKeystoreWithSource(path1, wrongSource)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}
	_, err = ksWrong.Get("test")
	if err == nil {
		t.Error("Get() should fail with wrong master key")
	}
}

func TestKeystoreV2SaltUniqueness(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two files with the same master key and same data
	source := &staticMasterKeySource{key: []byte("same-master-key")}

	path1 := filepath.Join(tmpDir, "keys1.enc")
	ks1, err := NewFileKeystoreWithSource(path1, source)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}
	if err := ks1.Set("test", "same-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	path2 := filepath.Join(tmpDir, "keys2.enc")
	ks2, err := NewFileKeystoreWithSource(path2, source)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}
	if err := ks2.Set("test", "same-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Read both files
	contents1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	contents2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// The salts should be different (bytes 5-20)
	salt1 := contents1[5:21]
	salt2 := contents2[5:21]

	if string(salt1) == string(salt2) {
		t.Error("Salts should be unique but are identical")
	}
}

func TestKeystoreV2WithSourcePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")

	// Create and populate keystore
	source := &staticMasterKeySource{key: []byte("persistent-master-key")}
	ks1, err := NewFileKeystoreWithSource(path, source)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}
	if err := ks1.Set("openai", "sk-persistent"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := ks1.Set("anthropic", "sk-ant-persistent"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Open with a new instance (same key)
	ks2, err := NewFileKeystoreWithSource(path, source)
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}

	// Verify data persisted
	value, err := ks2.Get("openai")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "sk-persistent" {
		t.Errorf("Get(openai) = %q, want sk-persistent", value)
	}

	value, err = ks2.Get("anthropic")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "sk-ant-persistent" {
		t.Errorf("Get(anthropic) = %q, want sk-ant-persistent", value)
	}
}

// --- MasterKeySource Tests ---

func TestEnvMasterKeySource(t *testing.T) {
	// Test with default env var name
	envVar := "TEST_IRIS_KEYSTORE_KEY"
	t.Setenv(envVar, "test-env-key")

	source := &EnvMasterKeySource{EnvVar: envVar}
	key, err := source.GetMasterKey()
	if err != nil {
		t.Fatalf("GetMasterKey() error = %v", err)
	}
	if string(key) != "test-env-key" {
		t.Errorf("GetMasterKey() = %q, want test-env-key", string(key))
	}
}

func TestEnvMasterKeySourceMissing(t *testing.T) {
	source := &EnvMasterKeySource{EnvVar: "NONEXISTENT_VAR_12345"}
	_, err := source.GetMasterKey()
	if err == nil {
		t.Error("GetMasterKey() should fail when env var is not set")
	}
}

func TestEnvMasterKeySourceDefaultVar(t *testing.T) {
	// Set the default env var
	t.Setenv(DefaultMasterKeyEnvVar, "default-key")

	source := &EnvMasterKeySource{} // No EnvVar set, uses default
	key, err := source.GetMasterKey()
	if err != nil {
		t.Fatalf("GetMasterKey() error = %v", err)
	}
	if string(key) != "default-key" {
		t.Errorf("GetMasterKey() = %q, want default-key", string(key))
	}
}

func TestFallbackMasterKeySource(t *testing.T) {
	// First source fails, second succeeds
	failSource := &EnvMasterKeySource{EnvVar: "NONEXISTENT_VAR_12345"}
	succeedSource := &staticMasterKeySource{key: []byte("fallback-key")}

	source := &FallbackMasterKeySource{
		Sources: []MasterKeySource{failSource, succeedSource},
	}

	key, err := source.GetMasterKey()
	if err != nil {
		t.Fatalf("GetMasterKey() error = %v", err)
	}
	if string(key) != "fallback-key" {
		t.Errorf("GetMasterKey() = %q, want fallback-key", string(key))
	}
}

func TestFallbackMasterKeySourceAllFail(t *testing.T) {
	failSource1 := &EnvMasterKeySource{EnvVar: "NONEXISTENT_VAR_1"}
	failSource2 := &EnvMasterKeySource{EnvVar: "NONEXISTENT_VAR_2"}

	source := &FallbackMasterKeySource{
		Sources: []MasterKeySource{failSource1, failSource2},
	}

	_, err := source.GetMasterKey()
	if err == nil {
		t.Error("GetMasterKey() should fail when all sources fail")
	}
}

func TestPromptMasterKeySource(t *testing.T) {
	source := &PromptMasterKeySource{
		Prompter: func(prompt string) ([]byte, error) {
			if prompt != "Enter keystore password: " {
				t.Errorf("Prompt = %q, unexpected", prompt)
			}
			return []byte("prompted-key"), nil
		},
	}

	key, err := source.GetMasterKey()
	if err != nil {
		t.Fatalf("GetMasterKey() error = %v", err)
	}
	if string(key) != "prompted-key" {
		t.Errorf("GetMasterKey() = %q, want prompted-key", string(key))
	}
}

func TestPromptMasterKeySourceNilPrompter(t *testing.T) {
	source := &PromptMasterKeySource{} // No prompter
	_, err := source.GetMasterKey()
	if err == nil {
		t.Error("GetMasterKey() should fail with nil prompter")
	}
}

// --- Argon2id Key Derivation Tests ---

func TestArgon2idKeyDerivation(t *testing.T) {
	masterKey := []byte("test-master-key")
	salt1 := []byte("salt1234567890ab") // 16 bytes
	salt2 := []byte("different-salt!!")

	key1 := deriveKeyV2(masterKey, salt1)
	key2 := deriveKeyV2(masterKey, salt2)

	// Keys should be 32 bytes (AES-256)
	if len(key1) != 32 {
		t.Errorf("Key1 length = %d, want 32", len(key1))
	}
	if len(key2) != 32 {
		t.Errorf("Key2 length = %d, want 32", len(key2))
	}

	// Different salts should produce different keys
	if string(key1) == string(key2) {
		t.Error("Different salts should produce different keys")
	}

	// Same salt should produce same key (deterministic)
	key1Again := deriveKeyV2(masterKey, salt1)
	if string(key1) != string(key1Again) {
		t.Error("Same inputs should produce same key")
	}
}

// --- isV2Format Tests ---

func TestIsV2Format(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"too short", []byte("IRI"), false},
		{"wrong magic", []byte("NOTM\x02abcdefgh"), false},
		{"wrong version", []byte("IRIS\x01abcdefgh"), false},
		{"valid v2", []byte("IRIS\x02abcdefghijklmnop"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isV2Format(tt.data)
			if got != tt.want {
				t.Errorf("isV2Format() = %v, want %v", got, tt.want)
			}
		})
	}
}
