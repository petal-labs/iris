// Package keystore provides secure storage for API keys.
package keystore

import (
	"os"
	"path/filepath"
	"runtime"
)

// Keystore defines the interface for secure key storage.
type Keystore interface {
	// Set stores a key-value pair.
	Set(name, value string) error
	// Get retrieves a value by name. Returns error if not found.
	Get(name string) (string, error)
	// Delete removes a key by name.
	Delete(name string) error
	// List returns all stored key names.
	List() ([]string, error)
}

// ErrKeyNotFound is returned when a requested key does not exist.
type ErrKeyNotFound struct {
	Name string
}

func (e *ErrKeyNotFound) Error() string {
	return "key not found: " + e.Name
}

// DefaultKeystorePath returns the default keystore file path.
// - macOS/Linux: ~/.iris/keys.enc
// - Windows: %USERPROFILE%\.iris\keys.enc
func DefaultKeystorePath() string {
	var homeDir string

	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("USERPROFILE")
	} else {
		homeDir = os.Getenv("HOME")
	}

	if homeDir == "" {
		return "keys.enc"
	}

	return filepath.Join(homeDir, ".iris", "keys.enc")
}

// NewKeystore creates a new keystore using file-based encrypted storage.
func NewKeystore() (Keystore, error) {
	return NewFileKeystore(DefaultKeystorePath())
}
