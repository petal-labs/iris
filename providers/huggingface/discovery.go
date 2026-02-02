package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ModelStatus represents the inference status of a model on Hugging Face.
type ModelStatus string

const (
	// ModelStatusWarm indicates the model has available inference providers.
	ModelStatusWarm ModelStatus = "warm"

	// ModelStatusUnknown indicates no inference provider is available.
	ModelStatusUnknown ModelStatus = ""
)

// InferenceProvider represents a provider serving a specific model.
type InferenceProvider struct {
	// Name is the provider identifier (e.g., "cerebras", "together", "groq").
	Name string

	// ProviderID is the provider's internal model identifier.
	ProviderID string

	// Status is the provider status: "live" or "staging".
	Status string

	// Task is the supported task type (e.g., "conversational").
	Task string
}

// HubModelInfo represents model information from the Hub API.
type HubModelInfo struct {
	// ID is the model identifier (e.g., "meta-llama/Llama-3-8B-Instruct").
	ID string

	// PipelineTag is the model's task type (e.g., "text-generation").
	PipelineTag string

	// Inference is the inference status ("warm" or empty).
	Inference string
}

// ListModelsOptions configures the ListModels query.
type ListModelsOptions struct {
	// Provider filters by inference provider.
	// Use "all" for any provider, or a specific provider name (e.g., "fireworks-ai").
	Provider string

	// PipelineTag filters by task type (e.g., "text-generation", "text-to-image").
	PipelineTag string

	// Limit is the maximum number of results to return.
	Limit int
}

// hubModelInfoResponse represents the response from the models API.
type hubModelInfoResponse struct {
	ID          string `json:"id"`
	PipelineTag string `json:"pipeline_tag"`
	Inference   string `json:"inference"`
}

// hubModelDetailResponse represents the detailed model info response.
type hubModelDetailResponse struct {
	ID                       string                              `json:"id"`
	Inference                string                              `json:"inference"`
	InferenceProviderMapping map[string]hubInferenceProviderInfo `json:"inferenceProviderMapping"`
}

// hubInferenceProviderInfo represents provider info from the Hub API.
type hubInferenceProviderInfo struct {
	Status     string `json:"status"`
	ProviderID string `json:"providerId"`
	Task       string `json:"task"`
}

// hubAPIURL constructs a URL for the Hub API.
func (p *HuggingFace) hubAPIURL(path string) string {
	return HubAPIBaseURL + path
}

// GetModelStatus checks if a model has available inference providers.
// Returns ModelStatusWarm if providers are available, ModelStatusUnknown otherwise.
func (p *HuggingFace) GetModelStatus(ctx context.Context, modelID string) (ModelStatus, error) {
	// Model IDs contain slashes (e.g., "google/gemma-2-2b-it") which should NOT be encoded
	apiURL := p.hubAPIURL("/models/" + modelID + "?expand=inference")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ModelStatusUnknown, newNetworkError(err)
	}

	// Set authorization header
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return ModelStatusUnknown, newNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return ModelStatusUnknown, normalizeError(resp.StatusCode, body, "")
	}

	var result hubModelDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ModelStatusUnknown, newDecodeError(err)
	}

	if result.Inference == "warm" {
		return ModelStatusWarm, nil
	}

	return ModelStatusUnknown, nil
}

// GetModelProviders returns the list of providers serving a specific model.
func (p *HuggingFace) GetModelProviders(ctx context.Context, modelID string) ([]InferenceProvider, error) {
	// Model IDs contain slashes (e.g., "google/gemma-2-2b-it") which should NOT be encoded
	apiURL := p.hubAPIURL("/models/" + modelID + "?expand=inferenceProviderMapping")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Set authorization header
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, normalizeError(resp.StatusCode, body, "")
	}

	var result hubModelDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, newDecodeError(err)
	}

	providers := make([]InferenceProvider, 0, len(result.InferenceProviderMapping))
	for name, info := range result.InferenceProviderMapping {
		providers = append(providers, InferenceProvider{
			Name:       name,
			ProviderID: info.ProviderID,
			Status:     info.Status,
			Task:       info.Task,
		})
	}

	return providers, nil
}

// ListModels queries available models with optional filters.
func (p *HuggingFace) ListModels(ctx context.Context, opts ListModelsOptions) ([]HubModelInfo, error) {
	// Build query parameters
	params := url.Values{}

	if opts.Provider != "" {
		params.Set("inference_provider", opts.Provider)
	}

	if opts.PipelineTag != "" {
		params.Set("pipeline_tag", opts.PipelineTag)
	}

	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	}

	apiURL := p.hubAPIURL("/models")
	if len(params) > 0 {
		apiURL = apiURL + "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Set authorization header
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, normalizeError(resp.StatusCode, body, "")
	}

	var results []hubModelInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, newDecodeError(err)
	}

	models := make([]HubModelInfo, len(results))
	for i, r := range results {
		models[i] = HubModelInfo(r)
	}

	return models, nil
}

// String returns a human-readable representation of the model status.
func (s ModelStatus) String() string {
	if s == ModelStatusWarm {
		return "warm"
	}
	return "unknown"
}

// String returns a human-readable representation of the provider.
func (p InferenceProvider) String() string {
	return fmt.Sprintf("%s (%s, %s)", p.Name, p.Status, p.Task)
}

// IsLive returns true if the provider status is "live".
func (p InferenceProvider) IsLive() bool {
	return strings.EqualFold(p.Status, "live")
}
