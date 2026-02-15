# Tools

rlmkit uses tool calling with an OpenAI-compatible schema: each tool is exposed as a `function` with JSON-schema-like `parameters`.

Implementation:
- Tool interface: `internal/tools/core/tool.go`
- Built-ins: `internal/tools/builtin/*`

## Built-in Tools (MVP)

### `list_files`
Lists files under repo root.

Input:
- `glob` (optional)
- `max` (optional, default 2000)

### `read_file`
Reads a file under repo root.

Input:
- `path` (required, relative to repo root)
- `max_bytes` (optional, default 200000)

### `search_repo`
Runs ripgrep (`rg`) under repo root and returns matching lines.

Input:
- `query` (required, ripgrep regex)
- `glob` (optional)
- `max_lines` (optional, default 200)

### `apply_patch`
Applies a unified diff to the repo using `git apply`.

Input:
- `patch` (required)

Notes:
- Requires the target repo to be a git repo (must have `.git/`).
- Runs `git apply --check` then `git apply`.

### `run_command` (disabled by default)
Runs an allowlisted command under repo root.

Input:
- `command` (required)
- `args` (optional)
- `timeout_sec` (optional, default 60)

Safety:
- Must be enabled (`--enable-run-command` or config).
- Must match an allowlisted prefix (`--allow-cmd-prefix` or config).

### `bash` (disabled by default)
Runs a `bash -lc` script under repo root.

Input:
- `script` (required)
- `timeout_sec` (optional, default 60)

Safety:
- Must be enabled (`--enable-bash` or config).
- Script must match an allowlisted prefix (`--allow-bash-prefix` or config).

### `http_get` (disabled by default)
Fetches a URL via HTTP GET.

Input:
- `url` (required)
- `max_bytes` (optional, default 200000)

Safety:
- Must be enabled (`--enable-http-get` or config).
- URL must match an allowlisted prefix (`--allow-url-prefix` or config).

### `web_search` (disabled by default)
Runs web search via a configured provider (currently Brave Search API).

Input:
- `query` (required)
- `count` (optional)
- `freshness` (optional, provider-specific)
- `country` (optional, provider-specific)

Safety/config:
- Must be enabled (`--enable-web-search` or config).
- Provider: `--web-search-provider` (default `brave`).
- Brave API key via `--brave-api-key` or `BRAVE_SEARCH_API_KEY`.
- Optional search-domain allowlist via `--allow-search-domain`.

### `duckdb_query` (disabled by default)
Runs SQL against a local DuckDB file using the `duckdb` CLI.

Input:
- `database_path` (required, relative to repo root)
- `sql` (required)
- `timeout_sec` (optional, default 60)
- `max_bytes` (optional, default 200000)

Safety:
- Must be enabled (`--enable-duckdb` or config).
- Database path is constrained to repo root.

### `ask_user`
Prompts the user for clarification during an agent run.

Input:
- `question` (required)
- `options` (optional)
- `allow_freeform` (optional, default true)

### `get_session_context`
Returns compact summaries of prior turns for the current session (RLM pattern).

Input:
- `last_n` (optional)
- `include_tool_calls` (optional)
