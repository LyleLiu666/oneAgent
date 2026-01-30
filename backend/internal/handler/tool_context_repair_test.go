package handler

import (
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
)

func TestBuildLLMHistoryFromMessages_DropsOrphanToolResultMessages(t *testing.T) {
	callPayload, err := marshalPersistedToolCall("json", "calling", "calling", []llm.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "dummy",
				Arguments: `{"x":"ok"}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshalPersistedToolCall: %v", err)
	}

	orphanResult, err := marshalPersistedToolResult("json", "call_orphan", "dummy", `{"x":"ok"}`, `{"ok":true}`, nil)
	if err != nil {
		t.Fatalf("marshalPersistedToolResult(orphan): %v", err)
	}

	validResult, err := marshalPersistedToolResult("json", "call_1", "dummy", `{"x":"ok"}`, `{"ok":true}`, nil)
	if err != nil {
		t.Fatalf("marshalPersistedToolResult(valid): %v", err)
	}

	msgs := []model.ChatMessage{
		{
			Role:    model.MessageRoleAssistant,
			Type:    model.MessageTypeToolCall,
			Content: callPayload,
		},
		{
			Role:    model.MessageRoleTool,
			Type:    model.MessageTypeToolResult,
			Content: orphanResult,
		},
		{
			Role:    model.MessageRoleTool,
			Type:    model.MessageTypeToolResult,
			Content: validResult,
		},
	}

	history := buildLLMHistoryFromMessages(msgs, "json")
	if len(history) != 2 {
		t.Fatalf("expected 2 history messages (tool_call + matched tool_result), got %d", len(history))
	}
	if history[0].Role != model.MessageRoleAssistant || len(history[0].ToolCalls) != 1 || history[0].ToolCalls[0].ID != "call_1" {
		t.Fatalf("unexpected tool call message: %+v", history[0])
	}
	if history[1].Role != model.MessageRoleTool || history[1].ToolCallID != "call_1" {
		t.Fatalf("unexpected tool result message: %+v", history[1])
	}
}
