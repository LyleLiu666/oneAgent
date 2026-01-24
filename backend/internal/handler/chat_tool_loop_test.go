package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
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

	combined, _, err := runToolLoop(
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
	if !strings.Contains(last.Content, "unknown tool") {
		t.Fatalf("expected unknown tool error payload, got %q", last.Content)
	}
}
