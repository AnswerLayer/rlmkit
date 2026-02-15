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

type BashTool struct {
	repoRoot        string
	enabled         bool
	allowedPrefixes []string
}

func NewBashTool(repoRoot string, enabled bool, allowedPrefixes []string) *BashTool {
	return &BashTool{
		repoRoot:        repoRoot,
		enabled:         enabled,
		allowedPrefixes: allowedPrefixes,
	}
}

func (t *BashTool) Name() string { return "bash" }
func (t *BashTool) Description() string {
	return "Run a shell command under the repo (bash -lc). Disabled by default; requires allowlisted script prefixes."
}
func (t *BashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"script": map[string]any{
				"type":        "string",
				"description": "Shell script to run (bash -lc).",
			},
			"timeout_sec": map[string]any{
				"type":        "integer",
				"description": "Timeout seconds (default 60).",
			},
		},
		"required": []string{"script"},
	}
}

type bashInput struct {
	Script     string `json:"script"`
	TimeoutSec int    `json:"timeout_sec"`
}

func (t *BashTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	if !t.enabled {
		return core.ToolResult{}, errors.New("bash is disabled (enable explicitly in config)")
	}

	var input bashInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	s := strings.TrimSpace(input.Script)
	if s == "" {
		return core.ToolResult{}, errors.New("missing script")
	}
	if input.TimeoutSec <= 0 {
		input.TimeoutSec = 60
	}
	if !t.isAllowed(s) {
		return core.ToolResult{}, errors.New("script is not in allowlist")
	}

	toolCtx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(toolCtx, "/bin/bash", "-lc", s)
	cmd.Dir = t.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return core.ToolResult{}, errors.New(string(out))
	}

	o := string(out)
	if len(o) > 20000 {
		o = o[:20000] + "...(truncated)"
	}
	return core.ToolResult{Content: o}, nil
}

func (t *BashTool) isAllowed(script string) bool {
	for _, p := range t.allowedPrefixes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(script, p) {
			return true
		}
	}
	return false
}
