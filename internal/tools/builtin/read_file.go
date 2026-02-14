package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/answerlayer/rlmkit/internal/tools/core"
	"github.com/answerlayer/rlmkit/internal/util"
)

type ReadFileTool struct {
	repoRoot string
}

func NewReadFileTool(repoRoot string) *ReadFileTool {
	return &ReadFileTool{repoRoot: repoRoot}
}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read a file under the repo root." }
func (t *ReadFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to repo root.",
			},
			"max_bytes": map[string]any{
				"type":        "integer",
				"description": "Maximum bytes to read (default 200000).",
			},
		},
		"required": []string{"path"},
	}
}

type readFileInput struct {
	Path     string `json:"path"`
	MaxBytes int64  `json:"max_bytes"`
}

func (t *ReadFileTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	var input readFileInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	if input.Path == "" {
		return core.ToolResult{}, errors.New("missing path")
	}
	if input.MaxBytes <= 0 {
		input.MaxBytes = 200_000
	}

	p, err := util.ResolvePathWithinRoot(t.repoRoot, input.Path)
	if err != nil {
		return core.ToolResult{}, err
	}

	f, err := os.Open(p)
	if err != nil {
		return core.ToolResult{}, err
	}
	defer f.Close()

	select {
	case <-ctx.Done():
		return core.ToolResult{}, ctx.Err()
	default:
	}

	b, err := io.ReadAll(io.LimitReader(f, input.MaxBytes))
	if err != nil {
		return core.ToolResult{}, err
	}

	return core.ToolResult{Content: string(b)}, nil
}
