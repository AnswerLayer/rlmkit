# rlmkit

Minimal RLM coding-agent toolkit (local models, tool-driven context).

rlmkit uses the RLM pattern (Retrieval-augmented Language Model) for agent state:
instead of stuffing the full conversation history into every model call, the
agent persists turns locally and retrieves prior context via a tool call when
needed (`get_session_context`).

## Status

This is an early MVP. Expect breaking changes.

## Quickstart

Prereqs:
- Go 1.22+
- A local OpenAI-compatible server backed by MLX (macOS)

See:
- `docs/running-with-mlx.md`
- `docs/architecture.md`
- `docs/session-format.md`
- `docs/tools.md`
- `docs/streaming.md`
- `docs/releases.md`

Run:

```bash
go test ./...
go run ./cmd/rlmkit chat --repo-root . --base-url http://127.0.0.1:8080/v1 --model auto
```

Coding mode (more opinionated prompt):

```bash
go run ./cmd/rlmkit code --repo-root . --base-url http://127.0.0.1:8080/v1 --model auto
```

One-shot:

```bash
go run ./cmd/rlmkit -p "Summarize this repository structure." --repo-root .
```

Streaming is enabled by default. Disable with `--stream=false`.

Print the currently available tools and schemas:

```bash
go run ./cmd/rlmkit tools --repo-root .
```

Enable web search (Brave):

```bash
export BRAVE_SEARCH_API_KEY=...
go run ./cmd/rlmkit code --repo-root . --base-url http://127.0.0.1:8082/v1 --model auto \
  --enable-web-search --web-search-provider brave --allow-search-domain github.com --allow-search-domain docs.python.org
```

## Safety

By default `run_command` is disabled. Enable it only if you trust the agent and
have configured a strict allowlist.
