package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"`
}

type webSearchProvider interface {
	Search(ctx context.Context, q string, count int, params webSearchInput) ([]searchResult, error)
}

type WebSearchTool struct {
	enabled        bool
	providerName   string
	provider       webSearchProvider
	allowedDomains []string
	maxResults     int
}

func NewWebSearchTool(enabled bool, providerName string, braveToken string, allowedDomains []string, maxResults int) *WebSearchTool {
	pn := strings.TrimSpace(strings.ToLower(providerName))
	if pn == "" {
		pn = "brave"
	}
	if maxResults <= 0 {
		maxResults = 8
	}

	var p webSearchProvider
	switch pn {
	case "brave":
		p = &braveSearchProvider{
			apiKey: braveToken,
			http: &http.Client{
				Timeout: 20 * time.Second,
			},
		}
	default:
		// Keep nil; tool will return a clear error on use.
	}

	return &WebSearchTool{
		enabled:        enabled,
		providerName:   pn,
		provider:       p,
		allowedDomains: allowedDomains,
		maxResults:     maxResults,
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }
func (t *WebSearchTool) Description() string {
	return "Search the web and return normalized results. Disabled by default; provider and API key required."
}
func (t *WebSearchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query.",
			},
			"count": map[string]any{
				"type":        "integer",
				"description": "Number of results (default 5; capped by config).",
			},
			"freshness": map[string]any{
				"type":        "string",
				"description": "Optional freshness hint (e.g. 'day', 'week', 'month'). Provider-specific.",
			},
			"country": map[string]any{
				"type":        "string",
				"description": "Optional country code (e.g. 'US'). Provider-specific.",
			},
		},
		"required": []string{"query"},
	}
}

type webSearchInput struct {
	Query     string `json:"query"`
	Count     int    `json:"count"`
	Freshness string `json:"freshness"`
	Country   string `json:"country"`
}

func (t *WebSearchTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	if !t.enabled {
		return core.ToolResult{}, errors.New("web_search is disabled (enable explicitly in config)")
	}
	if t.provider == nil {
		return core.ToolResult{}, fmt.Errorf("web_search provider '%s' is not supported", t.providerName)
	}

	var input webSearchInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	q := strings.TrimSpace(input.Query)
	if q == "" {
		return core.ToolResult{}, errors.New("missing query")
	}
	if input.Count <= 0 {
		input.Count = 5
	}
	if input.Count > t.maxResults {
		input.Count = t.maxResults
	}

	results, err := t.provider.Search(ctx, q, input.Count, input)
	if err != nil {
		return core.ToolResult{}, err
	}

	// Optional domain allowlist post-filter.
	if len(t.allowedDomains) > 0 {
		filtered := make([]searchResult, 0, len(results))
		for _, r := range results {
			if isDomainAllowed(r.URL, t.allowedDomains) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	out := map[string]any{
		"provider": t.providerName,
		"count":    len(results),
		"results":  results,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return core.ToolResult{Content: string(b)}, nil
}

func isDomainAllowed(rawURL string, allowed []string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, d := range allowed {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

type braveSearchProvider struct {
	apiKey string
	http   *http.Client
}

func (b *braveSearchProvider) Search(ctx context.Context, q string, count int, params webSearchInput) ([]searchResult, error) {
	key := strings.TrimSpace(b.apiKey)
	if key == "" {
		return nil, errors.New("missing Brave API key (set config brave_api_key or BRAVE_SEARCH_API_KEY)")
	}

	u, _ := url.Parse("https://api.search.brave.com/res/v1/web/search")
	qs := u.Query()
	qs.Set("q", q)
	qs.Set("count", strconv.Itoa(count))
	if params.Freshness != "" {
		qs.Set("freshness", params.Freshness)
	}
	if params.Country != "" {
		qs.Set("country", strings.ToUpper(params.Country))
	}
	u.RawQuery = qs.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", key)
	req.Header.Set("User-Agent", "rlmkit/0 (web_search)")

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("brave search http %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

	results := make([]searchResult, 0, len(out.Web.Results))
	for _, r := range out.Web.Results {
		results = append(results, searchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
			Source:  "brave",
		})
	}
	return results, nil
}
