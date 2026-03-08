package models

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// Generator creates Go source files from model data.
type Generator struct {
	packageName string
	provider    string
}

// NewGenerator creates a new code generator for a provider.
func NewGenerator(provider, packageName string) *Generator {
	return &Generator{
		provider:    provider,
		packageName: packageName,
	}
}

// GeneratedModel represents a model ready for code generation.
type GeneratedModel struct {
	ConstName    string
	ModelID      string
	DisplayName  string
	Capabilities []string
	APIEndpoint  string
	IsImageModel bool
}

// Generate produces Go source code for the given models.
func (g *Generator) Generate(models []ModelData) ([]byte, error) {
	// Convert to generated models
	genModels := g.convertModels(models)

	// Sort models for deterministic output
	sort.Slice(genModels, func(i, j int) bool {
		return genModels[i].ConstName < genModels[j].ConstName
	})

	// Execute template
	var buf bytes.Buffer
	if err := modelTemplate.Execute(&buf, templateData{
		Package: g.packageName,
		Models:  genModels,
	}); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code with error for debugging
		return buf.Bytes(), fmt.Errorf("format source: %w", err)
	}

	return formatted, nil
}

// convertModels transforms ModelData to GeneratedModel.
func (g *Generator) convertModels(models []ModelData) []GeneratedModel {
	var result []GeneratedModel
	for _, m := range models {
		gm := GeneratedModel{
			ConstName:   toConstName(m.ID),
			ModelID:     m.ID,
			DisplayName: m.Name,
		}

		// Determine capabilities based on model attributes
		gm.Capabilities = g.mapCapabilities(m)

		// Determine API endpoint (provider-specific logic)
		gm.APIEndpoint = g.mapAPIEndpoint(m)

		// Check if this is an image model
		gm.IsImageModel = isImageModel(m)

		result = append(result, gm)
	}
	return result
}

// mapCapabilities converts models.dev attributes to Iris features.
func (g *Generator) mapCapabilities(m ModelData) []string {
	var caps []string

	// Image models only have image generation capability
	if isImageModel(m) {
		return []string{"core.FeatureImageGeneration"}
	}

	// All chat models have basic chat capability
	caps = append(caps, "core.FeatureChat", "core.FeatureChatStreaming")

	// Tool calling
	if m.ToolCall {
		caps = append(caps, "core.FeatureToolCalling")
	}

	// Reasoning
	if m.Reasoning {
		caps = append(caps, "core.FeatureReasoning")
	}

	// Structured output
	if m.StructuredOutput {
		caps = append(caps, "core.FeatureStructuredOutput")
	}

	return caps
}

// mapAPIEndpoint determines the API endpoint for a model.
func (g *Generator) mapAPIEndpoint(m ModelData) string {
	// Image models don't need API endpoint specification
	if isImageModel(m) {
		return ""
	}

	// Provider-specific API endpoint logic
	switch g.provider {
	case "openai":
		return g.mapOpenAIEndpoint(m)
	default:
		return ""
	}
}

// mapOpenAIEndpoint maps OpenAI models to their API endpoints.
func (g *Generator) mapOpenAIEndpoint(m ModelData) string {
	id := strings.ToLower(m.ID)

	// GPT-5.x, GPT-4.1, and O-series use Responses API
	if strings.HasPrefix(id, "gpt-5") ||
		strings.HasPrefix(id, "gpt-4.1") ||
		strings.HasPrefix(id, "o1") ||
		strings.HasPrefix(id, "o3") ||
		strings.HasPrefix(id, "o4") {
		return "core.APIEndpointResponses"
	}

	// Default to completions API
	return "core.APIEndpointCompletions"
}

// isImageModel determines if a model is an image generation model.
func isImageModel(m ModelData) bool {
	id := strings.ToLower(m.ID)

	// Check by model ID patterns
	if strings.Contains(id, "dall-e") ||
		strings.Contains(id, "image") ||
		strings.Contains(id, "imagen") {
		return true
	}

	// Check by output modalities
	if m.Modalities != nil {
		for _, out := range m.Modalities.Output {
			if out == "image" {
				return true
			}
		}
	}

	return false
}

// toConstName converts a model ID to a Go constant name.
// e.g., "gpt-4o-mini" -> "ModelGPT4oMini"
func toConstName(id string) string {
	// Replace special characters with separators
	normalized := strings.NewReplacer(
		"-", " ",
		"_", " ",
		".", " ",
		"/", " ",
	).Replace(id)

	// Split into words and capitalize each
	words := strings.Fields(normalized)
	var parts []string
	for _, word := range words {
		parts = append(parts, capitalize(word))
	}

	return "Model" + strings.Join(parts, "")
}

// capitalize capitalizes a word, handling special cases.
// The convention follows existing Iris code: GPT, AI, API stay uppercase,
// but words like Mini, Pro, Nano are title-cased.
func capitalize(s string) string {
	if s == "" {
		return s
	}

	upper := strings.ToUpper(s)

	// True acronyms stay uppercase
	switch upper {
	case "GPT", "AI", "API", "LLM", "ID":
		return upper
	}

	// Handle numbers (keep as-is)
	if len(s) > 0 && unicode.IsDigit(rune(s[0])) {
		return s
	}

	// Title case: first letter uppercase, rest lowercase
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

type templateData struct {
	Package string
	Models  []GeneratedModel
}

var modelTemplate = template.Must(template.New("models").Parse(`// Code generated by gen-models. DO NOT EDIT.
// Source: https://github.com/sst/models.dev

package {{.Package}}

import "github.com/petal-labs/iris/core"

// Model constants generated from models.dev.
const (
{{- range .Models}}
	{{.ConstName}} core.ModelID = "{{.ModelID}}"
{{- end}}
)

// models is the static list of supported models.
var models = []core.ModelInfo{
{{- range .Models}}
	{
		ID:          {{.ConstName}},
		DisplayName: "{{.DisplayName}}",
{{- if .APIEndpoint}}
		APIEndpoint: {{.APIEndpoint}},
{{- end}}
		Capabilities: []core.Feature{
{{- range .Capabilities}}
			{{.}},
{{- end}}
		},
	},
{{- end}}
}

// modelRegistry is a map for quick model lookup by ID.
var modelRegistry = buildModelRegistry()

// buildModelRegistry creates a map from model ID to ModelInfo.
func buildModelRegistry() map[core.ModelID]*core.ModelInfo {
	registry := make(map[core.ModelID]*core.ModelInfo, len(models))
	for i := range models {
		registry[models[i].ID] = &models[i]
	}
	return registry
}

// GetModelInfo returns the ModelInfo for a given model ID, or nil if not found.
func GetModelInfo(id core.ModelID) *core.ModelInfo {
	return modelRegistry[id]
}
`))
