package builtin

import (
	"context"
	"encoding/json"

	"github.com/answerlayer/rlmkit/internal/session"
	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type SessionContextTool struct {
	store     *session.Store
	sessionID string
}

func NewSessionContextTool(store *session.Store, sessionID string) *SessionContextTool {
	return &SessionContextTool{store: store, sessionID: sessionID}
}

func (t *SessionContextTool) Name() string { return "get_session_context" }
func (t *SessionContextTool) Description() string {
	return "Query prior turns in the current session (RLM pattern). Use when resolving pronouns or referring to earlier results."
}
func (t *SessionContextTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"last_n": map[string]any{
				"type":        "integer",
				"description": "Return only the last N turns.",
			},
			"include_tool_calls": map[string]any{
				"type":        "boolean",
				"description": "Whether to include tool outputs (default false).",
			},
		},
	}
}

func (t *SessionContextTool) Execute(ctx context.Context, in json.RawMessage) (core.ToolResult, error) {
	var req session.SessionContextRequest
	_ = json.Unmarshal(in, &req)

	resp, err := t.store.GetSessionContext(ctx, t.sessionID, req)
	if err != nil {
		return core.ToolResult{}, err
	}

	b, _ := json.MarshalIndent(resp, "", "  ")
	return core.ToolResult{Content: string(b)}, nil
}
