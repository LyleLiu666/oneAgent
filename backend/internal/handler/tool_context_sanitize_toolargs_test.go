package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
)

func TestBuildLLMHistoryFromMessages_SanitizesInvalidToolCallArguments(t *testing.T) {
	callPayload, err := marshalPersistedToolCall("json", "calling", "calling", []llm.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "dummy",
				Arguments: `{"x":`,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshalPersistedToolCall: %v", err)
	}

	msgs := []model.ChatMessage{
		{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeToolCall,
			Content: callPayload,
		},
	}

	history := buildLLMHistoryFromMessages(msgs, "json")
	if len(history) != 1 {
		t.Fatalf("expected 1 history message, got %d", len(history))
	}
	if len(history[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(history[0].ToolCalls))
	}

	args := history[0].ToolCalls[0].Function.Arguments
	if strings.TrimSpace(args) == "" {
		t.Fatalf("expected arguments to be non-empty")
	}
	if !json.Valid([]byte(args)) {
		t.Fatalf("expected arguments to be valid JSON, got %q", args)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		t.Fatalf("expected arguments to be JSON object, got %q: %v", args, err)
	}
	if _, ok := parsed["_raw"]; !ok {
		t.Fatalf("expected sanitized arguments to include _raw, got %q", args)
	}
}

func TestBuildLLMHistoryFromMessages_UnquotesJSONToolArgumentsStrings(t *testing.T) {
	callPayload, err := marshalPersistedToolCall("json", "calling", "calling", []llm.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "dummy",
				Arguments: `"{\"x\":\"ok\"}"`,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshalPersistedToolCall: %v", err)
	}

	msgs := []model.ChatMessage{
		{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeToolCall,
			Content: callPayload,
		},
	}

	history := buildLLMHistoryFromMessages(msgs, "json")
	if len(history) != 1 {
		t.Fatalf("expected 1 history message, got %d", len(history))
	}
	if len(history[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(history[0].ToolCalls))
	}

	args := strings.TrimSpace(history[0].ToolCalls[0].Function.Arguments)
	if args != `{"x":"ok"}` {
		t.Fatalf("expected arguments to be unquoted JSON object, got %q", args)
	}
}
