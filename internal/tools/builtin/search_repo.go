package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type SearchRepoTool struct {
	repoRoot string
}

func NewSearchRepoTool(repoRoot string) *SearchRepoTool {
	return &SearchRepoTool{repoRoot: repoRoot}
}

func (t *SearchRepoTool) Name() string { return "search_repo" }
func (t *SearchRepoTool) Description() string {
	return "Search the repo using ripgrep (rg). Returns matching lines with paths and line numbers."
}
func (t *SearchRepoTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (ripgrep regex).",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "Optional glob filter (passed to rg as --glob).",
			},
			"max_lines": map[string]any{
				"type":        "integer",
				"description": "Maximum output lines to return (default 200).",
			},
		},
		"required": []string{"query"},
	}
}

type searchRepoInput struct {
	Query    string `json:"query"`
	Glob     string `json:"glob"`
	MaxLines int    `json:"max_lines"`
}

func (t *SearchRepoTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	var input searchRepoInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	if input.Query == "" {
		return core.ToolResult{}, errors.New("missing query")
	}
	if input.MaxLines <= 0 {
		input.MaxLines = 200
	}

	toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{"--line-number", "--no-heading", "--smart-case", input.Query, "."}
	if input.Glob != "" {
		args = append([]string{"--glob", input.Glob}, args...)
	}

	cmd := exec.CommandContext(toolCtx, "rg", args...)
	cmd.Dir = t.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		// If rg exited non-zero due to no matches, still return empty output.
		// rg uses exit code 1 for no matches.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return core.ToolResult{Content: ""}, nil
		}
		return core.ToolResult{}, err
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) > input.MaxLines {
		lines = lines[:input.MaxLines]
	}
	return core.ToolResult{Content: strings.Join(lines, "\n")}, nil
}
