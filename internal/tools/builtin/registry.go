package builtin

import (
	"github.com/answerlayer/rlmkit/internal/session"
	"github.com/answerlayer/rlmkit/internal/tools/core"
)

type BuiltinConfig struct {
	RepoRoot             string
	SessionStore         *session.Store
	SessionID            string
	EnableRunCommand     bool
	AllowedCommandPrefix []string
}

func RegisterAll(r *core.Registry, cfg BuiltinConfig) {
	r.Register(NewListFilesTool(cfg.RepoRoot))
	r.Register(NewReadFileTool(cfg.RepoRoot))
	r.Register(NewSearchRepoTool(cfg.RepoRoot))
	r.Register(NewApplyPatchTool(cfg.RepoRoot))
	r.Register(NewRunCommandTool(cfg.RepoRoot, cfg.EnableRunCommand, cfg.AllowedCommandPrefix))
	if cfg.SessionStore != nil && cfg.SessionID != "" {
		r.Register(NewSessionContextTool(cfg.SessionStore, cfg.SessionID))
	}
}
