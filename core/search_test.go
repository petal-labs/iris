package core

import (
	"context"
	"errors"
	"testing"
)

// searchProvider accepts any request and records the last one it saw.
type searchProvider struct {
	lastReq *ChatRequest
}

func (p *searchProvider) ID() string          { return "searchy" }
func (p *searchProvider) Models() []ModelInfo { return nil }
func (p *searchProvider) Supports(feature Feature) bool {
	return feature == FeatureChat || feature == FeatureWebSearch
}

func (p *searchProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	p.lastReq = req
	return &ChatResponse{ID: "resp-1", Model: req.Model, Output: "ok"}, nil
}

func (p *searchProvider) StreamChat(ctx context.Context, req *ChatRequest) (*ChatStream, error) {
	p.lastReq = req
	ch := make(chan ChatChunk, 1)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)
	go func() {
		ch <- ChatChunk{Delta: "ok"}
		close(ch)
		finalCh <- &ChatResponse{ID: "resp-1", Model: req.Model, Output: "ok"}
		close(finalCh)
		close(errCh)
	}()
	return &ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

func TestSearchOptionsReachProvider(t *testing.T) {
	p := &searchProvider{}
	c := NewClient(p)

	opts := &SearchOptions{
		SearchDomainFilter: []string{"go.dev", "-example.com"},
		Recency:            SearchRecencyMonth,
		Mode:               SearchModeAcademic,
	}
	_, err := c.Chat("sonar").User("hi").SearchOptions(opts).GetResponse(context.Background())
	if err != nil {
		t.Fatalf("GetResponse() error = %v", err)
	}

	if p.lastReq == nil || p.lastReq.SearchOptions == nil {
		t.Fatal("request reached provider without SearchOptions")
	}
	got := p.lastReq.SearchOptions
	if len(got.SearchDomainFilter) != 2 || got.SearchDomainFilter[0] != "go.dev" || got.SearchDomainFilter[1] != "-example.com" {
		t.Errorf("SearchDomainFilter = %v, want [go.dev -example.com]", got.SearchDomainFilter)
	}
	if got.Recency != SearchRecencyMonth {
		t.Errorf("Recency = %q, want %q", got.Recency, SearchRecencyMonth)
	}
	if got.Mode != SearchModeAcademic {
		t.Errorf("Mode = %q, want %q", got.Mode, SearchModeAcademic)
	}
}

func TestSearchOptionsUnsupportedIsHardError(t *testing.T) {
	// nonStructuredProvider.Supports returns false for everything, so it
	// lacks FeatureWebSearch; Chat must never be reached.
	c := NewClient(nonStructuredProvider{t: t})
	_, err := c.Chat("sonar").User("hi").
		SearchOptions(&SearchOptions{Recency: SearchRecencyDay}).
		GetResponse(context.Background())
	if !errors.Is(err, ErrSearchUnsupported) {
		t.Fatalf("err = %v, want ErrSearchUnsupported", err)
	}
}

func TestSearchOptionsNilAllowedOnAnyProvider(t *testing.T) {
	// No SearchOptions set: no gating, even on providers without the feature.
	c := NewClient(nonStructuredButChattyProvider{})
	_, err := c.Chat("m").User("hi").GetResponse(context.Background())
	if err != nil {
		t.Fatalf("GetResponse() error = %v", err)
	}
}

func TestSearchOptionsClearWithNil(t *testing.T) {
	b := NewClient(&searchProvider{}).Chat("sonar").
		SearchOptions(&SearchOptions{Recency: SearchRecencyWeek})
	if b.req.SearchOptions == nil {
		t.Fatal("SearchOptions not set")
	}
	b.SearchOptions(nil)
	if b.req.SearchOptions != nil {
		t.Errorf("SearchOptions = %v after nil, want nil", b.req.SearchOptions)
	}
}

func TestCloneWithSearchOptions(t *testing.T) {
	p := &searchProvider{}
	client := NewClient(p)

	original := client.Chat("sonar").
		User("Test").
		SearchOptions(&SearchOptions{
			SearchDomainFilter: []string{"go.dev"},
			Recency:            SearchRecencyMonth,
			Mode:               SearchModeWeb,
		})

	clone := original.Clone()

	if clone.req.SearchOptions == nil {
		t.Fatal("clone.SearchOptions is nil")
	}
	if clone.req.SearchOptions.Recency != SearchRecencyMonth {
		t.Errorf("clone.Recency = %q, want %q", clone.req.SearchOptions.Recency, SearchRecencyMonth)
	}
	if clone.req.SearchOptions.Mode != SearchModeWeb {
		t.Errorf("clone.Mode = %q, want %q", clone.req.SearchOptions.Mode, SearchModeWeb)
	}

	// Independence: mutating the clone's options must not affect the original.
	clone.req.SearchOptions.Recency = SearchRecencyYear
	clone.req.SearchOptions.SearchDomainFilter[0] = "mutated.dev"
	if original.req.SearchOptions.Recency != SearchRecencyMonth {
		t.Errorf("original.Recency = %q, want %q (clone mutation leaked)", original.req.SearchOptions.Recency, SearchRecencyMonth)
	}
	if original.req.SearchOptions.SearchDomainFilter[0] != "go.dev" {
		t.Errorf("original.SearchDomainFilter[0] = %q, want go.dev (clone mutation leaked)", original.req.SearchOptions.SearchDomainFilter[0])
	}
}

func TestChatResponseHasCitations(t *testing.T) {
	r := &ChatResponse{}
	if r.HasCitations() {
		t.Error("HasCitations() = true for nil citations")
	}
	r.Citations = []string{"https://example.com"}
	if !r.HasCitations() {
		t.Error("HasCitations() = false with one citation")
	}
}
