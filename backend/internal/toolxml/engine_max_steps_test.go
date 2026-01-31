package toolxml

import (
	"context"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestRunLoop_StopsAfterMaxStepsEnvOverride(t *testing.T) {
	t.Setenv("ONEAGENT_CHAT_TOOL_MAX_STEPS", "3")

	defs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	client := &scriptedClient{
		responses: []string{
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
		},
	}

	_, err = RunLoop(
		context.Background(),
		client,
		[]llm.ChatMessage{{Role: "user", Content: "run"}},
		nil,
		defs,
		"",
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "xml tool call limit reached") {
		t.Fatalf("expected xml tool call limit reached error, got %v", err)
	}
	if client.index != 3 {
		t.Fatalf("expected 3 llm calls, got %d", client.index)
	}
}
