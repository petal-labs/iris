package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

const (
	modelListPath             = "/api/tags"
	defaultDiscoveryTimeout   = 2 * time.Second
	maxModelListResponseBytes = 8 << 20
)

type ollamaModelListResponse struct {
	Models []ollamaModelEntry `json:"models"`
}

type ollamaModelEntry struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// ListModels returns the models currently installed on the configured Ollama
// instance. Ollama's tags API does not report per-model capabilities, so the
// returned capability lists are intentionally empty.
func (p *Ollama) ListModels(ctx context.Context) ([]core.ModelInfo, error) {
	if ctx == nil {
		return nil, fmt.Errorf("ollama: model listing context cannot be nil")
	}
	ctx, cancel := timeoutx.Apply(ctx, p.config.Timeout)
	defer cancel()

	resp, err := p.sendModelListRequest(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeModelListResponse(resp)
}

func (p *Ollama) sendModelListRequest(ctx context.Context) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, modelListEndpoint(p.config.BaseURL), nil)
	if err != nil {
		return nil, newNetworkError(err)
	}
	for key, values := range p.buildHeaders() {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}
	return resp, nil
}

func decodeModelListResponse(resp *http.Response) ([]core.ModelInfo, error) {
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, parseErrorResponse(resp)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelListResponseBytes+1))
	if err != nil {
		return nil, newNetworkError(err)
	}
	if len(body) > maxModelListResponseBytes {
		return nil, newDecodeError(fmt.Errorf("model list response exceeds %d bytes", maxModelListResponseBytes))
	}
	var list ollamaModelListResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, newDecodeErrorWithBody(err, body)
	}

	return mapModelList(list.Models), nil
}

func mapModelList(entries []ollamaModelEntry) []core.ModelInfo {
	models := make([]core.ModelInfo, 0, len(entries))
	for _, entry := range entries {
		id := entry.Name
		if id == "" {
			id = entry.Model
		}
		if id == "" {
			continue
		}
		models = append(models, core.ModelInfo{ID: core.ModelID(id), DisplayName: id})
	}
	return models
}

func modelListEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/api") {
		return baseURL + "/tags"
	}
	return baseURL + modelListPath
}

func (p *Ollama) discoveryContext() (context.Context, context.CancelFunc) {
	timeout := defaultDiscoveryTimeout
	if p.config.Timeout > 0 && p.config.Timeout < timeout {
		timeout = p.config.Timeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

func illustrativeModels() []core.ModelInfo {
	return []core.ModelInfo{
		{ID: "llama3.2", DisplayName: "Llama 3.2", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "llama3.2:70b", DisplayName: "Llama 3.2 70B", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "mistral", DisplayName: "Mistral 7B", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "mixtral", DisplayName: "Mixtral 8x7B", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "qwen3", DisplayName: "Qwen 3", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureReasoning}},
		{ID: "gemma3", DisplayName: "Gemma 3", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming}},
		{ID: "deepseek-coder", DisplayName: "DeepSeek Coder", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming}},
		{ID: "codellama", DisplayName: "Code Llama", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming}},
	}
}
