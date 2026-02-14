package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/answerlayer/rlmkit/internal/llm/openai"
	"github.com/answerlayer/rlmkit/internal/session"
	"github.com/answerlayer/rlmkit/internal/tools/core"
	"sync"
)

type Config struct {
	Model              string
	SystemPrompt       string
	RecentTurns        int
	MaxIterations      int
	MaxToolConcurrency int64
	ToolTimeout        time.Duration
}

type Engine struct {
	llm   *openai.Client
	tools *core.Registry
	store *session.Store
	cfg   Config
}

func New(llm *openai.Client, tools *core.Registry, store *session.Store, cfg Config) (*Engine, error) {
	if llm == nil || tools == nil || store == nil {
		return nil, errors.New("nil dependency")
	}
	if cfg.Model == "" {
		return nil, errors.New("missing model")
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 25
	}
	if cfg.MaxToolConcurrency <= 0 {
		cfg.MaxToolConcurrency = 4
	}
	if cfg.ToolTimeout <= 0 {
		cfg.ToolTimeout = 60 * time.Second
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt
	}
	if cfg.RecentTurns < 0 {
		cfg.RecentTurns = 0
	}
	return &Engine{llm: llm, tools: tools, store: store, cfg: cfg}, nil
}

type Result struct {
	SessionID string
	Reply     string
	ToolCalls []session.ToolCallRecord
}

func (e *Engine) Run(ctx context.Context, sessionID string, userInput string) (Result, error) {
	if sessionID == "" {
		return Result{}, errors.New("missing sessionID")
	}
	if userInput == "" {
		return Result{}, errors.New("empty input")
	}

	messages := []openai.Message{{Role: "system", Content: e.cfg.SystemPrompt}}

	if e.cfg.RecentTurns > 0 {
		turns, err := e.store.LoadRecentTurns(ctx, sessionID, e.cfg.RecentTurns)
		if err == nil {
			for _, t := range turns {
				// Keep history minimal: only user + assistant text.
				if t.UserInput != "" {
					messages = append(messages, openai.Message{Role: "user", Content: t.UserInput})
				}
				if t.Assistant != "" {
					messages = append(messages, openai.Message{Role: "assistant", Content: t.Assistant})
				}
			}
		}
	}

	messages = append(messages, openai.Message{Role: "user", Content: userInput})

	toolDefs := e.buildToolDefs()
	var toolRecords []session.ToolCallRecord

	for i := 0; i < e.cfg.MaxIterations; i++ {
		req := openai.ChatCompletionRequest{
			Model:      e.cfg.Model,
			Messages:   messages,
			Tools:      toolDefs,
			ToolChoice: "auto",
			Stream:     false,
		}

		msg, finish, err := e.llm.ChatCompletions(ctx, req)
		if err != nil {
			return Result{}, err
		}

		if len(msg.ToolCalls) == 0 {
			reply := openai.ExtractTextContent(msg)
			if reply == "" && finish == "tool_calls" {
				// Some servers emit tool_calls with empty content; but in this branch we have none.
				reply = "(empty response)"
			}

			// Persist turn
			rec := session.TurnRecord{
				Type:      "turn",
				SessionID: sessionID,
				Timestamp: time.Now(),
				UserInput: userInput,
				Assistant: reply,
				ToolCalls: toolRecords,
			}
			_ = e.store.AppendTurn(ctx, rec)

			return Result{
				SessionID: sessionID,
				Reply:     reply,
				ToolCalls: toolRecords,
			}, nil
		}

		// Append assistant tool call message.
		messages = append(messages, openai.Message{
			Role:      "assistant",
			Content:   openai.ExtractTextContent(msg),
			ToolCalls: msg.ToolCalls,
		})

		// Execute tool calls (bounded concurrency, deterministic ordering).
		toolResults, records, err := e.execToolCalls(ctx, msg.ToolCalls)
		if err != nil {
			return Result{}, err
		}
		toolRecords = append(toolRecords, records...)

		// Append tool results back to model.
		for _, tr := range toolResults {
			messages = append(messages, tr)
		}
	}

	return Result{}, fmt.Errorf("max iterations reached (%d)", e.cfg.MaxIterations)
}

func (e *Engine) buildToolDefs() []openai.ToolDef {
	all := e.tools.All()
	defs := make([]openai.ToolDef, 0, len(all))
	for _, t := range all {
		defs = append(defs, openai.ToolDef{
			Type: "function",
			Function: openai.ToolDefFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.InputSchema(),
			},
		})
	}
	return defs
}

func (e *Engine) execToolCalls(ctx context.Context, calls []openai.ToolCall) ([]openai.Message, []session.ToolCallRecord, error) {
	maxConc := e.cfg.MaxToolConcurrency
	if maxConc <= 0 {
		maxConc = 1
	}
	sem := make(chan struct{}, maxConc)

	type item struct {
		msg    openai.Message
		record session.ToolCallRecord
	}
	out := make([]item, len(calls))

	var wg sync.WaitGroup
	for i := range calls {
		i := i
		call := calls[i]
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			start := time.Now()
			rec := session.ToolCallRecord{
				Name:      call.Function.Name,
				StartedAt: start,
			}

			tool, ok := e.tools.Get(call.Function.Name)
			if !ok {
				rec.Error = "unknown tool"
				rec.DurationMs = time.Since(start).Milliseconds()
				out[i] = item{
					msg: openai.Message{
						Role:       "tool",
						ToolCallID: call.ID,
						Name:       call.Function.Name,
						Content:    "Error: unknown tool",
					},
					record: rec,
				}
				return
			}

			in := json.RawMessage(call.Function.Arguments)
			rec.Input = in

			toolCtx, cancel := context.WithTimeout(ctx, e.cfg.ToolTimeout)
			defer cancel()

			res, err := tool.Execute(toolCtx, in)
			if err != nil {
				rec.Error = err.Error()
				rec.Output = ""
			} else {
				rec.Output = truncateToolOutput(res.Content, 50000)
			}
			rec.DurationMs = time.Since(start).Milliseconds()

			content := res.Content
			if err != nil {
				content = "Error: " + err.Error()
			}
			content = truncateToolOutput(content, 50000)

			out[i] = item{
				msg: openai.Message{
					Role:       "tool",
					ToolCallID: call.ID,
					Name:       call.Function.Name,
					Content:    content,
				},
				record: rec,
			}
		}()
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	msgs := make([]openai.Message, 0, len(out))
	recs := make([]session.ToolCallRecord, 0, len(out))
	for _, it := range out {
		msgs = append(msgs, it.msg)
		recs = append(recs, it.record)
	}
	return msgs, recs, nil
}

func truncateToolOutput(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
