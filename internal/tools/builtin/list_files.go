package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type ListFilesTool struct {
	repoRoot string
}

func NewListFilesTool(repoRoot string) *ListFilesTool {
	return &ListFilesTool{repoRoot: repoRoot}
}

func (t *ListFilesTool) Name() string { return "list_files" }
func (t *ListFilesTool) Description() string {
	return "List files under the repo root. Useful for discovering project structure."
}
func (t *ListFilesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"glob": map[string]any{
				"type":        "string",
				"description": "Optional glob pattern relative to repo root (e.g. \"**/*.go\").",
			},
			"max": map[string]any{
				"type":        "integer",
				"description": "Maximum number of paths to return (default 2000).",
			},
		},
	}
}

type listFilesInput struct {
	Glob string `json:"glob"`
	Max  int    `json:"max"`
}

func (t *ListFilesTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	var input listFilesInput
	_ = json.Unmarshal(in, &input)
	if input.Max <= 0 {
		input.Max = 2000
	}

	var out []string
	err := filepath.WalkDir(t.repoRoot, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			return nil
		}

		rel, rerr := filepath.Rel(t.repoRoot, path)
		if rerr != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			// Avoid huge/irrelevant directories by default.
			if strings.HasPrefix(rel, ".git/") || rel == ".git" {
				return fs.SkipDir
			}
			return nil
		}

		if input.Glob != "" {
			ok, _ := doublestarMatch(input.Glob, rel)
			if !ok {
				return nil
			}
		}

		out = append(out, rel)
		if len(out) >= input.Max {
			return errStopWalk
		}
		return nil
	})
	if err != nil && err != errStopWalk && err != context.Canceled && err != context.DeadlineExceeded {
		return core.ToolResult{}, err
	}

	b, _ := json.MarshalIndent(map[string]any{
		"count": len(out),
		"paths": out,
	}, "", "  ")

	return core.ToolResult{Content: string(b)}, nil
}

var errStopWalk = fmt.Errorf("stop walk")

// MVP glob matching: supports "**" via a tiny helper (no external dep).
// We treat "**" as "match any segments".
func doublestarMatch(pattern, path string) (bool, error) {
	// Fast path: no ** => filepath.Match semantics (slash-normalized).
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, path)
	}

	// Very small implementation: split on ** and check ordered substrings.
	parts := strings.Split(pattern, "**")
	idx := 0
	for i, p := range parts {
		if p == "" {
			continue
		}
		j := strings.Index(path[idx:], p)
		if j < 0 {
			return false, nil
		}
		if i == 0 && !strings.HasPrefix(pattern, "**") && j != 0 {
			return false, nil
		}
		idx += j + len(p)
	}
	if !strings.HasSuffix(pattern, "**") && parts[len(parts)-1] != "" && idx != len(path) {
		// pattern tail must align to end if it didn't end with **
		return false, nil
	}
	return true, nil
}
