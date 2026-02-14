package core

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	Execute(ctx context.Context, in json.RawMessage) (ToolResult, error)
}

type ToolResult struct {
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Registry struct {
	byName map[string]Tool
	order  []Tool
}

func NewRegistry() *Registry {
	return &Registry{
		byName: map[string]Tool{},
		order:  nil,
	}
}

func (r *Registry) Register(t Tool) {
	if _, exists := r.byName[t.Name()]; !exists {
		r.order = append(r.order, t)
	}
	r.byName[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.byName[name]
	return t, ok
}

func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	out = append(out, r.order...)
	return out
}
