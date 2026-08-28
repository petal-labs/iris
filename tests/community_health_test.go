package tests

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type issueForm struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Body        []issueFormItem `yaml:"body"`
}

type issueFormItem struct {
	Type        string          `yaml:"type"`
	Validations map[string]bool `yaml:"validations"`
}

func TestCodeOwnersRoutesRepositoryReviews(t *testing.T) {
	codeowners := readRepositoryFile(t, ".github/CODEOWNERS")
	if !containsTrimmedLine(codeowners, "* @erikhoward") {
		t.Error("CODEOWNERS must route repository-wide reviews to @erikhoward")
	}
}

func TestIssueFormsCollectRequiredContext(t *testing.T) {
	for _, path := range []string{
		".github/ISSUE_TEMPLATE/bug.yml",
		".github/ISSUE_TEMPLATE/feature.yml",
	} {
		content := readRepositoryFile(t, path)
		var form issueForm
		if err := yaml.Unmarshal([]byte(content), &form); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.TrimSpace(form.Name) == "" || strings.TrimSpace(form.Description) == "" {
			t.Errorf("%s must define a name and description", path)
		}
		if !hasRequiredTextarea(form.Body) {
			t.Errorf("%s must require at least one detailed response", path)
		}
	}
}

func TestIssueTemplateConfigRoutesSecurityReportsPrivately(t *testing.T) {
	config := readRepositoryFile(t, ".github/ISSUE_TEMPLATE/config.yml")
	for _, required := range []string{
		"blank_issues_enabled: false",
		"https://github.com/petal-labs/iris/security/advisories/new",
	} {
		if !strings.Contains(config, required) {
			t.Errorf("issue template config must contain %q", required)
		}
	}
}

func TestPullRequestTemplateIncludesProjectChecklist(t *testing.T) {
	template := readRepositoryFile(t, ".github/PULL_REQUEST_TEMPLATE.md")
	for _, required := range []string{
		"## Summary",
		"## Testing",
		"docs/changes/",
		"- [ ]",
	} {
		if !strings.Contains(template, required) {
			t.Errorf("pull request template must contain %q", required)
		}
	}
}

func TestContributorCodeOfConductIsLinkedAndConfigured(t *testing.T) {
	conduct := readRepositoryFile(t, "CODE_OF_CONDUCT.md")
	if !strings.Contains(conduct, "Contributor Covenant Code of Conduct") {
		t.Error("CODE_OF_CONDUCT.md must use the Contributor Covenant")
	}
	if strings.Contains(conduct, "[INSERT CONTACT METHOD]") {
		t.Error("CODE_OF_CONDUCT.md must define a real enforcement contact")
	}

	contributing := readRepositoryFile(t, "CONTRIBUTING.md")
	if !strings.Contains(contributing, "[Code of Conduct](CODE_OF_CONDUCT.md)") {
		t.Error("CONTRIBUTING.md must link to CODE_OF_CONDUCT.md")
	}
}

func TestSecurityReportingUsesPrivateVulnerabilityReporting(t *testing.T) {
	security := readRepositoryFile(t, "docs/SECURITY.md")
	if !strings.Contains(security, "https://github.com/petal-labs/iris/security/advisories/new") {
		t.Error("security reporting must link to GitHub private vulnerability reporting")
	}
	if !strings.Contains(security, "Do not open a public GitHub issue") {
		t.Error("security reporting must warn against public disclosure")
	}
}

func hasRequiredTextarea(items []issueFormItem) bool {
	for _, item := range items {
		if item.Type == "textarea" && item.Validations["required"] {
			return true
		}
	}
	return false
}
