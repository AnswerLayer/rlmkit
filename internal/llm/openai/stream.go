package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type StreamEvent struct {
	DeltaText string
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   *string               `json:"content,omitempty"`
			ToolCalls []streamToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type streamToolCallDelta struct {
	Index    int     `json:"index"`
	ID       *string `json:"id,omitempty"`
	Type     *string `json:"type,omitempty"`
	Function *struct {
		Name      *string `json:"name,omitempty"`
		Arguments *string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// ChatCompletionsStream streams deltas and returns the assembled final assistant message.
// It supports OpenAI-compatible SSE responses ("data: {...}" lines terminated by "data: [DONE]").
func (c *Client) ChatCompletionsStream(ctx context.Context, req ChatCompletionRequest, onEvent func(StreamEvent)) (Message, string, error) {
	req.Stream = true

	httpReq, body, err := c.newChatRequest(ctx, req)
	if err != nil {
		return Message{}, "", err
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Message{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		return Message{}, "", fmt.Errorf("model HTTP %d: %s", resp.StatusCode, string(b))
	}

	_ = body // keep body referenced for clarity (newChatRequest may return it)

	var content strings.Builder
	var finishReason string
	var toolCalls []ToolCall

	sc := bufio.NewScanner(resp.Body)
	// Streaming can have fairly large chunks; bump scanner buffer.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 8*1024*1024)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return Message{}, "", ctx.Err()
		default:
		}

		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Ignore malformed chunks.
			continue
		}
		if chunk.Error != nil {
			return Message{}, "", errors.New(chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		ch := chunk.Choices[0]
		if ch.FinishReason != nil && *ch.FinishReason != "" {
			finishReason = *ch.FinishReason
		}

		if ch.Delta.Content != nil && *ch.Delta.Content != "" {
			d := *ch.Delta.Content
			content.WriteString(d)
			if onEvent != nil {
				onEvent(StreamEvent{DeltaText: d})
			}
		}

		if len(ch.Delta.ToolCalls) > 0 {
			for _, td := range ch.Delta.ToolCalls {
				for len(toolCalls) <= td.Index {
					toolCalls = append(toolCalls, ToolCall{
						Type: "function",
						Function: ToolCallFunction{
							Arguments: "",
						},
					})
				}
				tc := toolCalls[td.Index]
				if td.ID != nil {
					tc.ID = *td.ID
				}
				if td.Type != nil {
					tc.Type = *td.Type
				}
				if td.Function != nil {
					if td.Function.Name != nil {
						tc.Function.Name = *td.Function.Name
					}
					if td.Function.Arguments != nil {
						tc.Function.Arguments += *td.Function.Arguments
					}
				}
				toolCalls[td.Index] = tc
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Message{}, "", err
	}

	msg := Message{
		Role:      "assistant",
		Content:   content.String(),
		ToolCalls: toolCalls,
	}
	return msg, finishReason, nil
}
