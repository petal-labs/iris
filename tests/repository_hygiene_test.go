package tests

import (
	"os"
	"strings"
	"testing"
)

func TestWorkspaceChecksumIsCommittedPolicy(t *testing.T) {
	gitignore := readRepositoryFile(t, ".gitignore")
	if containsTrimmedLine(gitignore, "go.work.sum") {
		t.Error("go.work.sum must not be ignored")
	}
	info, err := os.Stat("../go.work.sum")
	if err != nil {
		t.Fatalf("go.work.sum must exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("go.work.sum must not be empty")
	}
}

func TestMacOSMetadataIgnorePattern(t *testing.T) {
	gitignore := readRepositoryFile(t, ".gitignore")
	if !containsTrimmedLine(gitignore, ".DS_Store") {
		t.Error(".gitignore must contain the case-correct .DS_Store pattern")
	}
	if containsTrimmedLine(gitignore, ".DS_STORE") {
		t.Error(".gitignore must not contain the incorrect .DS_STORE pattern")
	}
}

func TestContributorDocumentationIncludesChangeAndReleaseProcesses(t *testing.T) {
	contributing := readRepositoryFile(t, "CONTRIBUTING.md")
	for _, required := range []string{
		"## Change Documentation",
		"docs/changes/YYYY-MM-DD_v{version}_{feature-slug}.md",
		"## Release Process",
		".github/workflows/release.yml",
		"git tag",
	} {
		if !strings.Contains(contributing, required) {
			t.Errorf("CONTRIBUTING.md must document %q", required)
		}
	}
}
