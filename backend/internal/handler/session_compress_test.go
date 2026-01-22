package handler

import (
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
)

func TestSplitForCompression_KeepLastTwoRounds(t *testing.T) {
	msgs := []model.ChatMessage{
		{ID: 1, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u1"},
		{ID: 2, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a1"},
		{ID: 3, Role: model.MessageRoleAssistant, Type: model.MessageTypeToolCall, Content: `{"protocol":"json"}`},
		{ID: 4, Role: model.MessageRoleTool, Type: model.MessageTypeToolResult, Content: `{"protocol":"json"}`},
		{ID: 5, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u2"},
		{ID: 6, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a2"},
		{ID: 7, Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "u3"},
		{ID: 8, Role: model.MessageRoleAssistant, Type: model.MessageTypeText, Content: "a3"},
	}

	toSummarize, toKeep := splitForCompression(msgs, 4)
	if len(toKeep) != 4 {
		t.Fatalf("expected 4 kept messages, got %d", len(toKeep))
	}
	if toKeep[0].ID != 5 || toKeep[1].ID != 6 || toKeep[2].ID != 7 || toKeep[3].ID != 8 {
		t.Fatalf("unexpected kept IDs: %+v", []uint{toKeep[0].ID, toKeep[1].ID, toKeep[2].ID, toKeep[3].ID})
	}
	if len(toSummarize) != 4 {
		t.Fatalf("expected 4 summarized messages, got %d", len(toSummarize))
	}
	if toSummarize[0].ID != 1 || toSummarize[3].ID != 4 {
		t.Fatalf("unexpected summarized IDs: first=%d last=%d", toSummarize[0].ID, toSummarize[len(toSummarize)-1].ID)
	}
}

func TestFormatForSummaryInput_ToolMessages(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "bash",
				Arguments: `{"cmd":"ls"}`,
			},
		},
	}

	callPayload, err := marshalPersistedToolCall("json", "running bash", "running bash", toolCalls)
	if err != nil {
		t.Fatalf("marshalPersistedToolCall: %v", err)
	}

	resultPayload, err := marshalPersistedToolResult("json", "call_1", "bash", `{"cmd":"ls"}`, `{"ok":true}`, nil)
	if err != nil {
		t.Fatalf("marshalPersistedToolResult: %v", err)
	}

	input := formatForSummaryInput([]model.ChatMessage{
		{ID: 10, Role: model.MessageRoleAssistant, Type: model.MessageTypeToolCall, Content: callPayload},
		{ID: 11, Role: model.MessageRoleTool, Type: model.MessageTypeToolResult, Content: resultPayload},
	})

	if input == "" {
		t.Fatal("expected non-empty summary input")
	}
	if !strings.Contains(input, "tool_call: bash") || !strings.Contains(input, "tool_result: bash") {
		t.Fatalf("unexpected summary input:\n%s", input)
	}
}

func TestApproximateContextRunes_IncludesToolArguments(t *testing.T) {
	msg := llm.ChatMessage{
		Role:    "assistant",
		Content: "",
		ToolCalls: []llm.ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "bash",
				Arguments: "hello",
			},
		}},
	}

	got := approximateContextRunes([]llm.ChatMessage{msg})
	if got < len("hello") {
		t.Fatalf("expected rune count to include tool arguments, got %d", got)
	}
}
