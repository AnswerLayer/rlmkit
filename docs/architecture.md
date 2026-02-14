# Architecture

rlmkit is a minimal agent runtime:
- OpenAI-compatible model client (tool calling + optional streaming)
- Tool registry + bounded-concurrency tool executor
- JSONL session store (RLM pattern)
- CLI wrappers (`chat`, `code`, `-p`)

## Data Flow (One Turn)

1. CLI collects `user_input`.
2. Engine constructs a small prompt:
   - `system` prompt
   - last `recent_turns` from the session store (text only)
   - current `user` message
3. Engine calls the model with tool definitions.
4. If the model returns tool calls:
   - Engine executes tools with bounded concurrency and timeouts.
   - Tool results are appended as `tool` messages.
   - Loop back to step 3.
5. When the model returns a final assistant message:
   - Engine appends a `TurnRecord` to the session JSONL.

## RLM Pattern

The core rule is: do not stuff full history into every model call.

Instead:
- Persist turns locally (JSONL).
- Include only a small number of recent turns in the prompt.
- Provide a `get_session_context` tool so the model can fetch older context on demand.

## Key Packages

- `internal/agent`
  - Agent loop (`Engine.Run`, `Engine.RunStream`)
  - Tool-call orchestration (bounded concurrency)
- `internal/llm/openai`
  - OpenAI-compatible HTTP client (`/chat/completions`)
  - SSE streaming parser (OpenAI-style `data: ...` chunks)
- `internal/tools/core`
  - `Tool` interface + registry
- `internal/tools/builtin`
  - Built-in repo + session tools
- `internal/session`
  - JSONL store format and read APIs

