package azurefoundry

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestAzureFoundryImplementsProvider(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")
	var _ core.Provider = p
}

func TestAzureFoundryImplementsEmbeddingProvider(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")
	var _ core.EmbeddingProvider = p
}

func TestID(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if p.ID() != "azurefoundry" {
		t.Errorf("ID() = %q, want %q", p.ID(), "azurefoundry")
	}
}

func TestModels(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")
	models := p.Models()

	if len(models) < 10 {
		t.Errorf("Models() returned %d models, want at least 10", len(models))
	}

	// Check for required models
	modelIDs := make(map[core.ModelID]bool)
	for _, m := range models {
		modelIDs[m.ID] = true
	}

	if !modelIDs["gpt-4o"] {
		t.Error("Models() missing gpt-4o")
	}

	if !modelIDs["gpt-4o-mini"] {
		t.Error("Models() missing gpt-4o-mini")
	}

	if !modelIDs["text-embedding-3-large"] {
		t.Error("Models() missing text-embedding-3-large")
	}
}

func TestModelsReturnsCopy(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")
	models1 := p.Models()
	models2 := p.Models()

	if len(models1) > 0 {
		models1[0].DisplayName = "modified"
	}

	if models2[0].DisplayName == "modified" {
		t.Error("Models() did not return a copy")
	}
}

func TestModelsHaveCapabilities(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")
	models := p.Models()

	for _, m := range models {
		if len(m.Capabilities) == 0 {
			t.Errorf("Model %s has no capabilities", m.ID)
		}
	}
}

func TestSupportsChat(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if !p.Supports(core.FeatureChat) {
		t.Error("Supports(FeatureChat) = false, want true")
	}
}

func TestSupportsChatStreaming(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if !p.Supports(core.FeatureChatStreaming) {
		t.Error("Supports(FeatureChatStreaming) = false, want true")
	}
}

func TestProviderSupportsToolCalling(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if !p.Supports(core.FeatureToolCalling) {
		t.Error("Supports(FeatureToolCalling) = false, want true")
	}
}

func TestSupportsEmbeddings(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if !p.Supports(core.FeatureEmbeddings) {
		t.Error("Supports(FeatureEmbeddings) = false, want true")
	}
}

func TestProviderSupportsStructuredOutput(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if !p.Supports(core.FeatureStructuredOutput) {
		t.Error("Supports(FeatureStructuredOutput) = false, want true")
	}
}

func TestSupportsReasoning(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if !p.Supports(core.FeatureReasoning) {
		t.Error("Supports(FeatureReasoning) = false, want true")
	}
}

func TestSupportsUnknownFeature(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key")

	if p.Supports(core.Feature("unknown_feature")) {
		t.Error("Supports(unknown) = true, want false")
	}
}

func TestNewWithEndpointAndKey(t *testing.T) {
	p := New("https://my-resource.openai.azure.com", "my-api-key")

	if p.config.Endpoint != "https://my-resource.openai.azure.com" {
		t.Errorf("Endpoint = %q, want %q", p.config.Endpoint, "https://my-resource.openai.azure.com")
	}

	if p.config.APIKey.Expose() != "my-api-key" {
		t.Errorf("APIKey = %q, want %q", p.config.APIKey.Expose(), "my-api-key")
	}
}

func TestNewWithCredential(t *testing.T) {
	cred := &mockCredential{token: "mock-token"}
	p := NewWithCredential("https://my-resource.openai.azure.com", cred)

	if p.config.TokenCredential == nil {
		t.Error("TokenCredential is nil")
	}
}

func TestNewWithOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	p := New("https://test.openai.azure.com", "test-key",
		WithHTTPClient(customClient),
		WithAPIVersion("2024-06-01"),
		WithDeploymentID("my-deployment"),
		WithTimeout(30*time.Second),
		WithHeader("X-Custom-Header", "custom-value"),
	)

	if p.config.HTTPClient != customClient {
		t.Error("HTTPClient not set correctly")
	}

	if p.config.APIVersion != "2024-06-01" {
		t.Errorf("APIVersion = %q, want %q", p.config.APIVersion, "2024-06-01")
	}

	if p.config.DeploymentID != "my-deployment" {
		t.Errorf("DeploymentID = %q, want %q", p.config.DeploymentID, "my-deployment")
	}

	if p.config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", p.config.Timeout, 30*time.Second)
	}

	if p.config.Headers.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Custom header not set correctly")
	}
}

func TestWithUseOpenAIEndpoint(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key", WithOpenAIEndpoint())

	if !p.config.UseOpenAIEndpoint {
		t.Error("UseOpenAIEndpoint = false, want true")
	}
}

func TestBuildChatURLModelInference(t *testing.T) {
	p := New("https://my-resource.models.ai.azure.com", "test-key")

	url, err := p.buildChatURL("gpt-4o")
	if err != nil {
		t.Fatalf("buildChatURL() error = %v", err)
	}

	expected := "https://my-resource.models.ai.azure.com/models/chat/completions?api-version=2024-05-01-preview"
	if url != expected {
		t.Errorf("buildChatURL() = %q, want %q", url, expected)
	}
}

func TestBuildChatURLAzureOpenAI(t *testing.T) {
	p := New("https://my-resource.openai.azure.com", "test-key",
		WithOpenAIEndpoint(),
		WithDeploymentID("gpt-4o-deployment"),
	)

	url, err := p.buildChatURL("gpt-4o")
	if err != nil {
		t.Fatalf("buildChatURL() error = %v", err)
	}

	expected := "https://my-resource.openai.azure.com/openai/deployments/gpt-4o-deployment/chat/completions?api-version=2024-10-21"
	if url != expected {
		t.Errorf("buildChatURL() = %q, want %q", url, expected)
	}
}

func TestBuildChatURLAzureOpenAIUsesModelAsDeployment(t *testing.T) {
	p := New("https://my-resource.openai.azure.com", "test-key",
		WithOpenAIEndpoint(),
	)

	url, err := p.buildChatURL("my-model")
	if err != nil {
		t.Fatalf("buildChatURL() error = %v", err)
	}

	expected := "https://my-resource.openai.azure.com/openai/deployments/my-model/chat/completions?api-version=2024-10-21"
	if url != expected {
		t.Errorf("buildChatURL() = %q, want %q", url, expected)
	}
}

func TestBuildChatURLAzureOpenAINoDeployment(t *testing.T) {
	p := New("https://my-resource.openai.azure.com", "test-key",
		WithOpenAIEndpoint(),
	)

	// Empty model and no deployment should error
	_, err := p.buildChatURL("")
	if err != ErrDeploymentRequired {
		t.Errorf("buildChatURL() error = %v, want ErrDeploymentRequired", err)
	}
}

func TestBuildEmbeddingsURLModelInference(t *testing.T) {
	p := New("https://my-resource.models.ai.azure.com", "test-key")

	url, err := p.buildEmbeddingsURL("text-embedding-3-large")
	if err != nil {
		t.Fatalf("buildEmbeddingsURL() error = %v", err)
	}

	expected := "https://my-resource.models.ai.azure.com/models/embeddings?api-version=2024-05-01-preview"
	if url != expected {
		t.Errorf("buildEmbeddingsURL() = %q, want %q", url, expected)
	}
}

func TestBuildEmbeddingsURLAzureOpenAI(t *testing.T) {
	p := New("https://my-resource.openai.azure.com", "test-key",
		WithOpenAIEndpoint(),
		WithDeploymentID("embedding-deployment"),
	)

	url, err := p.buildEmbeddingsURL("text-embedding-3-large")
	if err != nil {
		t.Fatalf("buildEmbeddingsURL() error = %v", err)
	}

	expected := "https://my-resource.openai.azure.com/openai/deployments/embedding-deployment/embeddings?api-version=2024-10-21"
	if url != expected {
		t.Errorf("buildEmbeddingsURL() = %q, want %q", url, expected)
	}
}

func TestBuildHeadersWithAPIKey(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-api-key-123")
	headers, err := p.buildHeaders(context.Background())

	if err != nil {
		t.Fatalf("buildHeaders() error = %v", err)
	}

	apiKey := headers.Get("api-key")
	if apiKey != "test-api-key-123" {
		t.Errorf("api-key = %q, want %q", apiKey, "test-api-key-123")
	}

	contentType := headers.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestBuildHeadersWithTokenCredential(t *testing.T) {
	cred := &mockCredential{token: "mock-bearer-token"}
	p := NewWithCredential("https://test.openai.azure.com", cred)

	headers, err := p.buildHeaders(context.Background())
	if err != nil {
		t.Fatalf("buildHeaders() error = %v", err)
	}

	auth := headers.Get("Authorization")
	if auth != "Bearer mock-bearer-token" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer mock-bearer-token")
	}
}

func TestBuildHeadersWithCustomHeaders(t *testing.T) {
	p := New("https://test.openai.azure.com", "test-key",
		WithHeader("X-Custom-One", "value1"),
		WithHeader("X-Custom-Two", "value2"),
	)
	headers, err := p.buildHeaders(context.Background())
	if err != nil {
		t.Fatalf("buildHeaders() error = %v", err)
	}

	if headers.Get("X-Custom-One") != "value1" {
		t.Errorf("X-Custom-One = %q, want %q", headers.Get("X-Custom-One"), "value1")
	}

	if headers.Get("X-Custom-Two") != "value2" {
		t.Errorf("X-Custom-Two = %q, want %q", headers.Get("X-Custom-Two"), "value2")
	}
}

// mockCredential implements TokenCredential for testing.
type mockCredential struct {
	token string
	err   error
}

func (m *mockCredential) GetToken(ctx context.Context, options TokenRequestOptions) (AccessToken, error) {
	if m.err != nil {
		return AccessToken{}, m.err
	}
	return AccessToken{
		Token:     m.token,
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}
