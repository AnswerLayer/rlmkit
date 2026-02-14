package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL string, apiKey string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolDef struct {
	Type     string          `json:"type"` // "function"
	Function ToolDefFunction `json:"function"`
}

type ToolDefFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type ChatCompletionRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	Tools      []ToolDef `json:"tools,omitempty"`
	ToolChoice any       `json:"tool_choice,omitempty"` // "auto"
	Stream     bool      `json:"stream,omitempty"`
	MaxTokens  int       `json:"max_tokens,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *Client) ChatCompletions(ctx context.Context, req ChatCompletionRequest) (Message, string, error) {
	httpReq, _, err := c.newChatRequest(ctx, req)
	if err != nil {
		return Message{}, "", err
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Message{}, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Message{}, "", fmt.Errorf("model HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out ChatCompletionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return Message{}, "", fmt.Errorf("invalid model JSON: %w", err)
	}
	if out.Error != nil {
		return Message{}, "", errors.New(out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return Message{}, "", errors.New("no choices in response")
	}

	msg := out.Choices[0].Message
	return msg, out.Choices[0].FinishReason, nil
}

func (c *Client) newChatRequest(ctx context.Context, req ChatCompletionRequest) (*http.Request, []byte, error) {
	u := c.baseURL + "/chat/completions"
	b, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return httpReq, b, nil
}

func ExtractTextContent(msg Message) string {
	switch v := msg.Content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		// Some servers return content as array of parts. We keep MVP simple:
		// if it isn't a string, JSON-encode for visibility.
		b, _ := json.Marshal(v)
		return string(b)
	}
}
