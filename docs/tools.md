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

### `get_session_context`
Returns compact summaries of prior turns for the current session (RLM pattern).

Input:
- `last_n` (optional)
- `include_tool_calls` (optional)

