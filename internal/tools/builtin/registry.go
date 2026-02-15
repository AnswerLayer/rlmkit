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
	EnableBash           bool
	AllowedBashPrefix    []string
	EnableHTTPGet        bool
	AllowedURLPrefix     []string
	EnableDuckDB         bool
	EnableWebSearch      bool
	WebSearchProvider    string
	BraveAPIToken        string
	AllowSearchDomain    []string
	WebSearchMaxResults  int
	UserPrompter         UserPrompter
}

func RegisterAll(r *core.Registry, cfg BuiltinConfig) {
	r.Register(NewListFilesTool(cfg.RepoRoot))
	r.Register(NewReadFileTool(cfg.RepoRoot))
	r.Register(NewSearchRepoTool(cfg.RepoRoot))
	r.Register(NewApplyPatchTool(cfg.RepoRoot))
	r.Register(NewRunCommandTool(cfg.RepoRoot, cfg.EnableRunCommand, cfg.AllowedCommandPrefix))
	r.Register(NewBashTool(cfg.RepoRoot, cfg.EnableBash, cfg.AllowedBashPrefix))
	r.Register(NewHTTPGetTool(cfg.EnableHTTPGet, cfg.AllowedURLPrefix))
	r.Register(NewDuckDBQueryTool(cfg.RepoRoot, cfg.EnableDuckDB))
	r.Register(NewWebSearchTool(
		cfg.EnableWebSearch,
		cfg.WebSearchProvider,
		cfg.BraveAPIToken,
		cfg.AllowSearchDomain,
		cfg.WebSearchMaxResults,
	))
	r.Register(NewAskUserTool(cfg.UserPrompter))
	if cfg.SessionStore != nil && cfg.SessionID != "" {
		r.Register(NewSessionContextTool(cfg.SessionStore, cfg.SessionID))
	}
}
