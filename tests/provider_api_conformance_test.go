package tests

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/petal-labs/iris/providers"
	"github.com/petal-labs/iris/providers/anthropic"
	"github.com/petal-labs/iris/providers/azurefoundry"
	"github.com/petal-labs/iris/providers/gemini"
	"github.com/petal-labs/iris/providers/huggingface"
	"github.com/petal-labs/iris/providers/ollama"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/providers/perplexity"
	"github.com/petal-labs/iris/providers/voyageai"
	"github.com/petal-labs/iris/providers/xai"
	"github.com/petal-labs/iris/providers/zai"
)

type constructorShape string

const (
	standardConstructor constructorShape = "api-key"
	ollamaConstructor   constructorShape = "keyless-options"
	azureConstructor    constructorShape = "endpoint-and-api-key"
)

type providerAPISurface struct {
	id            string
	constructor   any
	newFromEnv    any
	withAPIKey    any
	withHeader    any
	defaultKeyEnv string
	shape         constructorShape
}

var providerAPISurfaces = []providerAPISurface{
	{id: "anthropic", constructor: anthropic.New, newFromEnv: anthropic.NewFromEnv, withAPIKey: anthropic.WithAPIKey, withHeader: anthropic.WithHeader, defaultKeyEnv: anthropic.DefaultAPIKeyEnvVar, shape: standardConstructor},
	{id: "azurefoundry", constructor: azurefoundry.New, newFromEnv: azurefoundry.NewFromEnv, withAPIKey: azurefoundry.WithAPIKey, withHeader: azurefoundry.WithHeader, defaultKeyEnv: azurefoundry.DefaultAPIKeyEnvVar, shape: azureConstructor},
	{id: "gemini", constructor: gemini.New, newFromEnv: gemini.NewFromEnv, withAPIKey: gemini.WithAPIKey, withHeader: gemini.WithHeader, defaultKeyEnv: gemini.DefaultAPIKeyEnvVar, shape: standardConstructor},
	{id: "huggingface", constructor: huggingface.New, newFromEnv: huggingface.NewFromEnv, withAPIKey: huggingface.WithAPIKey, withHeader: huggingface.WithHeader, defaultKeyEnv: huggingface.DefaultAPIKeyEnvVar, shape: standardConstructor},
	{id: "ollama", constructor: ollama.New, newFromEnv: ollama.NewFromEnv, withAPIKey: ollama.WithAPIKey, withHeader: ollama.WithHeader, defaultKeyEnv: ollama.DefaultAPIKeyEnvVar, shape: ollamaConstructor},
	{id: "openai", constructor: openai.New, newFromEnv: openai.NewFromEnv, withAPIKey: openai.WithAPIKey, withHeader: openai.WithHeader, defaultKeyEnv: openai.DefaultAPIKeyEnvVar, shape: standardConstructor},
	{id: "perplexity", constructor: perplexity.New, newFromEnv: perplexity.NewFromEnv, withAPIKey: perplexity.WithAPIKey, withHeader: perplexity.WithHeader, defaultKeyEnv: perplexity.DefaultAPIKeyEnvVar, shape: standardConstructor},
	{id: "voyageai", constructor: voyageai.New, newFromEnv: voyageai.NewFromEnv, withAPIKey: voyageai.WithAPIKey, withHeader: voyageai.WithHeader, defaultKeyEnv: voyageai.DefaultAPIKeyEnvVar, shape: standardConstructor},
	{id: "xai", constructor: xai.New, newFromEnv: xai.NewFromEnv, withAPIKey: xai.WithAPIKey, withHeader: xai.WithHeader, defaultKeyEnv: xai.DefaultAPIKeyEnvVar, shape: standardConstructor},
	{id: "zai", constructor: zai.New, newFromEnv: zai.NewFromEnv, withAPIKey: zai.WithAPIKey, withHeader: zai.WithHeader, defaultKeyEnv: zai.DefaultAPIKeyEnvVar, shape: standardConstructor},
}

func TestProviderAPISurfaceConformance(t *testing.T) {
	ids := make([]string, 0, len(providerAPISurfaces))
	for _, surface := range providerAPISurfaces {
		ids = append(ids, surface.id)
		t.Run(surface.id, func(t *testing.T) {
			assertNewFromEnvSignature(t, surface.newFromEnv)
			assertOptionSignature(t, "WithAPIKey", surface.withAPIKey, 1)
			assertOptionSignature(t, "WithHeader", surface.withHeader, 2)
			assertConstructorSignature(t, surface.constructor, surface.shape)
			if strings.TrimSpace(surface.defaultKeyEnv) == "" {
				t.Error("DefaultAPIKeyEnvVar must not be empty")
			}
		})
	}

	slices.Sort(ids)
	if registered := providers.List(); !slices.Equal(ids, registered) {
		t.Errorf("conformance providers = %v, registered providers = %v", ids, registered)
	}
}

func TestProviderConstructorExceptionsDocumented(t *testing.T) {
	content, err := os.ReadFile("../docs/ARCHITECTURE.md")
	if err != nil {
		t.Fatalf("read architecture docs: %v", err)
	}
	for _, signature := range []string{
		"ollama.New(opts ...Option)",
		"azurefoundry.New(endpoint, apiKey string, opts ...Option)",
	} {
		if !strings.Contains(string(content), signature) {
			t.Errorf("constructor exception %q is not documented", signature)
		}
	}
}

func assertNewFromEnvSignature(t *testing.T, fn any) {
	t.Helper()
	typ := reflect.TypeOf(fn)
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if !typ.IsVariadic() || typ.NumIn() != 1 || !isOptionSlice(typ.In(0)) {
		t.Errorf("NewFromEnv signature = %s, want func(...Option)", typ)
	}
	if typ.NumOut() != 2 || typ.Out(0).Kind() != reflect.Pointer || !typ.Out(1).Implements(errorType) {
		t.Errorf("NewFromEnv returns = %s, want (*Provider, error)", typ)
	}
}

func assertOptionSignature(t *testing.T, name string, fn any, stringInputs int) {
	t.Helper()
	typ := reflect.TypeOf(fn)
	if typ.IsVariadic() || typ.NumIn() != stringInputs || typ.NumOut() != 1 || typ.Out(0).Kind() != reflect.Func {
		t.Errorf("%s signature = %s, want %d string inputs and one Option output", name, typ, stringInputs)
		return
	}
	for i := 0; i < stringInputs; i++ {
		if typ.In(i).Kind() != reflect.String {
			t.Errorf("%s input %d = %s, want string", name, i, typ.In(i))
		}
	}
}

func assertConstructorSignature(t *testing.T, fn any, shape constructorShape) {
	t.Helper()
	typ := reflect.TypeOf(fn)
	wantInputs := map[constructorShape]int{standardConstructor: 2, ollamaConstructor: 1, azureConstructor: 3}[shape]
	if !typ.IsVariadic() || typ.NumIn() != wantInputs || typ.NumOut() != 1 || typ.Out(0).Kind() != reflect.Pointer {
		t.Errorf("constructor signature = %s, does not match documented %s shape", typ, shape)
		return
	}
	for i := 0; i < wantInputs-1; i++ {
		if typ.In(i).Kind() != reflect.String {
			t.Errorf("constructor input %d = %s, want string", i, typ.In(i))
		}
	}
	if !isOptionSlice(typ.In(wantInputs - 1)) {
		t.Errorf("constructor variadic input = %s, want Option", typ.In(wantInputs-1))
	}
}

func isOptionSlice(typ reflect.Type) bool {
	return typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.Func
}
