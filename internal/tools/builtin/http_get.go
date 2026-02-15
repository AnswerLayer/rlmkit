package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type HTTPGetTool struct {
	enabled         bool
	allowedPrefixes []string
	client          *http.Client
}

func NewHTTPGetTool(enabled bool, allowedPrefixes []string) *HTTPGetTool {
	return &HTTPGetTool{
		enabled:         enabled,
		allowedPrefixes: allowedPrefixes,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (t *HTTPGetTool) Name() string { return "http_get" }
func (t *HTTPGetTool) Description() string {
	return "Fetch a URL via HTTP GET. Disabled by default; requires allowlisted URL prefixes."
}
func (t *HTTPGetTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "URL to fetch (http/https).",
			},
			"max_bytes": map[string]any{
				"type":        "integer",
				"description": "Maximum bytes to return (default 200000).",
			},
		},
		"required": []string{"url"},
	}
}

type httpGetInput struct {
	URL      string `json:"url"`
	MaxBytes int64  `json:"max_bytes"`
}

func (t *HTTPGetTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	if !t.enabled {
		return core.ToolResult{}, errors.New("http_get is disabled (enable explicitly in config)")
	}

	var input httpGetInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	u := strings.TrimSpace(input.URL)
	if u == "" {
		return core.ToolResult{}, errors.New("missing url")
	}
	if input.MaxBytes <= 0 {
		input.MaxBytes = 200_000
	}

	pu, err := url.Parse(u)
	if err != nil {
		return core.ToolResult{}, err
	}
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return core.ToolResult{}, errors.New("only http/https URLs are allowed")
	}
	if !t.isAllowed(u) {
		return core.ToolResult{}, errors.New("url is not in allowlist")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return core.ToolResult{}, err
	}
	req.Header.Set("User-Agent", "rlmkit/0 (http_get)")

	resp, err := t.client.Do(req)
	if err != nil {
		return core.ToolResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, input.MaxBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return core.ToolResult{}, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	return core.ToolResult{Content: string(body)}, nil
}

func (t *HTTPGetTool) isAllowed(u string) bool {
	for _, p := range t.allowedPrefixes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	return false
}
