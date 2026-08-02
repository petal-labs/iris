package voyageai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestCreateEmbeddingsEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	p := New("", WithBaseURL(srv.URL))
	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: ModelVoyage3Large,
		Input: []core.EmbeddingInput{{Text: "hi"}},
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("no HTTP request should be made with an empty key")
	}
}

func TestCreateContextualizedEmbeddingsEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	p := New("", WithBaseURL(srv.URL))
	_, err := p.CreateContextualizedEmbeddings(context.Background(), &core.ContextualizedEmbeddingRequest{
		Model:  ModelVoyageContext3,
		Inputs: [][]string{{"hi"}},
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("no HTTP request should be made with an empty key")
	}
}

func TestRerankEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	p := New("", WithBaseURL(srv.URL))
	_, err := p.Rerank(context.Background(), &core.RerankRequest{
		Model:     ModelRerank25,
		Query:     "hi",
		Documents: []string{"doc one", "doc two"},
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("no HTTP request should be made with an empty key")
	}
}
