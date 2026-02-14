# Running With MLX (OpenAI-Compatible Server)

rlmkit is a client: it talks to a local OpenAI-compatible HTTP server.

Recommended setup on macOS:
1. Start an MLX-backed OpenAI-compatible server (for example, `mlx-openai-server`).
2. Point rlmkit at it:

```bash
go run ./cmd/rlmkit chat --base-url http://127.0.0.1:8080/v1 --model <model> --repo-root .
```

Why this approach:
- Keeps rlmkit focused on orchestration/tools.
- Avoids coupling the agent to one inference backend.
- Localhost HTTP overhead is negligible compared to model inference time.

