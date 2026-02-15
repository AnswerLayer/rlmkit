package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/answerlayer/rlmkit/internal/agent"
	"github.com/answerlayer/rlmkit/internal/coding"
	"github.com/answerlayer/rlmkit/internal/llm/openai"
	"github.com/answerlayer/rlmkit/internal/session"
	"github.com/answerlayer/rlmkit/internal/tools/builtin"
	"github.com/answerlayer/rlmkit/internal/tools/core"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type FileConfig struct {
	BaseURL            string   `json:"base_url"`
	APIKey             string   `json:"api_key"`
	Model              string   `json:"model"`
	RepoRoot           string   `json:"repo_root"`
	SessionDir         string   `json:"session_dir"`
	RecentTurns        int      `json:"recent_turns"`
	MaxIterations      int      `json:"max_iterations"`
	MaxToolConcurrency int64    `json:"max_tool_concurrency"`
	ToolTimeoutSec     int      `json:"tool_timeout_sec"`
	Stream             bool     `json:"stream"`
	EnableRunCommand   bool     `json:"enable_run_command"`
	AllowCommandPrefix []string `json:"allow_command_prefix"`
	EnableBash         bool     `json:"enable_bash"`
	AllowBashPrefix    []string `json:"allow_bash_prefix"`
	EnableHTTPGet      bool     `json:"enable_http_get"`
	AllowURLPrefix     []string `json:"allow_url_prefix"`
	EnableDuckDB       bool     `json:"enable_duckdb"`
	EnableWebSearch    bool     `json:"enable_web_search"`
	WebSearchProvider  string   `json:"web_search_provider"`
	BraveAPIKey        string   `json:"brave_api_key"`
	AllowSearchDomain  []string `json:"allow_search_domain"`
	WebSearchMaxResult int      `json:"web_search_max_results"`
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h") {
		usage()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("rlmkit %s (%s) %s\n", version, commit, date)
		return
	}

	// Subcommand-ish parsing: `rlmkit chat ...` or `rlmkit -p ...`
	if len(os.Args) > 1 && os.Args[1] == "chat" {
		runChat(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "code" {
		runCode(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "tools" {
		runTools(os.Args[2:])
		return
	}

	runOneShot(os.Args[1:])
}

func usage() {
	fmt.Println("rlmkit - minimal RLM coding agent")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  rlmkit chat [flags]          Interactive chat")
	fmt.Println("  rlmkit code [flags]          Interactive coding mode (more opinionated prompt)")
	fmt.Println("  rlmkit -p \"...\" [flags]      One-shot prompt")
	fmt.Println("  rlmkit tools [flags]         Print available tools as JSON")
	fmt.Println("  rlmkit version               Print version info")
	fmt.Println("")
	fmt.Println("Common flags:")
	fmt.Println("  --config <path>              Config file (default ./rlmkit.json if present)")
	fmt.Println("  --base-url <url>             OpenAI-compatible base URL (default http://127.0.0.1:8080/v1)")
	fmt.Println("  --model <name|auto>          Model name (optional; 'auto' uses /v1/models)")
	fmt.Println("  --repo-root <path>           Repo root (default current directory)")
	fmt.Println("  --session-dir <path>         Session storage dir (default ./sessions)")
	fmt.Println("  --session-id <id>            Resume or pin a session ID")
	fmt.Println("  --recent-turns <n>           Number of recent turns to include (default 2)")
	fmt.Println("  --stream                     Stream model output (default true)")
	fmt.Println("")
	fmt.Println("Safety flags:")
	fmt.Println("  --enable-run-command         Enable run_command tool (disabled by default)")
	fmt.Println("  --allow-cmd-prefix <s>       Allowlisted command prefix (repeatable)")
	fmt.Println("  --enable-bash                Enable bash tool (disabled by default)")
	fmt.Println("  --allow-bash-prefix <s>      Allowlisted bash script prefix (repeatable)")
	fmt.Println("  --enable-http-get            Enable http_get tool (disabled by default)")
	fmt.Println("  --allow-url-prefix <s>       Allowlisted URL prefix (repeatable)")
	fmt.Println("  --enable-duckdb              Enable duckdb_query tool (disabled by default)")
	fmt.Println("  --enable-web-search          Enable web_search tool (disabled by default)")
	fmt.Println("  --web-search-provider <p>    Search provider (default brave)")
	fmt.Println("  --brave-api-key <key>        Brave API key (or set BRAVE_SEARCH_API_KEY)")
	fmt.Println("  --allow-search-domain <d>    Allowlisted search domain (repeatable)")
	fmt.Println("  --web-search-max-results <n> Max search results returned by tool")
}

func loadFileConfig(path string) (FileConfig, bool, error) {
	if path == "" {
		path = "rlmkit.json"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, false, nil
		}
		return FileConfig{}, false, err
	}
	var cfg FileConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return FileConfig{}, false, err
	}
	return cfg, true, nil
}

func runChat(args []string) {
	fs := flag.NewFlagSet("chat", flag.ExitOnError)
	var (
		configPath  = fs.String("config", "", "config file path (default ./rlmkit.json if present)")
		baseURL     = fs.String("base-url", "", "OpenAI-compatible base URL")
		apiKey      = fs.String("api-key", "", "API key (usually empty for local servers)")
		model       = fs.String("model", "", "model name")
		repoRoot    = fs.String("repo-root", "", "repo root")
		sessionDir  = fs.String("session-dir", "", "session dir")
		sessionID   = fs.String("session-id", "", "session id")
		recentTurns = fs.Int("recent-turns", 0, "recent turns to include (0 uses config/default)")
		stream      = fs.Bool("stream", true, "stream model output")
		enableRun   = fs.Bool("enable-run-command", false, "enable run_command tool")
		allowPrefix multiStringFlag
		enableBash  = fs.Bool("enable-bash", false, "enable bash tool")
		allowBash   multiStringFlag
		enableHTTP  = fs.Bool("enable-http-get", false, "enable http_get tool")
		allowURL    multiStringFlag
		enableDuck  = fs.Bool("enable-duckdb", false, "enable duckdb_query tool")
		enableWeb   = fs.Bool("enable-web-search", false, "enable web_search tool")
		webProvider = fs.String("web-search-provider", "", "search provider (default brave)")
		braveKey    = fs.String("brave-api-key", "", "Brave API key")
		allowDomain multiStringFlag
		webMax      = fs.Int("web-search-max-results", 0, "max search results")
	)
	fs.Var(&allowPrefix, "allow-cmd-prefix", "allowlisted command prefix (repeatable)")
	fs.Var(&allowBash, "allow-bash-prefix", "allowlisted bash script prefix (repeatable)")
	fs.Var(&allowURL, "allow-url-prefix", "allowlisted URL prefix (repeatable)")
	fs.Var(&allowDomain, "allow-search-domain", "allowlisted search domain (repeatable)")
	_ = fs.Parse(args)

	cfg := resolveConfig(*configPath, *baseURL, *apiKey, *model, *repoRoot, *sessionDir, *recentTurns, *enableRun, allowPrefix)
	cfg.Stream = *stream
	if *enableBash {
		cfg.EnableBash = true
	}
	if len(allowBash) > 0 {
		cfg.AllowBashPrefix = allowBash
	}
	if *enableHTTP {
		cfg.EnableHTTPGet = true
	}
	if len(allowURL) > 0 {
		cfg.AllowURLPrefix = allowURL
	}
	if *enableDuck {
		cfg.EnableDuckDB = true
	}
	if *enableWeb {
		cfg.EnableWebSearch = true
	}
	if *webProvider != "" {
		cfg.WebSearchProvider = *webProvider
	}
	if *braveKey != "" {
		cfg.BraveAPIKey = *braveKey
	}
	if len(allowDomain) > 0 {
		cfg.AllowSearchDomain = allowDomain
	}
	if *webMax > 0 {
		cfg.WebSearchMaxResult = *webMax
	}
	sid := *sessionID
	if sid == "" {
		sid = newSessionID()
	}

	eng, store, err := buildEngine(cfg, sid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	_ = store.EnsureDir()

	fmt.Printf("session: %s\n", sid)
	fmt.Println("type 'exit' to quit")

	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		ctx := context.Background()
		if cfg.Stream {
			evCh, errCh := eng.RunStream(ctx, sid, line)
			for ev := range evCh {
				switch ev.Type {
				case agent.EventAssistantDelta:
					fmt.Print(ev.Text)
				case agent.EventToolStart:
					fmt.Fprintf(os.Stderr, "\n[tool] %s\n", ev.ToolName)
				case agent.EventToolEnd:
					fmt.Fprintf(os.Stderr, "[tool done] %s\n", ev.ToolName)
				case agent.EventFinal:
				}
			}
			if err := <-errCh; err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			fmt.Println("")
		} else {
			res, err := eng.Run(ctx, sid, line)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				continue
			}
			fmt.Println(res.Reply)
		}
	}
}

func runCode(args []string) {
	// Same as chat, but uses a coding-oriented system prompt.
	fs := flag.NewFlagSet("code", flag.ExitOnError)
	var (
		configPath  = fs.String("config", "", "config file path (default ./rlmkit.json if present)")
		baseURL     = fs.String("base-url", "", "OpenAI-compatible base URL")
		apiKey      = fs.String("api-key", "", "API key (usually empty for local servers)")
		model       = fs.String("model", "", "model name")
		repoRoot    = fs.String("repo-root", "", "repo root")
		sessionDir  = fs.String("session-dir", "", "session dir")
		sessionID   = fs.String("session-id", "", "session id")
		recentTurns = fs.Int("recent-turns", 0, "recent turns to include (0 uses config/default)")
		stream      = fs.Bool("stream", true, "stream model output")
		enableRun   = fs.Bool("enable-run-command", false, "enable run_command tool")
		allowPrefix multiStringFlag
		enableBash  = fs.Bool("enable-bash", false, "enable bash tool")
		allowBash   multiStringFlag
		enableHTTP  = fs.Bool("enable-http-get", false, "enable http_get tool")
		allowURL    multiStringFlag
		enableDuck  = fs.Bool("enable-duckdb", false, "enable duckdb_query tool")
		enableWeb   = fs.Bool("enable-web-search", false, "enable web_search tool")
		webProvider = fs.String("web-search-provider", "", "search provider (default brave)")
		braveKey    = fs.String("brave-api-key", "", "Brave API key")
		allowDomain multiStringFlag
		webMax      = fs.Int("web-search-max-results", 0, "max search results")
	)
	fs.Var(&allowPrefix, "allow-cmd-prefix", "allowlisted command prefix (repeatable)")
	fs.Var(&allowBash, "allow-bash-prefix", "allowlisted bash script prefix (repeatable)")
	fs.Var(&allowURL, "allow-url-prefix", "allowlisted URL prefix (repeatable)")
	fs.Var(&allowDomain, "allow-search-domain", "allowlisted search domain (repeatable)")
	_ = fs.Parse(args)

	cfg := resolveConfig(*configPath, *baseURL, *apiKey, *model, *repoRoot, *sessionDir, *recentTurns, *enableRun, allowPrefix)
	cfg.Stream = *stream
	if *enableBash {
		cfg.EnableBash = true
	}
	if len(allowBash) > 0 {
		cfg.AllowBashPrefix = allowBash
	}
	if *enableHTTP {
		cfg.EnableHTTPGet = true
	}
	if len(allowURL) > 0 {
		cfg.AllowURLPrefix = allowURL
	}
	if *enableDuck {
		cfg.EnableDuckDB = true
	}
	if *enableWeb {
		cfg.EnableWebSearch = true
	}
	if *webProvider != "" {
		cfg.WebSearchProvider = *webProvider
	}
	if *braveKey != "" {
		cfg.BraveAPIKey = *braveKey
	}
	if len(allowDomain) > 0 {
		cfg.AllowSearchDomain = allowDomain
	}
	if *webMax > 0 {
		cfg.WebSearchMaxResult = *webMax
	}
	sid := *sessionID
	if sid == "" {
		sid = newSessionID()
	}

	eng, store, err := buildEngineWithPrompt(cfg, sid, "coding")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	_ = store.EnsureDir()

	fmt.Printf("session: %s\n", sid)
	fmt.Println("type 'exit' to quit")

	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		ctx := context.Background()
		if cfg.Stream {
			evCh, errCh := eng.RunStream(ctx, sid, line)
			for ev := range evCh {
				switch ev.Type {
				case agent.EventAssistantDelta:
					fmt.Print(ev.Text)
				case agent.EventToolStart:
					fmt.Fprintf(os.Stderr, "\n[tool] %s\n", ev.ToolName)
				case agent.EventToolEnd:
					fmt.Fprintf(os.Stderr, "[tool done] %s\n", ev.ToolName)
				case agent.EventFinal:
				}
			}
			if err := <-errCh; err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			fmt.Println("")
		} else {
			res, err := eng.Run(ctx, sid, line)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				continue
			}
			fmt.Println(res.Reply)
		}
	}
}

func runOneShot(args []string) {
	fs := flag.NewFlagSet("rlmkit", flag.ExitOnError)
	var (
		configPath  = fs.String("config", "", "config file path (default ./rlmkit.json if present)")
		prompt      = fs.String("p", "", "prompt")
		baseURL     = fs.String("base-url", "", "OpenAI-compatible base URL")
		apiKey      = fs.String("api-key", "", "API key (usually empty for local servers)")
		model       = fs.String("model", "", "model name")
		repoRoot    = fs.String("repo-root", "", "repo root")
		sessionDir  = fs.String("session-dir", "", "session dir")
		sessionID   = fs.String("session-id", "", "session id")
		recentTurns = fs.Int("recent-turns", 0, "recent turns to include (0 uses config/default)")
		stream      = fs.Bool("stream", true, "stream model output")
		enableRun   = fs.Bool("enable-run-command", false, "enable run_command tool")
		allowPrefix multiStringFlag
		enableBash  = fs.Bool("enable-bash", false, "enable bash tool")
		allowBash   multiStringFlag
		enableHTTP  = fs.Bool("enable-http-get", false, "enable http_get tool")
		allowURL    multiStringFlag
		enableDuck  = fs.Bool("enable-duckdb", false, "enable duckdb_query tool")
		enableWeb   = fs.Bool("enable-web-search", false, "enable web_search tool")
		webProvider = fs.String("web-search-provider", "", "search provider (default brave)")
		braveKey    = fs.String("brave-api-key", "", "Brave API key")
		allowDomain multiStringFlag
		webMax      = fs.Int("web-search-max-results", 0, "max search results")
	)
	fs.Var(&allowPrefix, "allow-cmd-prefix", "allowlisted command prefix (repeatable)")
	fs.Var(&allowBash, "allow-bash-prefix", "allowlisted bash script prefix (repeatable)")
	fs.Var(&allowURL, "allow-url-prefix", "allowlisted URL prefix (repeatable)")
	fs.Var(&allowDomain, "allow-search-domain", "allowlisted search domain (repeatable)")
	_ = fs.Parse(args)

	if *prompt == "" {
		usage()
		os.Exit(2)
	}

	cfg := resolveConfig(*configPath, *baseURL, *apiKey, *model, *repoRoot, *sessionDir, *recentTurns, *enableRun, allowPrefix)
	cfg.Stream = *stream
	if *enableBash {
		cfg.EnableBash = true
	}
	if len(allowBash) > 0 {
		cfg.AllowBashPrefix = allowBash
	}
	if *enableHTTP {
		cfg.EnableHTTPGet = true
	}
	if len(allowURL) > 0 {
		cfg.AllowURLPrefix = allowURL
	}
	if *enableDuck {
		cfg.EnableDuckDB = true
	}
	if *enableWeb {
		cfg.EnableWebSearch = true
	}
	if *webProvider != "" {
		cfg.WebSearchProvider = *webProvider
	}
	if *braveKey != "" {
		cfg.BraveAPIKey = *braveKey
	}
	if len(allowDomain) > 0 {
		cfg.AllowSearchDomain = allowDomain
	}
	if *webMax > 0 {
		cfg.WebSearchMaxResult = *webMax
	}
	sid := *sessionID
	if sid == "" {
		sid = newSessionID()
	}

	eng, store, err := buildEngine(cfg, sid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	_ = store.EnsureDir()

	ctx := context.Background()
	if cfg.Stream {
		evCh, errCh := eng.RunStream(ctx, sid, *prompt)
		for ev := range evCh {
			switch ev.Type {
			case agent.EventAssistantDelta:
				fmt.Print(ev.Text)
			case agent.EventToolStart:
				fmt.Fprintf(os.Stderr, "\n[tool] %s\n", ev.ToolName)
			case agent.EventToolEnd:
				fmt.Fprintf(os.Stderr, "[tool done] %s\n", ev.ToolName)
			case agent.EventFinal:
			}
		}
		if err := <-errCh; err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println("")
	} else {
		res, err := eng.Run(ctx, sid, *prompt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println(res.Reply)
	}
}

func runTools(args []string) {
	fs := flag.NewFlagSet("tools", flag.ExitOnError)
	var (
		configPath = fs.String("config", "", "config file path (default ./rlmkit.json if present)")
		repoRoot   = fs.String("repo-root", "", "repo root")
		sessionDir = fs.String("session-dir", "", "session dir")
		enableRun  = fs.Bool("enable-run-command", false, "enable run_command tool")
		allowCmd   multiStringFlag
		enableBash = fs.Bool("enable-bash", false, "enable bash tool")
		allowBash  multiStringFlag
		enableHTTP = fs.Bool("enable-http-get", false, "enable http_get tool")
		allowURL   multiStringFlag
		enableDuck = fs.Bool("enable-duckdb", false, "enable duckdb_query tool")
		enableWeb  = fs.Bool("enable-web-search", false, "enable web_search tool")
		webProv    = fs.String("web-search-provider", "", "search provider (default brave)")
		braveKey   = fs.String("brave-api-key", "", "Brave API key")
		allowDom   multiStringFlag
		webMax     = fs.Int("web-search-max-results", 0, "max search results")
	)
	fs.Var(&allowCmd, "allow-cmd-prefix", "allowlisted command prefix (repeatable)")
	fs.Var(&allowBash, "allow-bash-prefix", "allowlisted bash script prefix (repeatable)")
	fs.Var(&allowURL, "allow-url-prefix", "allowlisted URL prefix (repeatable)")
	fs.Var(&allowDom, "allow-search-domain", "allowlisted search domain (repeatable)")
	_ = fs.Parse(args)

	cfg := resolveConfig(*configPath, "", "", "", *repoRoot, *sessionDir, 0, *enableRun, allowCmd)
	if *enableBash {
		cfg.EnableBash = true
	}
	if len(allowBash) > 0 {
		cfg.AllowBashPrefix = allowBash
	}
	if *enableHTTP {
		cfg.EnableHTTPGet = true
	}
	if len(allowURL) > 0 {
		cfg.AllowURLPrefix = allowURL
	}
	if *enableDuck {
		cfg.EnableDuckDB = true
	}
	if *enableWeb {
		cfg.EnableWebSearch = true
	}
	if *webProv != "" {
		cfg.WebSearchProvider = *webProv
	}
	if *braveKey != "" {
		cfg.BraveAPIKey = *braveKey
	}
	if len(allowDom) > 0 {
		cfg.AllowSearchDomain = allowDom
	}
	if *webMax > 0 {
		cfg.WebSearchMaxResult = *webMax
	}

	tools, _, err := buildTools(cfg, "tools")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	var out []map[string]any
	for _, t := range tools.All() {
		out = append(out, map[string]any{
			"name":        t.Name(),
			"description": t.Description(),
			"schema":      t.InputSchema(),
		})
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}

func resolveConfig(configPath, baseURL, apiKey, model, repoRoot, sessionDir string, recentTurns int, enableRun bool, allowPrefix []string) FileConfig {
	fc, ok, err := loadFileConfig(configPath)
	if err == nil && ok {
		// start from file config; overlay flags below.
	} else {
		fc = FileConfig{}
	}

	if baseURL != "" {
		fc.BaseURL = baseURL
	}
	if apiKey != "" {
		fc.APIKey = apiKey
	}
	if model != "" {
		fc.Model = model
	}
	if repoRoot != "" {
		fc.RepoRoot = repoRoot
	}
	if sessionDir != "" {
		fc.SessionDir = sessionDir
	}
	if recentTurns != 0 {
		fc.RecentTurns = recentTurns
	}
	if enableRun {
		fc.EnableRunCommand = true
	}
	if len(allowPrefix) > 0 {
		fc.AllowCommandPrefix = allowPrefix
	}

	// defaults
	if fc.BaseURL == "" {
		fc.BaseURL = "http://127.0.0.1:8080/v1"
	}
	if fc.RepoRoot == "" {
		cwd, _ := os.Getwd()
		fc.RepoRoot = cwd
	}
	if fc.SessionDir == "" {
		fc.SessionDir = filepath.Join(fc.RepoRoot, "sessions")
	}
	if fc.RecentTurns == 0 {
		fc.RecentTurns = 2
	}
	if fc.ToolTimeoutSec == 0 {
		fc.ToolTimeoutSec = 60
	}
	if !fc.Stream {
		fc.Stream = true
	}
	if fc.MaxToolConcurrency == 0 {
		fc.MaxToolConcurrency = 4
	}
	if fc.MaxIterations == 0 {
		fc.MaxIterations = 25
	}
	if fc.WebSearchProvider == "" {
		fc.WebSearchProvider = "brave"
	}
	if fc.WebSearchMaxResult == 0 {
		fc.WebSearchMaxResult = 8
	}
	if fc.BraveAPIKey == "" {
		fc.BraveAPIKey = os.Getenv("BRAVE_SEARCH_API_KEY")
	}
	return fc
}

func buildEngine(cfg FileConfig, sessionID string) (*agent.Engine, *session.Store, error) {
	return buildEngineWithPrompt(cfg, sessionID, "default")
}

func buildEngineWithPrompt(cfg FileConfig, sessionID string, mode string) (*agent.Engine, *session.Store, error) {
	tools, store, err := buildTools(cfg, sessionID)
	if err != nil {
		return nil, nil, err
	}

	systemPrompt := agent.DefaultSystemPrompt
	if mode == "coding" {
		systemPrompt = coding.SystemPromptCoding
	}

	llm := openai.NewClient(cfg.BaseURL, cfg.APIKey, 120*time.Second)
	model := strings.TrimSpace(cfg.Model)
	if model == "" || model == "auto" {
		ids, err := llm.Models(context.Background())
		if err != nil {
			return nil, nil, fmt.Errorf("model not provided and failed to auto-detect via %s/models: %w", cfg.BaseURL, err)
		}
		if len(ids) == 0 {
			return nil, nil, fmt.Errorf("no models returned by %s/models", cfg.BaseURL)
		}
		model = ids[0]
		fmt.Fprintf(os.Stderr, "auto-selected model: %s\n", model)
	}
	eng, err := agent.New(llm, tools, store, agent.Config{
		Model:              model,
		SystemPrompt:       systemPrompt,
		RecentTurns:        cfg.RecentTurns,
		MaxIterations:      cfg.MaxIterations,
		MaxToolConcurrency: cfg.MaxToolConcurrency,
		ToolTimeout:        time.Duration(cfg.ToolTimeoutSec) * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}

	return eng, store, nil
}

func buildTools(cfg FileConfig, sessionID string) (*core.Registry, *session.Store, error) {
	store := session.NewStore(cfg.SessionDir)
	tools := core.NewRegistry()

	// Use /dev/tty for ask_user to avoid fighting the main stdin scanner.
	tty, _ := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	var reader *bufio.Reader
	if tty != nil {
		reader = bufio.NewReader(tty)
	}
	p := builtin.BasicPrompter{
		In: func() (string, error) {
			if reader == nil {
				return "", fmt.Errorf("no tty available for ask_user")
			}
			s, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			return strings.TrimRight(s, "\r\n"), nil
		},
		Out: func(s string) { fmt.Fprint(os.Stderr, s) },
	}

	builtin.RegisterAll(tools, builtin.BuiltinConfig{
		RepoRoot:             cfg.RepoRoot,
		SessionStore:         store,
		SessionID:            sessionID,
		EnableRunCommand:     cfg.EnableRunCommand,
		AllowedCommandPrefix: cfg.AllowCommandPrefix,
		EnableBash:           cfg.EnableBash,
		AllowedBashPrefix:    cfg.AllowBashPrefix,
		EnableHTTPGet:        cfg.EnableHTTPGet,
		AllowedURLPrefix:     cfg.AllowURLPrefix,
		EnableDuckDB:         cfg.EnableDuckDB,
		EnableWebSearch:      cfg.EnableWebSearch,
		WebSearchProvider:    cfg.WebSearchProvider,
		BraveAPIToken:        cfg.BraveAPIKey,
		AllowSearchDomain:    cfg.AllowSearchDomain,
		WebSearchMaxResults:  cfg.WebSearchMaxResult,
		UserPrompter:         p,
	})

	return tools, store, nil
}

type multiStringFlag []string

func (m *multiStringFlag) String() string { return strings.Join(*m, ",") }
func (m *multiStringFlag) Set(s string) error {
	*m = append(*m, s)
	return nil
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("session_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
