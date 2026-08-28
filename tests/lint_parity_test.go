package tests

import (
	"regexp"
	"strings"
	"testing"
)

var makefileLintVersionPattern = regexp.MustCompile(`(?m)^GOLANGCI_LINT_VERSION := (v\d+\.\d+\.\d+)$`)
var workflowLintVersionPattern = regexp.MustCompile(`(?m)^\s+version:\s+["']?(v\d+\.\d+\.\d+)["']?\s*$`)

func TestLocalLintMatchesPinnedCIVersion(t *testing.T) {
	makefile := readRepositoryFile(t, "Makefile")
	workflow := readRepositoryFile(t, ".github/workflows/ci.yml")

	localVersion := requiredPatternMatch(t, makefileLintVersionPattern, makefile, "Makefile golangci-lint version")
	ciVersion := requiredPatternMatch(t, workflowLintVersionPattern, workflow, "CI golangci-lint version")
	if localVersion != ciVersion {
		t.Errorf("local golangci-lint version %s must match CI version %s", localVersion, ciVersion)
	}

	for _, required := range []string{
		"lint: fmt-check vet lint-ci",
		"lint-ci:",
		"command -v \"$(GOLANGCI_LINT)\"",
		"\"$(GOLANGCI_LINT)\" run",
		"go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)",
	} {
		if !strings.Contains(makefile, required) {
			t.Errorf("Makefile must contain %q", required)
		}
	}
}

func TestContributorDocsExplainPinnedLintSetup(t *testing.T) {
	makefile := readRepositoryFile(t, "Makefile")
	contributing := readRepositoryFile(t, "CONTRIBUTING.md")
	version := requiredPatternMatch(t, makefileLintVersionPattern, makefile, "Makefile golangci-lint version")

	for _, required := range []string{
		"make install-golangci-lint",
		"make lint",
		version,
	} {
		if !strings.Contains(contributing, required) {
			t.Errorf("CONTRIBUTING.md must document %q", required)
		}
	}
}

func requiredPatternMatch(t *testing.T, pattern *regexp.Regexp, content, description string) string {
	t.Helper()
	match := pattern.FindStringSubmatch(content)
	if len(match) != 2 {
		t.Fatalf("missing %s", description)
	}
	return match[1]
}
