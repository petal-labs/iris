package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	githubAPIBase = "https://api.github.com/repos/sst/models.dev/contents"
)

// GitHubContent represents a file/directory from the GitHub API.
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
	DownloadURL string `json:"download_url"`
}

// Client fetches model data from models.dev GitHub repository.
type Client struct {
	httpClient *http.Client
	token      string // Optional GitHub token for higher rate limits
}

// NewClient creates a new models.dev client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithToken sets a GitHub API token for higher rate limits.
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

// FetchProviders returns the list of available providers.
func (c *Client) FetchProviders() ([]string, error) {
	url := fmt.Sprintf("%s/providers", githubAPIBase)
	contents, err := c.fetchContents(url)
	if err != nil {
		return nil, fmt.Errorf("fetch providers: %w", err)
	}

	var providers []string
	for _, item := range contents {
		if item.Type == "dir" {
			providers = append(providers, item.Name)
		}
	}
	return providers, nil
}

// FetchProviderModels fetches all models for a given provider.
func (c *Client) FetchProviderModels(provider string) ([]ModelData, error) {
	url := fmt.Sprintf("%s/providers/%s/models", githubAPIBase, provider)
	contents, err := c.fetchContents(url)
	if err != nil {
		return nil, fmt.Errorf("fetch models for %s: %w", provider, err)
	}

	var models []ModelData
	for _, item := range contents {
		if item.Type == "file" && strings.HasSuffix(item.Name, ".toml") {
			model, err := c.fetchModel(item.Path, item.Name)
			if err != nil {
				// Log but continue with other models
				fmt.Printf("Warning: failed to fetch model %s: %v\n", item.Name, err)
				continue
			}
			models = append(models, model)
		} else if item.Type == "dir" {
			// Handle nested directories (e.g., openai/gpt-5)
			subModels, err := c.fetchNestedModels(item.Path, item.Name)
			if err != nil {
				fmt.Printf("Warning: failed to fetch nested models from %s: %v\n", item.Path, err)
				continue
			}
			models = append(models, subModels...)
		}
	}

	return models, nil
}

// fetchNestedModels handles models in subdirectories.
func (c *Client) fetchNestedModels(path, prefix string) ([]ModelData, error) {
	url := fmt.Sprintf("%s/%s", githubAPIBase, path)
	contents, err := c.fetchContents(url)
	if err != nil {
		return nil, err
	}

	var models []ModelData
	for _, item := range contents {
		if item.Type == "file" && strings.HasSuffix(item.Name, ".toml") {
			// Construct full model ID with prefix
			modelID := prefix + "/" + strings.TrimSuffix(item.Name, ".toml")
			model, err := c.fetchModelWithID(item.Path, modelID)
			if err != nil {
				fmt.Printf("Warning: failed to fetch model %s: %v\n", modelID, err)
				continue
			}
			models = append(models, model)
		}
	}
	return models, nil
}

// fetchModel fetches and parses a single model TOML file.
func (c *Client) fetchModel(path, filename string) (ModelData, error) {
	modelID := strings.TrimSuffix(filename, ".toml")
	return c.fetchModelWithID(path, modelID)
}

// fetchModelWithID fetches and parses a model with a specific ID.
func (c *Client) fetchModelWithID(path, modelID string) (ModelData, error) {
	url := fmt.Sprintf("%s/%s", githubAPIBase, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ModelData{}, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ModelData{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ModelData{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var content GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return ModelData{}, err
	}

	// Decode base64 content
	data, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return ModelData{}, fmt.Errorf("decode base64: %w", err)
	}

	var model ModelData
	if _, err := toml.Decode(string(data), &model); err != nil {
		return ModelData{}, fmt.Errorf("parse TOML: %w", err)
	}

	model.ID = modelID
	return model, nil
}

// fetchContents fetches a directory listing from GitHub API.
func (c *Client) fetchContents(url string) ([]GitHubContent, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var contents []GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	return contents, nil
}

// setHeaders sets common request headers.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "iris-gen-models")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// LoadLocalModels loads model definitions from local TOML files.
// The directory should contain TOML files named by model ID.
func LoadLocalModels(dir string) ([]ModelData, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var models []ModelData
	for _, entry := range entries {
		if entry.IsDir() {
			// Handle nested directories
			subDir := filepath.Join(dir, entry.Name())
			subModels, err := loadNestedLocalModels(subDir, entry.Name())
			if err != nil {
				fmt.Printf("Warning: failed to load models from %s: %v\n", subDir, err)
				continue
			}
			models = append(models, subModels...)
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		model, err := loadLocalModel(filepath.Join(dir, entry.Name()), strings.TrimSuffix(entry.Name(), ".toml"))
		if err != nil {
			fmt.Printf("Warning: failed to load %s: %v\n", entry.Name(), err)
			continue
		}
		models = append(models, model)
	}

	return models, nil
}

// loadNestedLocalModels loads models from a subdirectory with a prefix.
func loadNestedLocalModels(dir, prefix string) ([]ModelData, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var models []ModelData
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		modelID := prefix + "/" + strings.TrimSuffix(entry.Name(), ".toml")
		model, err := loadLocalModel(filepath.Join(dir, entry.Name()), modelID)
		if err != nil {
			fmt.Printf("Warning: failed to load %s: %v\n", entry.Name(), err)
			continue
		}
		models = append(models, model)
	}

	return models, nil
}

// loadLocalModel loads a single model from a TOML file.
func loadLocalModel(path, modelID string) (ModelData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ModelData{}, err
	}

	var model ModelData
	if _, err := toml.Decode(string(data), &model); err != nil {
		return ModelData{}, fmt.Errorf("parse TOML: %w", err)
	}

	model.ID = modelID
	return model, nil
}
