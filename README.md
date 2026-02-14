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

Run:

```bash
go test ./...
go run ./cmd/rlmkit chat --repo-root . --base-url http://127.0.0.1:8080/v1 --model <model>
```

One-shot:

```bash
go run ./cmd/rlmkit -p "Summarize this repository structure." --repo-root .
```

## Safety

By default `run_command` is disabled. Enable it only if you trust the agent and
have configured a strict allowlist.

