// Package models provides a code generator for model constants from models.dev.
package models

// ModelData represents a model definition from models.dev.
type ModelData struct {
	ID               string         `toml:"-"` // Derived from filename
	Name             string         `toml:"name"`
	Attachment       bool           `toml:"attachment"`
	Reasoning        bool           `toml:"reasoning"`
	ToolCall         bool           `toml:"tool_call"`
	StructuredOutput bool           `toml:"structured_output"`
	Temperature      bool           `toml:"temperature"`
	Knowledge        string         `toml:"knowledge"`
	ReleaseDate      string         `toml:"release_date"`
	LastUpdated      string         `toml:"last_updated"`
	OpenWeights      bool           `toml:"open_weights"`
	Status           string         `toml:"status"` // alpha, beta, deprecated
	Cost             *CostData      `toml:"cost"`
	Limit            *LimitData     `toml:"limit"`
	Modalities       *ModalityData  `toml:"modalities"`
	Interleaved      any            `toml:"interleaved"` // bool or object
	API              string         `toml:"api"`         // Optional API endpoint override
}

// CostData represents pricing information.
type CostData struct {
	Input     float64 `toml:"input"`
	Output    float64 `toml:"output"`
	CacheRead float64 `toml:"cache_read"`
	Reasoning float64 `toml:"reasoning"`
}

// LimitData represents context and output limits.
type LimitData struct {
	Context   int `toml:"context"`
	Input     int `toml:"input"`
	Output    int `toml:"output"`
	Reasoning int `toml:"reasoning"`
}

// ModalityData represents supported input/output modalities.
type ModalityData struct {
	Input  []string `toml:"input"`
	Output []string `toml:"output"`
}

// ProviderData represents a provider configuration from models.dev.
type ProviderData struct {
	Name        string `toml:"name"`
	URL         string `toml:"url"`
	Description string `toml:"description"`
}

// ProviderMapping maps models.dev provider names to Iris provider packages.
var ProviderMapping = map[string]string{
	"openai":    "openai",
	"anthropic": "anthropic",
	"google":    "gemini",
	"ollama":    "ollama",
	"xai":       "xai",
	"perplexity": "perplexity",
	"huggingface": "huggingface",
}

// IrisFeature represents a feature constant name in Iris.
type IrisFeature string

const (
	FeatureChat             IrisFeature = "core.FeatureChat"
	FeatureChatStreaming    IrisFeature = "core.FeatureChatStreaming"
	FeatureToolCalling      IrisFeature = "core.FeatureToolCalling"
	FeatureReasoning        IrisFeature = "core.FeatureReasoning"
	FeatureStructuredOutput IrisFeature = "core.FeatureStructuredOutput"
	FeatureImageGeneration  IrisFeature = "core.FeatureImageGeneration"
	FeatureEmbeddings       IrisFeature = "core.FeatureEmbeddings"
)
