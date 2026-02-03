package keystore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"golang.org/x/crypto/argon2"
)

// File format constants
const (
	// magicHeader identifies v2 keystore files
	magicHeader = "IRIS"
	// versionV2 is the current file format version
	versionV2 = byte(0x02)
	// saltLength is the length of the Argon2id salt
	saltLength = 16
	// nonceLength is the AES-GCM nonce length
	nonceLength = 12
)

// Argon2id parameters (OWASP recommended)
const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4
	argon2KeyLen  = 32
)

// FileKeystore implements Keystore using encrypted file storage.
// Keys are stored in a JSON map encrypted with AES-256-GCM.
// v2 format uses Argon2id for key derivation from a master key.
type FileKeystore struct {
	path      string
	masterKey []byte
	mu        sync.RWMutex
}

// NewFileKeystore creates a new file-based keystore at the given path.
// The encryption key is derived from machine-specific data (v1 legacy mode).
// For production use, prefer NewFileKeystoreWithSource.
func NewFileKeystore(path string) (*FileKeystore, error) {
	key, err := deriveKeyV1()
	if err != nil {
		return nil, err
	}

	return &FileKeystore{
		path:      path,
		masterKey: key,
	}, nil
}

// NewFileKeystoreWithSource creates a new file-based keystore with a master key source.
// This is the recommended way to create a keystore for production use.
func NewFileKeystoreWithSource(path string, source MasterKeySource) (*FileKeystore, error) {
	masterKey, err := source.GetMasterKey()
	if err != nil {
		return nil, err
	}

	return &FileKeystore{
		path:      path,
		masterKey: masterKey,
	}, nil
}

// Set stores a key-value pair.
func (f *FileKeystore) Set(name, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.loadData()
	if err != nil {
		return err
	}

	data[name] = value
	return f.saveData(data)
}

// Get retrieves a value by name.
func (f *FileKeystore) Get(name string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := f.loadData()
	if err != nil {
		return "", err
	}

	value, ok := data[name]
	if !ok {
		return "", &ErrKeyNotFound{Name: name}
	}

	return value, nil
}

// Delete removes a key by name.
func (f *FileKeystore) Delete(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.loadData()
	if err != nil {
		return err
	}

	if _, ok := data[name]; !ok {
		return &ErrKeyNotFound{Name: name}
	}

	delete(data, name)
	return f.saveData(data)
}

// List returns all stored key names.
func (f *FileKeystore) List() ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := f.loadData()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(data))
	for name := range data {
		names = append(names, name)
	}
	sort.Strings(names)

	return names, nil
}

// loadData reads and decrypts the keystore file.
// Automatically detects v1 vs v2 format.
func (f *FileKeystore) loadData() (map[string]string, error) {
	data := make(map[string]string)

	ciphertext, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}

	if len(ciphertext) == 0 {
		return data, nil
	}

	// Detect format version
	var plaintext []byte
	if isV2Format(ciphertext) {
		plaintext, err = f.decryptV2(ciphertext)
	} else {
		// Legacy v1 format
		plaintext, err = f.decryptV1(ciphertext)
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// saveData encrypts and writes the keystore file.
// Always uses v2 format for new writes.
func (f *FileKeystore) saveData(data map[string]string) error {
	// Ensure directory exists
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	plaintext, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ciphertext, err := f.encryptV2(plaintext)
	if err != nil {
		return err
	}

	// Write with restrictive permissions (user only)
	return os.WriteFile(f.path, ciphertext, 0600)
}

// isV2Format checks if the ciphertext starts with the v2 magic header.
func isV2Format(ciphertext []byte) bool {
	if len(ciphertext) < len(magicHeader)+1 {
		return false
	}
	return string(ciphertext[:len(magicHeader)]) == magicHeader && ciphertext[len(magicHeader)] == versionV2
}

// deriveKeyV2 derives an encryption key from the master key using Argon2id.
func deriveKeyV2(masterKey, salt []byte) []byte {
	return argon2.IDKey(masterKey, salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
}

// encryptV2 encrypts data using AES-256-GCM with Argon2id key derivation.
// Format: [magic (4)] [version (1)] [salt (16)] [nonce (12)] [ciphertext]
func (f *FileKeystore) encryptV2(plaintext []byte) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// Derive encryption key using Argon2id
	key := deriveKeyV2(f.masterKey, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Build output: magic + version + salt + nonce + ciphertext
	header := make([]byte, 0, len(magicHeader)+1+saltLength+nonceLength)
	header = append(header, []byte(magicHeader)...)
	header = append(header, versionV2)
	header = append(header, salt...)
	header = append(header, nonce...)

	ciphertext := gcm.Seal(nil, nonce, plaintext, header)
	return append(header, ciphertext...), nil
}

// decryptV2 decrypts data using AES-256-GCM with Argon2id key derivation.
func (f *FileKeystore) decryptV2(ciphertext []byte) ([]byte, error) {
	headerLen := len(magicHeader) + 1 + saltLength + nonceLength
	if len(ciphertext) < headerLen {
		return nil, errors.New("ciphertext too short")
	}

	// Parse header
	offset := len(magicHeader) + 1 // Skip magic and version
	salt := ciphertext[offset : offset+saltLength]
	offset += saltLength
	nonce := ciphertext[offset : offset+nonceLength]
	offset += nonceLength
	encrypted := ciphertext[offset:]
	header := ciphertext[:offset]

	// Derive key using Argon2id
	key := deriveKeyV2(f.masterKey, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, encrypted, header)
}

// encryptV1 encrypts data using AES-256-GCM (legacy v1 format).
func (f *FileKeystore) encryptV1(plaintext []byte) ([]byte, error) {
	key := f.deriveKeyFromMasterV1()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Prepend nonce to ciphertext
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptV1 decrypts data using AES-256-GCM (legacy v1 format).
func (f *FileKeystore) decryptV1(ciphertext []byte) ([]byte, error) {
	key := f.deriveKeyFromMasterV1()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// deriveKeyFromMasterV1 hashes the master key to get a 32-byte key for v1 format.
// This provides backward compatibility when using NewFileKeystoreWithSource with v1 files.
func (f *FileKeystore) deriveKeyFromMasterV1() []byte {
	hash := sha256.Sum256(f.masterKey)
	return hash[:]
}

// deriveKeyV1 creates a machine-specific encryption key (legacy v1 method).
// Uses hostname and user as entropy sources, hashed to create a 32-byte key.
// SECURITY NOTE: This is predictable and kept only for backward compatibility.
func deriveKeyV1() ([]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	// Combine machine-specific data with a salt
	material := hostname + ":" + username + ":iris-keystore-v1"

	// SHA-256 produces a 32-byte key suitable for AES-256
	hash := sha256.Sum256([]byte(material))
	return hash[:], nil
}

// MigrateToV2 migrates a v1 keystore to v2 format.
// The keystore must be opened with the new master key source.
func (f *FileKeystore) MigrateToV2() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Read file to check format
	ciphertext, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to migrate
		}
		return err
	}

	if len(ciphertext) == 0 {
		return nil // Empty file, nothing to migrate
	}

	// Already v2 format
	if isV2Format(ciphertext) {
		return nil
	}

	// Read data using v1 format
	plaintext, err := f.decryptV1(ciphertext)
	if err != nil {
		return err
	}

	var data map[string]string
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return err
	}

	// Re-encrypt with v2 format
	newPlaintext, err := json.Marshal(data)
	if err != nil {
		return err
	}

	newCiphertext, err := f.encryptV2(newPlaintext)
	if err != nil {
		return err
	}

	// Backup old file
	backupPath := f.path + ".v1.bak"
	if err := os.WriteFile(backupPath, ciphertext, 0600); err != nil {
		return err
	}

	// Write new file
	return os.WriteFile(f.path, newCiphertext, 0600)
}

// IsV2Format checks if the keystore file is in v2 format.
func (f *FileKeystore) IsV2Format() (bool, error) {
	ciphertext, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil // New files will be v2
		}
		return false, err
	}

	if len(ciphertext) == 0 {
		return true, nil // Empty files will become v2 on first write
	}

	return isV2Format(ciphertext), nil
}

// Ensure FileKeystore implements Keystore
var _ Keystore = (*FileKeystore)(nil)

// Legacy function alias for backward compatibility
var deriveKey = deriveKeyV1

// Helper for testing: check if two byte slices have the same prefix
func hasPrefix(data, prefix []byte) bool {
	return len(data) >= len(prefix) && bytes.Equal(data[:len(prefix)], prefix)
}
