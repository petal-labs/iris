package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
	_ "github.com/petal-labs/iris/providers/anthropic"
	_ "github.com/petal-labs/iris/providers/azurefoundry"
	_ "github.com/petal-labs/iris/providers/gemini"
	_ "github.com/petal-labs/iris/providers/huggingface"
	_ "github.com/petal-labs/iris/providers/ollama"
	_ "github.com/petal-labs/iris/providers/openai"
	_ "github.com/petal-labs/iris/providers/perplexity"
	_ "github.com/petal-labs/iris/providers/voyageai"
	_ "github.com/petal-labs/iris/providers/xai"
	_ "github.com/petal-labs/iris/providers/zai"
)

// TestProvidersDocExists verifies PROVIDERS.md exists and contains required sections.
func TestProvidersDocExists(t *testing.T) {
	content := readDocFile(t, "PROVIDERS.md")

	requiredSections := []string{
		"# Provider Comparison",
		"## Feature Support Matrix",
		"## Provider Details",
		"### OpenAI",
		"### Anthropic",
		"### Google Gemini",
		"### xAI (Grok)",
		"### Perplexity",
		"### Z.ai (GLM)",
		"### Ollama",
		"### HuggingFace",
		"### VoyageAI",
		"### Azure AI Foundry",
		"## Choosing a Provider",
		"## Rate Limits and Pricing",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("PROVIDERS.md missing required section: %q", section)
		}
	}

	// Verify feature matrix table exists
	if !strings.Contains(content, "| Provider |") {
		t.Error("PROVIDERS.md missing feature support matrix table")
	}

	// Verify code examples exist for major providers
	providers := []string{"openai", "anthropic", "gemini", "ollama"}
	for _, p := range providers {
		if !strings.Contains(strings.ToLower(content), "```go") {
			t.Errorf("PROVIDERS.md missing Go code examples")
			break
		}
		if !strings.Contains(strings.ToLower(content), p+".new") {
			t.Errorf("PROVIDERS.md missing usage example for %s provider", p)
		}
	}
}

// matrixColumns maps the PROVIDERS.md feature-matrix columns to the core
// Feature constants each represents.
var matrixColumns = []struct {
	column  string
	feature core.Feature
}{
	{"Chat", core.FeatureChat},
	{"Streaming", core.FeatureChatStreaming},
	{"Tool Calling", core.FeatureToolCalling},
	{"Reasoning", core.FeatureReasoning},
	{"Built-in Tools", core.FeatureBuiltInTools},
	{"Response Chain", core.FeatureResponseChain},
	{"Structured Output", core.FeatureStructuredOutput},
	{"Embeddings", core.FeatureEmbeddings},
	{"Reranking", core.FeatureReranking},
	{"Images", core.FeatureImageGeneration},
}

// docMatrixProviderID maps the PROVIDERS.md display names to registry IDs.
var docMatrixProviderID = map[string]string{
	"OpenAI":           "openai",
	"Anthropic":        "anthropic",
	"Gemini":           "gemini",
	"xAI (Grok)":       "xai",
	"Perplexity":       "perplexity",
	"Z.ai (GLM)":       "zai",
	"Ollama":           "ollama",
	"HuggingFace":      "huggingface",
	"Azure AI Foundry": "azurefoundry",
	"VoyageAI":         "voyageai",
}

// TestProvidersDocMatrixAccuracy verifies the PROVIDERS.md feature matrix
// matches each provider's actual Supports() implementation, so the docs
// cannot silently drift from the code.
func TestProvidersDocMatrixAccuracy(t *testing.T) {
	content := readDocFile(t, "PROVIDERS.md")
	matrix := featureMatrixSection(t, content)

	// Parse matrix rows: | <Provider> | Yes | No | ... |
	// Note: [^|\n] — without excluding newlines the greedy inner group
	// swallows the whole table into a single match.
	rowRe := regexp.MustCompile(`(?m)^\| ([^|\n]+) \|((?:[^|\n]+\|)+)$`)
	cells := map[string][]string{}
	for _, match := range rowRe.FindAllStringSubmatch(matrix, -1) {
		name := strings.TrimSpace(match[1])
		if _, ok := docMatrixProviderID[name]; !ok {
			continue // header or separator row
		}
		var row []string
		for _, cell := range strings.Split(strings.TrimSuffix(match[2], "|"), "|") {
			row = append(row, strings.TrimSpace(cell))
		}
		cells[name] = row
	}

	if len(cells) != len(docMatrixProviderID) {
		t.Errorf("matrix has %d provider rows, want %d (missing providers: %v)",
			len(cells), len(docMatrixProviderID), missingMatrixRows(cells))
	}

	header := matrixHeader(matrix)
	if len(header) == 0 {
		t.Fatal("could not parse feature matrix header row")
	}

	for displayName, id := range docMatrixProviderID {
		row, ok := cells[displayName]
		if !ok {
			t.Errorf("matrix missing row for %q", displayName)
			continue
		}

		// Instantiate the provider through the registry with a dummy key;
		// Supports() does not depend on the key.
		provider, err := providers.Create(id, "docs-test-key")
		if err != nil {
			t.Errorf("provider %q is not registered: %v", id, err)
			continue
		}

		for _, col := range matrixColumns {
			idx := headerIndex(header, col.column)
			if idx < 0 {
				t.Errorf("matrix has no %q column", col.column)
				continue
			}
			// Header index 0 is the "Provider" name column; row cells are
			// everything after it, so shift by one.
			cellIdx := idx - 1
			if cellIdx < 0 || cellIdx >= len(row) {
				t.Errorf("matrix row for %q has no %q cell", displayName, col.column)
				continue
			}

			supported := provider.Supports(col.feature)
			cell := row[cellIdx]
			switch strings.TrimSuffix(cell, "†") { // gated-sentinel footnote marker
			case "Yes":
				// Bare Yes asserts a provider-level capability.
				if !supported {
					t.Errorf("matrix cell %s/%s = %q, but Supports(%s) = false (use \"Yes*\" if model-dependent)",
						displayName, col.column, cell, col.feature)
				}
			case "Yes*":
				// Starred Yes asserts the capability exists for at least one
				// model in the provider's catalog, even if not declared at
				// provider level.
				if !supported && !anyModelHasCapability(provider, col.feature) {
					t.Errorf("matrix cell %s/%s = %q, but Supports(%s) = false and no catalog model has the capability",
						displayName, col.column, cell, col.feature)
				}
			case "No":
				if supported {
					t.Errorf("matrix cell %s/%s = %q, want \"Yes\" (Supports(%s) = true)",
						displayName, col.column, cell, col.feature)
				}
			default:
				// "N/A" and anything else is provider-specific prose; not
				// machine-checked.
			}
		}
	}
}

// anyModelHasCapability reports whether any model in the provider's catalog
// declares the feature.
func anyModelHasCapability(p core.Provider, feature core.Feature) bool {
	for _, m := range p.Models() {
		if m.HasCapability(feature) {
			return true
		}
	}
	return false
}

func missingMatrixRows(cells map[string][]string) []string {
	var missing []string
	for name := range docMatrixProviderID {
		if _, ok := cells[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// featureMatrixSection returns the body of the "## Feature Support Matrix"
// section. The matrix is parsed by matching any table row whose first cell is
// a provider display name, so the parse must be scoped: PROVIDERS.md carries
// other provider-keyed tables (the "Supported Providers" status table) whose
// rows would otherwise be mistaken for matrix rows and silently override them.
func featureMatrixSection(t *testing.T, content string) string {
	t.Helper()

	const heading = "## Feature Support Matrix"
	start := strings.Index(content, heading)
	if start < 0 {
		t.Fatalf("PROVIDERS.md missing %q section", heading)
	}

	body := content[start+len(heading):]
	if end := strings.Index(body, "\n## "); end >= 0 {
		body = body[:end]
	}
	return body
}

func matrixHeader(content string) []string {
	re := regexp.MustCompile(`(?m)^\| Provider \|([^|\n]+\|)+$`)
	match := re.FindString(content)
	if match == "" {
		return nil
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(match, "|"), "|")
	var header []string
	for _, cell := range strings.Split(trimmed, "|") {
		header = append(header, strings.TrimSpace(cell))
	}
	return header
}

func headerIndex(header []string, column string) int {
	for i, h := range header {
		if h == column {
			return i
		}
	}
	return -1
}

// TestArchitectureDocExists verifies ARCHITECTURE.md exists and contains required sections.
func TestArchitectureDocExists(t *testing.T) {
	content := readDocFile(t, "ARCHITECTURE.md")

	requiredSections := []string{
		"# Architecture Design Decisions",
		"## Why Streaming Is First-Class",
		"## Why Provider Is an Interface",
		"## Why ChatBuilder Is Not Thread-Safe",
		"## Why Tools Use json.RawMessage",
		"## Why Sentinel Errors",
		"## Why Exponential Backoff",
		"## Why Features Are Explicit",
		"## Summary of Design Principles",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("ARCHITECTURE.md missing required section: %q", section)
		}
	}

	// Verify each section has rationale
	if strings.Count(content, "### Rationale") < 5 {
		t.Error("ARCHITECTURE.md should have Rationale subsections for design decisions")
	}

	// Verify alternatives considered are documented
	if strings.Count(content, "### Alternatives Considered") < 3 {
		t.Error("ARCHITECTURE.md should document alternatives considered for major decisions")
	}

	// Verify code examples are included
	if !strings.Contains(content, "```go") {
		t.Error("ARCHITECTURE.md should include Go code examples")
	}
}

// TestSecurityDocExists verifies SECURITY.md exists and contains required sections.
func TestSecurityDocExists(t *testing.T) {
	content := readDocFile(t, "SECURITY.md")

	requiredSections := []string{
		"# Security Guide",
		"## Keystore Encryption",
		"### Creating an Encryption Key",
		"### How It Works",
		"### Storing API Keys",
		"## Secret Type",
		"## Telemetry Security",
		"## CI/CD Best Practices",
		"## Security Checklist",
		"## Cryptographic Details",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("SECURITY.md missing required section: %q", section)
		}
	}

	// Verify key technical details are documented
	technicalTerms := []string{
		"IRIS_KEYSTORE_KEY",
		"Argon2id",
		"AES-256-GCM",
		"core.Secret",
	}
	for _, term := range technicalTerms {
		if !strings.Contains(content, term) {
			t.Errorf("SECURITY.md should document %s", term)
		}
	}

	// Verify code examples are included
	if !strings.Contains(content, "```bash") {
		t.Error("SECURITY.md should include bash examples for key setup")
	}
	if !strings.Contains(content, "```go") {
		t.Error("SECURITY.md should include Go code examples")
	}

	// Verify V1 vs V2 is explained
	if !strings.Contains(content, "V1") || !strings.Contains(content, "V2") {
		t.Error("SECURITY.md should explain V1 vs V2 keystore formats")
	}
}

// TestCoreDocGoExists verifies core/doc.go has comprehensive package documentation.
func TestCoreDocGoExists(t *testing.T) {
	content := readCoreDocFile(t)

	requiredSections := []string{
		"Package core provides",
		"# Client and Provider",
		"# ChatBuilder",
		"# Streaming",
		"# Provider Interface",
		"# Features",
		"# Error Handling",
		"# Telemetry",
		"# Retry Policy",
		"# Thread Safety",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("core/doc.go missing required section: %q", section)
		}
	}

	// Verify examples are included
	if !strings.Contains(content, "provider :=") {
		t.Error("core/doc.go should include provider creation example")
	}
	if !strings.Contains(content, "client.Chat(") {
		t.Error("core/doc.go should include Chat usage example")
	}

	// Verify feature constants are documented
	features := []string{
		"FeatureChat",
		"FeatureChatStreaming",
		"FeatureToolCalling",
		"FeatureReasoning",
	}
	for _, f := range features {
		if !strings.Contains(content, f) {
			t.Errorf("core/doc.go should document %s feature", f)
		}
	}

	// Verify error constants are documented
	errors := []string{
		"ErrUnauthorized",
		"ErrRateLimited",
		"ErrBadRequest",
		"ErrModelRequired",
	}
	for _, e := range errors {
		if !strings.Contains(content, e) {
			t.Errorf("core/doc.go should document %s error", e)
		}
	}
}

// readDocFile reads a file from the docs directory.
func readDocFile(t *testing.T, filename string) string {
	t.Helper()

	path := filepath.Join("..", "docs", filename)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", filename, err)
	}

	return string(content)
}

// readCoreDocFile reads the core/doc.go file.
func readCoreDocFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "core", "doc.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read core/doc.go: %v", err)
	}

	return string(content)
}
