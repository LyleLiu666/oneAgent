package handler

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type scriptedToolClient struct {
	results      []llm.ChatCompletionResult
	callMessages [][]llm.ChatMessage
	index        int
}

func (c *scriptedToolClient) ChatCompletionWithTools(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	_ = ctx
	_ = opts
	c.callMessages = append(c.callMessages, append([]llm.ChatMessage(nil), messages...))
	if c.index >= len(c.results) {
		return llm.ChatCompletionResult{}, nil
	}
	res := c.results[c.index]
	c.index++
	return res, nil
}

func TestRunToolLoop_SelfHeal_UnknownTool(t *testing.T) {
	defs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	client := &scriptedToolClient{
		results: []llm.ChatCompletionResult{
			{
				Content: "",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_0",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "does_not_exist",
						Arguments: `{}`,
					},
				}},
			},
			{
				Content:   "done",
				ToolCalls: nil,
			},
		},
	}

	sm := NewStreamManager()
	broadcaster := sm.GetOrCreate("session-1")

	store, err := sessionstore.New(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	if _, err := store.GetOrCreateSession("session-1", "user-1", ChatModule, "title"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	combined, _, _, err := runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "hi"}},
		nil,
		defs,
		broadcaster,
		"session-1",
		"user-1",
		nil,
		"model-1",
		false,
		store,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected combined %q, got %q", "done", combined)
	}

	if len(client.callMessages) < 2 {
		t.Fatalf("expected at least 2 llm calls, got %d", len(client.callMessages))
	}
	second := client.callMessages[1]
	if len(second) == 0 {
		t.Fatalf("expected second call to include messages")
	}
	last := second[len(second)-1]
	if last.Role != model.MessageRoleTool {
		t.Fatalf("expected last role %q, got %q", model.MessageRoleTool, last.Role)
	}
	if last.ToolCallID != "call_0" {
		t.Fatalf("expected tool_call_id=%q, got %q", "call_0", last.ToolCallID)
	}
	if last.Name != "does_not_exist" {
		t.Fatalf("expected tool name=%q, got %q", "does_not_exist", last.Name)
	}
	if !strings.Contains(last.Content, "BEGIN_UNTRUSTED_CONTENT") || !strings.Contains(last.Content, "END_UNTRUSTED_CONTENT") {
		t.Fatalf("expected untrusted boundary markers, got %q", last.Content)
	}
	if !strings.Contains(last.Content, "unknown tool") {
		t.Fatalf("expected unknown tool error payload, got %q", last.Content)
	}
}

type scriptedToolClientWithErrors struct {
	results      []llm.ChatCompletionResult
	errs         []error
	callMessages [][]llm.ChatMessage
	index        int
}

func (c *scriptedToolClientWithErrors) ChatCompletionWithTools(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	_ = ctx
	_ = opts
	c.callMessages = append(c.callMessages, append([]llm.ChatMessage(nil), messages...))

	if c.index < len(c.errs) && c.errs[c.index] != nil {
		err := c.errs[c.index]
		c.index++
		return llm.ChatCompletionResult{}, err
	}

	if c.index >= len(c.results) {
		c.index++
		return llm.ChatCompletionResult{}, nil
	}

	res := c.results[c.index]
	c.index++
	return res, nil
}

func TestRunToolLoop_StopsAfterMaxStepsEnvOverride(t *testing.T) {
	t.Setenv("ONEAGENT_CHAT_TOOL_MAX_STEPS", "3")

	defs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	client := &scriptedToolClient{
		results: []llm.ChatCompletionResult{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call_0",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "does_not_exist",
						Arguments: `{}`,
					},
				}},
			},
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "does_not_exist",
						Arguments: `{}`,
					},
				}},
			},
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call_2",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "does_not_exist",
						Arguments: `{}`,
					},
				}},
			},
		},
	}

	sm := NewStreamManager()
	broadcaster := sm.GetOrCreate("session-1")

	store, err := sessionstore.New(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	if _, err := store.GetOrCreateSession("session-1", "user-1", ChatModule, "title"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, _, _, err = runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "hi"}},
		nil,
		defs,
		broadcaster,
		"session-1",
		"user-1",
		nil,
		"model-1",
		false,
		store,
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tool call limit reached") {
		t.Fatalf("expected tool call limit reached error, got %v", err)
	}
	if client.index != 3 {
		t.Fatalf("expected 3 llm calls, got %d", client.index)
	}
}

func TestRunToolLoop_MapsInvalidArgumentsErrorToStructuredPayload(t *testing.T) {
	defs := []tool.Definition{
		{
			ID: "dummy",
			Spec: llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name:        "dummy",
					Description: "dummy tool",
					Parameters: map[string]any{
						"type":                 "object",
						"properties":           map[string]any{},
						"required":             []string{},
						"additionalProperties": false,
					},
				},
			},
			Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
				_ = ctx
				_ = raw
				return nil, &tool.InvalidArgumentsError{ToolName: "dummy", Message: "bad args"}
			},
		},
	}

	client := &scriptedToolClient{
		results: []llm.ChatCompletionResult{
			{
				Content: "",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_0",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "dummy",
						Arguments: `{}`,
					},
				}},
			},
			{
				Content:   "done",
				ToolCalls: nil,
			},
		},
	}

	sm := NewStreamManager()
	broadcaster := sm.GetOrCreate("session-1")

	store, err := sessionstore.New(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	if _, err := store.GetOrCreateSession("session-1", "user-1", ChatModule, "title"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, _, _, err = runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "hi"}},
		nil,
		defs,
		broadcaster,
		"session-1",
		"user-1",
		nil,
		"model-1",
		false,
		store,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, msgs, err := store.GetSessionWithMessages("session-1", "user-1")
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

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload.Content), &decoded); err != nil {
		t.Fatalf("expected tool result content to be json, got %v (%q)", err, payload.Content)
	}
	if decoded["error"] != "invalid_arguments" {
		t.Fatalf("expected error=invalid_arguments, got %+v", decoded)
	}
	if decoded["invalid_args"] != true {
		t.Fatalf("expected invalid_args=true, got %+v", decoded)
	}
	msg, _ := decoded["message"].(string)
	if !strings.Contains(msg, "bad args") {
		t.Fatalf("expected message to contain %q, got %q", "bad args", msg)
	}
}

func TestRunToolLoop_RetriesOnInvalidFunctionArgumentsAPIError(t *testing.T) {
	defs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	client := &scriptedToolClientWithErrors{
		errs: []error{
			&llm.APIError{
				StatusCode: 400,
				Type:       "invalid_request_error",
				Message:    "invalid params, invalid function arguments json string, tool_call_id: call_1",
			},
			nil,
		},
		results: []llm.ChatCompletionResult{
			{},
			{Content: "done"},
		},
	}

	sm := NewStreamManager()
	broadcaster := sm.GetOrCreate("session-1")

	store, err := sessionstore.New(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	if _, err := store.GetOrCreateSession("session-1", "user-1", ChatModule, "title"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	combined, _, _, err := runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "hi"}},
		nil,
		defs,
		broadcaster,
		"session-1",
		"user-1",
		nil,
		"model-1",
		false,
		store,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected combined %q, got %q", "done", combined)
	}
	if client.index != 2 {
		t.Fatalf("expected 2 llm calls, got %d", client.index)
	}
	if len(client.callMessages) < 2 {
		t.Fatalf("expected retry to call llm twice, got %d", len(client.callMessages))
	}
	if got := client.callMessages[1]; len(got) < 2 || got[len(got)-1].Role != model.MessageRoleUser {
		t.Fatalf("expected retry call to append a user repair message, got %+v", got)
	}
}

func TestRunToolLoop_InjectsInvocationMetaIntoHandlerContext(t *testing.T) {
	var seen tool.InvocationMeta
	defs := []tool.Definition{
		{
			ID: "dummy",
			Spec: llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name:        "dummy",
					Description: "dummy tool",
					Parameters: map[string]any{
						"type":                 "object",
						"properties":           map[string]any{},
						"required":             []string{},
						"additionalProperties": false,
					},
				},
			},
			Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
				_ = raw
				meta, ok := tool.InvocationMetaFromContext(ctx)
				if !ok {
					t.Fatalf("expected invocation meta in context")
				}
				seen = meta
				return map[string]any{"ok": true}, nil
			},
		},
	}

	client := &scriptedToolClient{
		results: []llm.ChatCompletionResult{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call_0",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "dummy",
						Arguments: `{}`,
					},
				}},
			},
			{Content: "done"},
		},
	}

	sm := NewStreamManager()
	broadcaster := sm.GetOrCreate("session-1")
	store, err := sessionstore.New(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	if _, err := store.GetOrCreateSession("session-1", "user-1", ChatModule, "title"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := tool.ContextWithInvocationMeta(context.Background(), tool.InvocationMeta{
		RunID:  "chat:session-1",
		TurnID: "turn-1",
	})

	_, _, _, err = runToolLoop(
		ctx,
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "hi"}},
		nil,
		defs,
		broadcaster,
		"session-1",
		"user-1",
		nil,
		"model-1",
		false,
		store,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if seen.RunID != "chat:session-1" || seen.TurnID != "turn-1" {
		t.Fatalf("expected run/turn ids in handler context, got %+v", seen)
	}
	if seen.ToolCallID != "call_0" || seen.ToolName != "dummy" || seen.Protocol != "json" {
		t.Fatalf("expected per-call invocation metadata, got %+v", seen)
	}
}

func TestRunToolLoop_DurablePersistedFalseWhenFinalAssistantWriteFails(t *testing.T) {
	defs := []tool.Definition{
		{
			ID: "dummy",
			Spec: llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name:        "dummy",
					Description: "dummy tool",
					Parameters: map[string]any{
						"type":                 "object",
						"properties":           map[string]any{},
						"required":             []string{},
						"additionalProperties": false,
					},
				},
			},
			Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
				_ = ctx
				_ = raw
				return map[string]any{"ok": true}, nil
			},
		},
	}

	client := &scriptedToolClient{
		results: []llm.ChatCompletionResult{
			{
				Content: "",
				ToolCalls: []llm.ToolCall{{
					ID:   "call_0",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "dummy",
						Arguments: `{}`,
					},
				}},
			},
			{
				Content: "done",
			},
		},
	}

	store := &failingToolLoopSessionStore{failAppendAt: 3}
	broadcaster := &StreamBroadcaster{clients: map[chan StreamEvent]bool{}}
	combined, persisted, durable, err := runToolLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: model.MessageRoleUser, Content: "hi"}},
		nil,
		defs,
		broadcaster,
		"session-1",
		"user-1",
		nil,
		"model-1",
		false,
		store,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected combined %q, got %q", "done", combined)
	}
	if !persisted {
		t.Fatalf("expected persisted=true because earlier tool messages were written")
	}
	if durable {
		t.Fatalf("expected durable=false when final assistant persistence fails")
	}
	if store.appendCalls != 3 {
		t.Fatalf("expected 3 append attempts, got %d", store.appendCalls)
	}
}

type failingToolLoopSessionStore struct {
	appendCalls  int
	failAppendAt int
}

func (f *failingToolLoopSessionStore) AppendMessage(sessionID string, msg model.ChatMessage) (model.ChatMessage, error) {
	_ = sessionID
	f.appendCalls++
	if f.failAppendAt > 0 && f.appendCalls == f.failAppendAt {
		return model.ChatMessage{}, errors.New("append failed")
	}
	msg.ID = uint(f.appendCalls)
	return msg, nil
}

func (f *failingToolLoopSessionStore) UpdateMessageTrace(sessionID string, messageID uint, trace model.TraceDataJSON) error {
	_ = sessionID
	_ = messageID
	_ = trace
	return nil
}
