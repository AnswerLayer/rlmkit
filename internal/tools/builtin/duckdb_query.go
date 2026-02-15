package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/answerlayer/rlmkit/internal/tools/core"
	"github.com/answerlayer/rlmkit/internal/util"
)

type DuckDBQueryTool struct {
	repoRoot string
	enabled  bool
}

func NewDuckDBQueryTool(repoRoot string, enabled bool) *DuckDBQueryTool {
	return &DuckDBQueryTool{repoRoot: repoRoot, enabled: enabled}
}

func (t *DuckDBQueryTool) Name() string { return "duckdb_query" }
func (t *DuckDBQueryTool) Description() string {
	return "Run a SQL query against a local DuckDB database file using the `duckdb` CLI. Disabled by default."
}
func (t *DuckDBQueryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"database_path": map[string]any{
				"type":        "string",
				"description": "Path to a .duckdb file relative to repo root.",
			},
			"sql": map[string]any{
				"type":        "string",
				"description": "SQL query to run.",
			},
			"timeout_sec": map[string]any{
				"type":        "integer",
				"description": "Timeout seconds (default 60).",
			},
			"max_bytes": map[string]any{
				"type":        "integer",
				"description": "Maximum bytes to return (default 200000).",
			},
		},
		"required": []string{"database_path", "sql"},
	}
}

type duckdbQueryInput struct {
	DatabasePath string `json:"database_path"`
	SQL          string `json:"sql"`
	TimeoutSec   int    `json:"timeout_sec"`
	MaxBytes     int64  `json:"max_bytes"`
}

func (t *DuckDBQueryTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	if !t.enabled {
		return core.ToolResult{}, errors.New("duckdb_query is disabled (enable explicitly in config)")
	}

	var input duckdbQueryInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	if strings.TrimSpace(input.DatabasePath) == "" {
		return core.ToolResult{}, errors.New("missing database_path")
	}
	if strings.TrimSpace(input.SQL) == "" {
		return core.ToolResult{}, errors.New("missing sql")
	}
	if input.TimeoutSec <= 0 {
		input.TimeoutSec = 60
	}
	if input.MaxBytes <= 0 {
		input.MaxBytes = 200_000
	}

	dbPath, err := util.ResolvePathWithinRoot(t.repoRoot, input.DatabasePath)
	if err != nil {
		return core.ToolResult{}, err
	}

	toolCtx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSec)*time.Second)
	defer cancel()

	// Use JSON output for easier downstream handling.
	cmd := exec.CommandContext(toolCtx, "duckdb", dbPath, "-json", "-c", input.SQL)
	cmd.Dir = t.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return core.ToolResult{}, errors.New(string(out))
	}

	s := string(out)
	if int64(len(s)) > input.MaxBytes {
		s = s[:input.MaxBytes] + "...(truncated)"
	}
	return core.ToolResult{Content: s}, nil
}
