package models

import (
	"strings"
	"testing"
)

func TestToConstName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gpt-4o", "ModelGPT4o"},
		{"gpt-4o-mini", "ModelGPT4oMini"},
		{"gpt-3.5-turbo", "ModelGPT35Turbo"},
		{"o1", "ModelO1"},
		{"o3-mini", "ModelO3Mini"},
		{"dall-e-3", "ModelDallE3"},
		{"claude-3-opus", "ModelClaude3Opus"},
		{"gemini-2.5-flash", "ModelGemini25Flash"},
		{"gpt-4.1-nano", "ModelGPT41Nano"},
		{"text-embedding-ada-002", "ModelTextEmbeddingAda002"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := toConstName(tc.input)
			if result != tc.expected {
				t.Errorf("toConstName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gpt", "GPT"},
		{"api", "API"},
		{"mini", "Mini"},
		{"turbo", "Turbo"},
		{"flash", "Flash"},
		{"pro", "Pro"},
		{"4o", "4o"},
		{"35", "35"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := capitalize(tc.input)
			if result != tc.expected {
				t.Errorf("capitalize(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsImageModel(t *testing.T) {
	tests := []struct {
		model    ModelData
		expected bool
	}{
		{
			model:    ModelData{ID: "dall-e-3"},
			expected: true,
		},
		{
			model:    ModelData{ID: "gpt-image-1"},
			expected: true,
		},
		{
			model:    ModelData{ID: "gpt-4o"},
			expected: false,
		},
		{
			model: ModelData{
				ID:         "some-model",
				Modalities: &ModalityData{Output: []string{"image"}},
			},
			expected: true,
		},
		{
			model: ModelData{
				ID:         "some-model",
				Modalities: &ModalityData{Output: []string{"text"}},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.model.ID, func(t *testing.T) {
			result := isImageModel(tc.model)
			if result != tc.expected {
				t.Errorf("isImageModel(%q) = %v, want %v", tc.model.ID, result, tc.expected)
			}
		})
	}
}

func TestMapCapabilities(t *testing.T) {
	gen := NewGenerator("openai", "openai")

	tests := []struct {
		name     string
		model    ModelData
		expected []string
	}{
		{
			name: "basic chat model",
			model: ModelData{
				ID:       "gpt-4o",
				ToolCall: true,
			},
			expected: []string{"core.FeatureChat", "core.FeatureChatStreaming", "core.FeatureToolCalling"},
		},
		{
			name: "reasoning model",
			model: ModelData{
				ID:        "o1",
				ToolCall:  true,
				Reasoning: true,
			},
			expected: []string{"core.FeatureChat", "core.FeatureChatStreaming", "core.FeatureToolCalling", "core.FeatureReasoning"},
		},
		{
			name: "structured output model",
			model: ModelData{
				ID:               "gpt-4o",
				ToolCall:         true,
				StructuredOutput: true,
			},
			expected: []string{"core.FeatureChat", "core.FeatureChatStreaming", "core.FeatureToolCalling", "core.FeatureStructuredOutput"},
		},
		{
			name: "image model",
			model: ModelData{
				ID:         "dall-e-3",
				Modalities: &ModalityData{Output: []string{"image"}},
			},
			expected: []string{"core.FeatureImageGeneration"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := gen.mapCapabilities(tc.model)
			if len(result) != len(tc.expected) {
				t.Errorf("mapCapabilities() returned %d capabilities, want %d", len(result), len(tc.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Want: %v", tc.expected)
				return
			}
			for i, cap := range result {
				if cap != tc.expected[i] {
					t.Errorf("mapCapabilities()[%d] = %q, want %q", i, cap, tc.expected[i])
				}
			}
		})
	}
}

func TestMapAPIEndpoint(t *testing.T) {
	gen := NewGenerator("openai", "openai")

	tests := []struct {
		modelID  string
		expected string
	}{
		{"gpt-4o", "core.APIEndpointCompletions"},
		{"gpt-4o-mini", "core.APIEndpointCompletions"},
		{"gpt-4-turbo", "core.APIEndpointCompletions"},
		{"gpt-4.1", "core.APIEndpointResponses"},
		{"gpt-4.1-mini", "core.APIEndpointResponses"},
		{"gpt-5", "core.APIEndpointResponses"},
		{"gpt-5.2-codex", "core.APIEndpointResponses"},
		{"o1", "core.APIEndpointResponses"},
		{"o3-mini", "core.APIEndpointResponses"},
		{"o4-mini", "core.APIEndpointResponses"},
	}

	for _, tc := range tests {
		t.Run(tc.modelID, func(t *testing.T) {
			model := ModelData{ID: tc.modelID}
			result := gen.mapAPIEndpoint(model)
			if result != tc.expected {
				t.Errorf("mapAPIEndpoint(%q) = %q, want %q", tc.modelID, result, tc.expected)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	gen := NewGenerator("openai", "openai")

	models := []ModelData{
		{
			ID:               "gpt-4o",
			Name:             "GPT-4o",
			ToolCall:         true,
			StructuredOutput: true,
		},
		{
			ID:         "dall-e-3",
			Name:       "DALL-E 3",
			Modalities: &ModalityData{Output: []string{"image"}},
		},
	}

	code, err := gen.Generate(models)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Verify code contains expected elements
	codeStr := string(code)

	expectedPatterns := []string{
		"// Code generated by gen-models. DO NOT EDIT.",
		"package openai",
		"ModelDallE3",
		"ModelGPT4o",
		"core.FeatureImageGeneration",
		"core.FeatureToolCalling",
		"func buildModelRegistry()",
		"func GetModelInfo(",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(codeStr, pattern) {
			t.Errorf("Generated code missing expected pattern: %q", pattern)
		}
	}
}
