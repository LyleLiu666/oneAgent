package llm

import "testing"

func TestBuildAnthropicPayload_CacheControlCoversToolBlocks(t *testing.T) {
	msgs := []ChatMessage{
		BuildSystemMessage("sys"),
		BuildUserMessage("hi"),
		{
			Role:    "assistant",
			Content: "calling tool",
			ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "rg",
					Arguments: `{"pattern":"x"}`,
				},
			}},
		},
		{
			Role:       "tool",
			ToolCallID: "call_1",
			Name:       "rg",
			Content:    `{"ok":true}`,
		},
	}

	system, converted := buildAnthropicPayload(msgs, true)
	if len(system) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(system))
	}
	if system[0].CacheControl == nil {
		t.Fatalf("expected system block cache_control")
	}
	if len(converted) != 3 {
		t.Fatalf("expected 3 messages (user + assistant + tool_result), got %d", len(converted))
	}

	var (
		assistantMsg  *anthropicMessage
		toolResultMsg *anthropicMessage
	)
	for i := range converted {
		if converted[i].Role == "assistant" {
			assistantMsg = &converted[i]
		}
		if converted[i].Role == "user" && len(converted[i].Content) == 1 && converted[i].Content[0].Type == "tool_result" {
			toolResultMsg = &converted[i]
		}
	}
	if assistantMsg == nil {
		t.Fatalf("assistant message not found")
	}
	if len(assistantMsg.Content) < 2 {
		t.Fatalf("expected assistant to include text + tool_use blocks, got %d", len(assistantMsg.Content))
	}
	for _, block := range assistantMsg.Content {
		if block.CacheControl == nil {
			t.Fatalf("expected assistant block %q to have cache_control", block.Type)
		}
	}
	if toolResultMsg == nil {
		t.Fatalf("tool_result message not found")
	}
	if toolResultMsg.Content[0].CacheControl == nil {
		t.Fatalf("expected tool_result block cache_control")
	}
}
