package agentsdk

import (
	"context"
	"time"
)

// Message is a minimal chat message format for SDK consumers.
// Protocol adapters (e.g. XML tool calling) may append additional messages to
// drive multi-step tool loops.
type Message struct {
	Role    string
	Content string
}

// StreamCallback is called for each streamed chunk.
// Returning a non-nil error aborts the stream.
type StreamCallback func(chunk string) error

// ChatCompletionOptions configures the LLM request.
// This type is intentionally minimal and SDK-owned so the agentsdk package can
// be extracted into a standalone module later.
type ChatCompletionOptions struct {
	Model       string
	Temperature *float64
	MaxTokens   *int
	Stop        []string
}

// Client is the minimal LLM interface required by the agent loop.
type Client interface {
	ChatCompletionStream(ctx context.Context, messages []Message, opts *ChatCompletionOptions, callback StreamCallback) error
}

type EventKind string

const (
	EventKindLLMRequest  EventKind = "llm_request"
	EventKindLLMResponse EventKind = "llm_response"
	EventKindToolCall    EventKind = "tool_call"
	EventKindToolResult  EventKind = "tool_result"
	EventKindTrace       EventKind = "trace"
	EventKindError       EventKind = "error"
)

// Event is a structured, append-only log record emitted by agent loops.
// SDK consumers can persist these events for full observability and replay.
type Event struct {
	Kind     EventKind
	Protocol string
	Step     int
	Time     time.Time
	Payload  any
}

type EventSink interface {
	OnEvent(ctx context.Context, event Event)
}

type EventSinkFunc func(ctx context.Context, event Event)

func (f EventSinkFunc) OnEvent(ctx context.Context, event Event) {
	if f == nil {
		return
	}
	f(ctx, event)
}

type LLMRequestEvent struct {
	Messages []Message
	Options  *ChatCompletionOptions
}

type LLMResponseEvent struct {
	Raw     string
	Visible string
	Error   string
}

type ToolCallEvent struct {
	ID     string
	Name   string
	Fields map[string]string
	Raw    string
}

type ToolResultEvent struct {
	ToolName   string
	ToolCallID string
	OK         bool
	OutputJSON string
	Error      string
}

type ErrorEvent struct {
	Error string
}

type TraceEvent struct {
	Message string
}
