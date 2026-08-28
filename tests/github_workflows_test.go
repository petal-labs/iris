package tests

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var actionReferencePattern = regexp.MustCompile(`uses:\s+[^@\s]+@([^\s#]+)`)
var fullCommitSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func TestDependabotCoversActionsAndWorkspaceModules(t *testing.T) {
	config := readRepositoryFile(t, ".github/dependabot.yml")
	for _, required := range []string{
		`package-ecosystem: "github-actions"`,
		`package-ecosystem: "gomod"`,
		`directory: "/"`,
		`directory: "/examples"`,
		`directory: "/contrib/otel"`,
	} {
		if !strings.Contains(config, required) {
			t.Errorf("dependabot config must contain %q", required)
		}
	}
}

func TestWorkflowActionsUseFullCommitSHAs(t *testing.T) {
	for path, workflow := range readWorkflowFiles(t) {
		for _, match := range actionReferencePattern.FindAllStringSubmatch(workflow, -1) {
			if !fullCommitSHAPattern.MatchString(match[1]) {
				t.Errorf("%s action ref %q must be a full commit SHA", path, match[1])
			}
		}
	}
}

func TestWorkflowsDefaultToReadOnlyPermissions(t *testing.T) {
	for path, workflow := range readWorkflowFiles(t) {
		if !strings.Contains(workflow, "\npermissions:\n  contents: read\n") {
			t.Errorf("%s must set top-level contents: read permissions", path)
		}
	}
}

func TestPullRequestWorkflowsCancelSupersededRuns(t *testing.T) {
	for path, workflow := range readWorkflowFiles(t) {
		if !strings.Contains(workflow, "pull_request:") {
			continue
		}
		if !strings.Contains(workflow, "\nconcurrency:\n") || !strings.Contains(workflow, "cancel-in-progress: true") {
			t.Errorf("%s must cancel superseded pull-request runs", path)
		}
	}
}

func TestCodeQLAndMultiOSRaceCoverageConfigured(t *testing.T) {
	codeql := readRepositoryFile(t, ".github/workflows/codeql.yml")
	for _, required := range []string{
		"github/codeql-action/init@",
		"github/codeql-action/analyze@",
		"security-events: write",
	} {
		if !strings.Contains(codeql, required) {
			t.Errorf("CodeQL workflow must contain %q", required)
		}
	}

	ci := readRepositoryFile(t, ".github/workflows/ci.yml")
	if !strings.Contains(ci, "if: matrix.os == 'macos-latest'") || strings.Count(ci, "go test -race") < 2 {
		t.Error("CI must run race-enabled tests on Ubuntu and macOS")
	}
}

func TestCodecovEnforcesProjectAndPatchThresholds(t *testing.T) {
	config := readRepositoryFile(t, "codecov.yml")
	for _, required := range []string{"project:", "patch:", "target: 80%"} {
		if !strings.Contains(config, required) {
			t.Errorf("codecov.yml must contain %q", required)
		}
	}
}

func TestGitHubYAMLConfigurationParses(t *testing.T) {
	configs := readWorkflowFiles(t)
	for _, path := range []string{".github/dependabot.yml", "codecov.yml"} {
		configs[path] = readRepositoryFile(t, path)
	}
	for path, content := range configs {
		var parsed any
		if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
			t.Errorf("parse %s: %v", path, err)
		}
	}
}

func readWorkflowFiles(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("../.github/workflows/*.yml")
	if err != nil {
		t.Fatalf("list workflow files: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no GitHub workflows found")
	}
	workflows := make(map[string]string, len(paths))
	for _, path := range paths {
		repositoryPath := strings.TrimPrefix(path, "../")
		workflows[repositoryPath] = readRepositoryFile(t, repositoryPath)
	}
	return workflows
}
