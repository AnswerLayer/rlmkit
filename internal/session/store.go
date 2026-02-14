package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.dir, 0o755)
}

func (s *Store) PathFor(sessionID string) string {
	return filepath.Join(s.dir, sessionID+".jsonl")
}

type ToolCallRecord struct {
	Name       string          `json:"name"`
	Input      json.RawMessage `json:"input"`
	Output     string          `json:"output"`
	StartedAt  time.Time       `json:"started_at"`
	DurationMs int64           `json:"duration_ms"`
	Error      string          `json:"error,omitempty"`
}

type TurnRecord struct {
	Type      string           `json:"type"` // "turn"
	SessionID string           `json:"session_id"`
	Timestamp time.Time        `json:"timestamp"`
	UserInput string           `json:"user_input"`
	Assistant string           `json:"assistant"`
	ToolCalls []ToolCallRecord `json:"tool_calls,omitempty"`
}

func (s *Store) AppendTurn(ctx context.Context, rec TurnRecord) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	p := s.PathFor(rec.SessionID)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	defer bw.Flush()

	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := bw.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

type SessionContextRequest struct {
	LastN            *int `json:"last_n,omitempty"`
	IncludeToolCalls bool `json:"include_tool_calls,omitempty"`
}

type TurnSummary struct {
	UserInput string `json:"user_input"`
	Assistant string `json:"assistant"`
	Tools     []struct {
		Name   string `json:"name"`
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	} `json:"tools,omitempty"`
}

type SessionContextResponse struct {
	SessionID string        `json:"session_id"`
	TurnCount int           `json:"turn_count"`
	Turns     []TurnSummary `json:"turns"`
}

func (s *Store) GetSessionContext(ctx context.Context, sessionID string, req SessionContextRequest) (SessionContextResponse, error) {
	p := s.PathFor(sessionID)
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SessionContextResponse{SessionID: sessionID, TurnCount: 0, Turns: nil}, nil
		}
		return SessionContextResponse{}, err
	}
	defer f.Close()

	var turns []TurnRecord
	sc := bufio.NewScanner(f)
	// Allow moderately large lines (tool outputs are truncated elsewhere).
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 2*1024*1024)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return SessionContextResponse{}, ctx.Err()
		default:
		}

		var tr TurnRecord
		if err := json.Unmarshal(sc.Bytes(), &tr); err != nil {
			// Skip malformed lines rather than failing the session.
			continue
		}
		if tr.Type != "turn" {
			continue
		}
		turns = append(turns, tr)
	}
	if err := sc.Err(); err != nil {
		return SessionContextResponse{}, err
	}

	total := len(turns)
	start := 0
	if req.LastN != nil && *req.LastN >= 0 && *req.LastN < total {
		start = total - *req.LastN
	}

	out := SessionContextResponse{
		SessionID: sessionID,
		TurnCount: total,
		Turns:     make([]TurnSummary, 0, total-start),
	}

	for _, t := range turns[start:] {
		ts := TurnSummary{
			UserInput: truncate(t.UserInput, 2000),
			Assistant: truncate(t.Assistant, 2000),
		}
		if req.IncludeToolCalls {
			for _, tc := range t.ToolCalls {
				ts.Tools = append(ts.Tools, struct {
					Name   string `json:"name"`
					Output string `json:"output"`
					Error  string `json:"error,omitempty"`
				}{
					Name:   tc.Name,
					Output: truncate(tc.Output, 2000),
					Error:  tc.Error,
				})
			}
		}
		out.Turns = append(out.Turns, ts)
	}

	return out, nil
}

// LoadRecentTurns loads the last N turns (or fewer) for internal prompt construction.
func (s *Store) LoadRecentTurns(ctx context.Context, sessionID string, lastN int) ([]TurnRecord, error) {
	if lastN <= 0 {
		return nil, nil
	}

	p := s.PathFor(sessionID)
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var turns []TurnRecord
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 2*1024*1024)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var tr TurnRecord
		if err := json.Unmarshal(sc.Bytes(), &tr); err != nil {
			continue
		}
		if tr.Type != "turn" {
			continue
		}
		turns = append(turns, tr)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if len(turns) <= lastN {
		return turns, nil
	}
	return turns[len(turns)-lastN:], nil
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("...(+%d chars)", len(s)-max)
}
