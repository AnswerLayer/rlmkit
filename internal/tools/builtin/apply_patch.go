package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type ApplyPatchTool struct {
	repoRoot string
}

func NewApplyPatchTool(repoRoot string) *ApplyPatchTool {
	return &ApplyPatchTool{repoRoot: repoRoot}
}

func (t *ApplyPatchTool) Name() string { return "apply_patch" }
func (t *ApplyPatchTool) Description() string {
	return "Apply a unified diff patch to the repo using `git apply`. Requires the target repo to be a git repository."
}
func (t *ApplyPatchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patch": map[string]any{
				"type":        "string",
				"description": "Unified diff patch.",
			},
		},
		"required": []string{"patch"},
	}
}

type applyPatchInput struct {
	Patch string `json:"patch"`
}

func (t *ApplyPatchTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	var input applyPatchInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	if input.Patch == "" {
		return core.ToolResult{}, errors.New("missing patch")
	}

	if _, err := os.Stat(filepath.Join(t.repoRoot, ".git")); err != nil {
		return core.ToolResult{}, errors.New("apply_patch requires a git repository (missing .git)")
	}

	tmpDir, err := os.MkdirTemp("", "rlmkit_patch_*")
	if err != nil {
		return core.ToolResult{}, err
	}
	defer os.RemoveAll(tmpDir)

	patchPath := filepath.Join(tmpDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(input.Patch), 0o600); err != nil {
		return core.ToolResult{}, err
	}

	toolCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Check first for a clearer error message.
	check := exec.CommandContext(toolCtx, "git", "apply", "--check", patchPath)
	check.Dir = t.repoRoot
	if out, err := check.CombinedOutput(); err != nil {
		return core.ToolResult{}, errors.New(string(out))
	}

	apply := exec.CommandContext(toolCtx, "git", "apply", patchPath)
	apply.Dir = t.repoRoot
	if out, err := apply.CombinedOutput(); err != nil {
		return core.ToolResult{}, errors.New(string(out))
	}

	return core.ToolResult{Content: "Patch applied."}, nil
}
