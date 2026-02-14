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

type RunCommandTool struct {
	repoRoot        string
	enabled         bool
	allowedPrefixes []string
}

func NewRunCommandTool(repoRoot string, enabled bool, allowedPrefixes []string) *RunCommandTool {
	return &RunCommandTool{
		repoRoot:        repoRoot,
		enabled:         enabled,
		allowedPrefixes: allowedPrefixes,
	}
}

func (t *RunCommandTool) Name() string { return "run_command" }
func (t *RunCommandTool) Description() string {
	return "Run a command in the repo. Disabled by default; requires allowlist configuration."
}
func (t *RunCommandTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Executable name (e.g. \"go\", \"rg\", \"git\").",
			},
			"args": map[string]any{
				"type":        "array",
				"description": "Arguments array.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"timeout_sec": map[string]any{
				"type":        "integer",
				"description": "Timeout seconds (default 60).",
			},
		},
		"required": []string{"command"},
	}
}

type runCommandInput struct {
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	TimeoutSec int      `json:"timeout_sec"`
}

func (t *RunCommandTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	if !t.enabled {
		return core.ToolResult{}, errors.New("run_command is disabled (enable explicitly in config)")
	}

	var input runCommandInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	if input.Command == "" {
		return core.ToolResult{}, errors.New("missing command")
	}
	if input.TimeoutSec <= 0 {
		input.TimeoutSec = 60
	}

	if !t.isAllowed(input.Command, input.Args) {
		return core.ToolResult{}, errors.New("command is not in allowlist")
	}

	toolCtx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(toolCtx, input.Command, input.Args...)
	cmd.Dir = t.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return core.ToolResult{}, errors.New(string(out))
	}

	// Truncate output to keep prompts sane.
	s := string(out)
	if len(s) > 20000 {
		s = s[:20000] + "...(truncated)"
	}

	return core.ToolResult{Content: s}, nil
}

func (t *RunCommandTool) isAllowed(cmd string, args []string) bool {
	full := strings.TrimSpace(strings.Join(append([]string{cmd}, args...), " "))
	for _, p := range t.allowedPrefixes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(full, p) {
			return true
		}
	}
	return false
}
