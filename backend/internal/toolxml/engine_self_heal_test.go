package toolxml

import (
	"context"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestRunLoop_SelfHeal_TruncatedToolData(t *testing.T) {
	defs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name>run_command</tool_name>`,
			`done`,
		},
	}

	var steps []StepRecord
	combined, err := RunLoop(
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
		func(step StepRecord) { steps = append(steps, step) },
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected final content %q, got %q", "done", combined)
	}
	if len(steps) == 0 {
		t.Fatalf("expected at least one step record")
	}
	if len(steps[0].ToolResults) == 0 {
		t.Fatalf("expected protocol error step to include tool result")
	}
	if steps[0].ToolResults[0].OK {
		t.Fatalf("expected protocol error to be ok=false")
	}
}

func TestRunLoop_SelfHeal_ParseToolDataError(t *testing.T) {
	defs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name></tool_name>
  </call>
</tool_data>`,
			`done`,
		},
	}

	combined, err := RunLoop(
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
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected final content %q, got %q", "done", combined)
	}
}

func TestBuildToolResultMessage_EscapesCDATAEndMarker(t *testing.T) {
	msg := buildToolResultMessage([]ToolResult{{
		ToolName:   "bash",
		ToolCallID: "xml_0_0",
		OK:         false,
		OutputJSON: `{"out":"]]> inside"}`,
		Error:      `bad ]]> error`,
	}})
	if !strings.Contains(msg, "]]]]><![CDATA[>") {
		t.Fatalf("expected CDATA escape sequence, got %q", msg)
	}
}
