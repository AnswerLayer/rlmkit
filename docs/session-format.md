# Session Format (JSONL)

Sessions are stored as JSON Lines (one JSON object per line).

Default location:
- `--session-dir` (default `<repo-root>/sessions`)
- file: `sessions/<session_id>.jsonl`

## Turn Record

Each completed turn is appended as a `TurnRecord`:

```json
{
  "type": "turn",
  "session_id": "abcd1234...",
  "timestamp": "2026-02-14T20:00:00Z",
  "user_input": "…",
  "assistant": "…",
  "tool_calls": [
    {
      "name": "read_file",
      "input": {"path":"README.md","max_bytes":200000},
      "output": "…",
      "started_at": "2026-02-14T20:00:01Z",
      "duration_ms": 12,
      "error": ""
    }
  ]
}
```

Notes:
- Tool `output` is truncated before writing (to keep sessions small).
- Session context retrieval (`get_session_context`) returns compact summaries and truncates long fields.

## RLM Retrieval

Two ways history is used:
- `recent_turns`: the engine loads the last N turns (text only) into the prompt.
- `get_session_context`: a tool the model can call to fetch older turns on demand.

