package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type fakeToolCaller struct {
	results []llm.ChatCompletionResult
	errs    []error
	calls   int
}

func (f *fakeToolCaller) ChatCompletionWithTools(ctx context.Context, msgs []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	idx := f.calls
	f.calls++
	if idx >= len(f.results) {
		return llm.ChatCompletionResult{Content: ""}, nil
	}
	var err error
	if idx < len(f.errs) {
		err = f.errs[idx]
	}
	return f.results[idx], err
}

func TestToolcallingMetrics_JSONToolResultIncludesStructuredResults(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("sessionstore.New: %v", err)
	}

	sessionID := uuid.NewString()
	userID := "u_1"
	if _, err := sessions.GetOrCreateSession(sessionID, userID, "", ""); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	handlerCalled := 0
	defs := []tool.Definition{
		{
			ID: "dummy",
			Spec: llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name: "dummy",
					Parameters: map[string]any{
						"type":                 "object",
						"properties":           map[string]any{"x": map[string]any{"type": "string"}},
						"required":             []string{"x"},
						"additionalProperties": false,
					},
				},
			},
			Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
				handlerCalled++
				return map[string]any{"ok": true, "echo": "y"}, nil
			},
		},
	}

	client := &fakeToolCaller{
		results: []llm.ChatCompletionResult{
			{
				Content: "calling tool",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "dummy",
							Arguments: `{"x":"ok"}`,
						},
					},
				},
			},
			{
				Content: "done",
			},
		},
	}

	broadcaster := &StreamBroadcaster{clients: map[chan StreamEvent]bool{}}
	out, persisted, err := runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "go"}},
		&llm.ChatCompletionOptions{},
		defs,
		broadcaster,
		sessionID,
		userID,
		nil,
		"test-model",
		false,
		sessions,
	)
	if err != nil {
		t.Fatalf("runToolLoop error: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty output")
	}
	if !persisted {
		t.Fatalf("expected persisted=true")
	}
	if handlerCalled != 1 {
		t.Fatalf("expected tool handler called once, got %d", handlerCalled)
	}

	_, msgs, err := sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		t.Fatalf("GetSessionWithMessages: %v", err)
	}

	var found model.ChatMessage
	for _, msg := range msgs {
		if msg.Type == model.MessageTypeToolResult {
			found = msg
			break
		}
	}
	if found.ID == 0 {
		t.Fatalf("expected a tool result message")
	}

	payload, ok := parsePersistedToolResult(found.Content)
	if !ok {
		t.Fatalf("expected persisted tool result payload JSON, got: %q", found.Content)
	}
	if payload.Protocol != "json" {
		t.Fatalf("expected protocol=json, got %q", payload.Protocol)
	}
	if len(payload.Results) == 0 {
		t.Fatalf("expected structured results for json protocol")
	}
	got := payload.Results[0]
	if got.ToolName != "dummy" || got.ToolCallID != "call_1" {
		t.Fatalf("unexpected structured result: %+v", got)
	}
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
}

func TestToolcallingMetrics_JSONToolArgsRequiredValidationRejectsBeforeHandler(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("sessionstore.New: %v", err)
	}

	sessionID := uuid.NewString()
	userID := "u_1"
	if _, err := sessions.GetOrCreateSession(sessionID, userID, "", ""); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	handlerCalled := 0
	defs := []tool.Definition{
		{
			ID: "dummy",
			Spec: llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name: "dummy",
					Parameters: map[string]any{
						"type":                 "object",
						"properties":           map[string]any{"x": map[string]any{"type": "string"}},
						"required":             []string{"x"},
						"additionalProperties": false,
					},
				},
			},
			Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
				handlerCalled++
				return map[string]any{"ok": true}, nil
			},
		},
	}

	client := &fakeToolCaller{
		results: []llm.ChatCompletionResult{
			{
				Content: "calling tool",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "dummy",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				Content: "done",
			},
		},
	}

	broadcaster := &StreamBroadcaster{clients: map[chan StreamEvent]bool{}}
	_, _, err = runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "go"}},
		&llm.ChatCompletionOptions{},
		defs,
		broadcaster,
		sessionID,
		userID,
		nil,
		"test-model",
		false,
		sessions,
	)
	if err != nil {
		t.Fatalf("runToolLoop error: %v", err)
	}
	if handlerCalled != 0 {
		t.Fatalf("expected tool handler not called, got %d", handlerCalled)
	}

	_, msgs, err := sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		t.Fatalf("GetSessionWithMessages: %v", err)
	}

	var found model.ChatMessage
	for _, msg := range msgs {
		if msg.Type == model.MessageTypeToolResult {
			found = msg
			break
		}
	}
	if found.ID == 0 {
		t.Fatalf("expected a tool result message")
	}

	payload, ok := parsePersistedToolResult(found.Content)
	if !ok {
		t.Fatalf("expected persisted tool result payload JSON, got: %q", found.Content)
	}
	if payload.Protocol != "json" {
		t.Fatalf("expected protocol=json, got %q", payload.Protocol)
	}
	if len(payload.Results) == 0 {
		t.Fatalf("expected structured results for json protocol")
	}
	got := payload.Results[0]
	if got.OK {
		t.Fatalf("expected ok=false, got %+v", got)
	}
	if got.Error == "" {
		t.Fatalf("expected error to be populated, got %+v", got)
	}
}

