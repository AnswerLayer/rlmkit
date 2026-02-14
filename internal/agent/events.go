package agent

type EventType string

const (
	EventAssistantDelta EventType = "assistant_delta"
	EventToolStart      EventType = "tool_start"
	EventToolEnd        EventType = "tool_end"
	EventFinal          EventType = "final"
)

type Event struct {
	Type     EventType
	Text     string
	ToolName string
}
