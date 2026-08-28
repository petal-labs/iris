package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContribOtelIsCoveredByWorkspaceAndCI(t *testing.T) {
	workspace := readRepositoryFile(t, "go.work")
	if !containsTrimmedLine(workspace, "./contrib/otel") {
		t.Error("go.work must include ./contrib/otel")
	}

	workflow := readRepositoryFile(t, ".github/workflows/ci.yml")
	for _, command := range []string{"go build", "go test"} {
		if !containsCommandForModule(workflow, command, "./contrib/otel/...") {
			t.Errorf("CI must run %q for ./contrib/otel/...", command)
		}
	}
}

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func containsTrimmedLine(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func containsCommandForModule(content, command, module string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, command) && strings.Contains(line, module) {
			return true
		}
	}
	return false
}
