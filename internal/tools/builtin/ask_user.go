package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type UserPrompter interface {
	Ask(ctx context.Context, question string, options []string, allowFreeform bool) (string, int, error)
}

type AskUserTool struct {
	p UserPrompter
}

func NewAskUserTool(p UserPrompter) *AskUserTool {
	return &AskUserTool{p: p}
}

func (t *AskUserTool) Name() string { return "ask_user" }
func (t *AskUserTool) Description() string {
	return "Ask the user a question and wait for a response. Use to resolve ambiguity before making changes."
}
func (t *AskUserTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "Question to ask the user.",
			},
			"options": map[string]any{
				"type":        "array",
				"description": "Optional list of choices. If provided, user can pick by number.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"allow_freeform": map[string]any{
				"type":        "boolean",
				"description": "Whether user may type a freeform answer (default true).",
				"default":     true,
			},
		},
		"required": []string{"question"},
	}
}

type askUserInput struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	AllowFreeform *bool    `json:"allow_freeform"`
}

func (t *AskUserTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	if t.p == nil {
		return core.ToolResult{}, errors.New("ask_user is not available (no prompter configured)")
	}

	var input askUserInput
	if err := json.Unmarshal(in, &input); err != nil {
		return core.ToolResult{}, err
	}
	q := strings.TrimSpace(input.Question)
	if q == "" {
		return core.ToolResult{}, errors.New("missing question")
	}
	allow := true
	if input.AllowFreeform != nil {
		allow = *input.AllowFreeform
	}

	ans, idx, err := t.p.Ask(ctx, q, input.Options, allow)
	if err != nil {
		return core.ToolResult{}, err
	}

	out := map[string]any{
		"answer":       ans,
		"choice_index": idx, // -1 for freeform
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return core.ToolResult{Content: string(b)}, nil
}

// BasicPrompter is used by the CLI.
// It prints to stderr and reads from stdin. It is intentionally simple.
type BasicPrompter struct {
	In  func() (string, error)
	Out func(string)
}

func (p BasicPrompter) Ask(ctx context.Context, question string, options []string, allowFreeform bool) (string, int, error) {
	if p.In == nil || p.Out == nil {
		return "", -1, errors.New("prompter not configured")
	}

	p.Out(fmt.Sprintf("\n[ask_user] %s\n", question))
	for i, opt := range options {
		p.Out(fmt.Sprintf("  %d) %s\n", i+1, opt))
	}

	for {
		select {
		case <-ctx.Done():
			return "", -1, ctx.Err()
		default:
		}

		if len(options) > 0 && allowFreeform {
			p.Out("Choose a number, or type an answer: ")
		} else if len(options) > 0 {
			p.Out("Choose a number: ")
		} else {
			p.Out("Answer: ")
		}

		line, err := p.In()
		if err != nil {
			return "", -1, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if len(options) > 0 {
			if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(options) {
				return options[n-1], n - 1, nil
			}
		}

		if allowFreeform {
			return line, -1, nil
		}
	}
}
