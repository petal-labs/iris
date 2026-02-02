//go:build integration

// Package integration provides integration tests for the Iris SDK.
package integration

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// cliBinary holds the path to the pre-built CLI binary.
// It is set once in TestMain and used by all tests.
var cliBinary string

// TestMain builds the CLI binary once before running all tests,
// and cleans up afterward. This avoids ~500ms of redundant builds per test.
func TestMain(m *testing.M) {
	// Find the project root (where go.mod is)
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		log.Fatal("Could not find project root (go.mod)")
	}

	// Create temp directory for the binary
	tmpDir, err := os.MkdirTemp("", "iris-integration-test")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	// Build the CLI binary once
	cliBinary = filepath.Join(tmpDir, "iris-test")
	cmd := exec.Command("go", "build", "-o", cliBinary, "./cli/cmd/iris")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Failed to build CLI: %v\n%s", err, output)
	}

	// Run all tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)

	os.Exit(code)
}

// findProjectRoot locates the project root by looking for go.mod.
func findProjectRoot() string {
	// Start from the current working directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return ""
}

// getCliBinary returns the path to the pre-built CLI binary.
// Tests should use this instead of building their own.
func getCliBinary() string {
	return cliBinary
}
