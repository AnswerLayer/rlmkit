# Streaming

rlmkit supports OpenAI-compatible streaming for `/chat/completions`:
- Request includes `"stream": true`
- Response is Server-Sent Events (SSE) with lines like `data: {...}` and terminates with `data: [DONE]`

Implementation:
- `internal/llm/openai/stream.go`

## What rlmkit Streams

- Assistant text deltas are emitted to the CLI as they arrive.
- Tool calls may arrive as streamed partial JSON argument strings; rlmkit accumulates them by tool-call index until complete.

## Caveats

Streaming behavior varies between OpenAI-compatible servers. If a server returns non-standard SSE framing, streaming may fail and you can fall back to `--stream=false`.

