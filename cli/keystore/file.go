package keystore

import (
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
)

// FileKeystore implements Keystore using encrypted file storage.
// Keys are stored in a JSON map encrypted with AES-256-GCM.
type FileKeystore struct {
	path string
	key  []byte
	mu   sync.RWMutex
}

// NewFileKeystore creates a new file-based keystore at the given path.
// The encryption key is derived from machine-specific data.
func NewFileKeystore(path string) (*FileKeystore, error) {
	key, err := deriveKey()
	if err != nil {
		return nil, err
	}

	return &FileKeystore{
		path: path,
		key:  key,
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

	plaintext, err := f.decrypt(ciphertext)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// saveData encrypts and writes the keystore file.
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

	ciphertext, err := f.encrypt(plaintext)
	if err != nil {
		return err
	}

	// Write with restrictive permissions (user only)
	return os.WriteFile(f.path, ciphertext, 0600)
}

// encrypt encrypts data using AES-256-GCM.
func (f *FileKeystore) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
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

// decrypt decrypts data using AES-256-GCM.
func (f *FileKeystore) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
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

// deriveKey creates a machine-specific encryption key.
// Uses hostname and user as entropy sources, hashed to create a 32-byte key.
func deriveKey() ([]byte, error) {
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
