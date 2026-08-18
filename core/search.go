package core

// SearchRecencyFilter limits web-search results by freshness for
// search-grounded providers (currently Perplexity).
type SearchRecencyFilter string

// Supported search recency filters.
const (
	SearchRecencyHour  SearchRecencyFilter = "hour"
	SearchRecencyDay   SearchRecencyFilter = "day"
	SearchRecencyWeek  SearchRecencyFilter = "week"
	SearchRecencyMonth SearchRecencyFilter = "month"
	SearchRecencyYear  SearchRecencyFilter = "year"
)

// SearchMode selects the corpus searched by search-grounded providers.
type SearchMode string

// Supported search modes.
const (
	// SearchModeWeb searches the general web (default).
	SearchModeWeb SearchMode = "web"
	// SearchModeAcademic searches academic sources (Perplexity).
	SearchModeAcademic SearchMode = "academic"
	// SearchModeSEC searches SEC filings (Perplexity).
	SearchModeSEC SearchMode = "sec"
)

// SearchOptions configures web-search grounding for providers that support
// FeatureWebSearch (currently Perplexity). Set them via
// ChatBuilder.SearchOptions; requests carrying search options against a
// provider without the capability are rejected with ErrSearchUnsupported
// before any HTTP call is made.
type SearchOptions struct {
	// SearchDomainFilter restricts results to the listed domains. A domain
	// prefixed with "-" is excluded instead of included (e.g. "-example.com").
	SearchDomainFilter []string `json:"search_domain_filter,omitempty"`

	// Recency limits results by freshness (hour, day, week, month, year).
	Recency SearchRecencyFilter `json:"search_recency_filter,omitempty"`

	// Mode selects the search corpus: web (default), academic, or sec.
	Mode SearchMode `json:"search_mode,omitempty"`
}
