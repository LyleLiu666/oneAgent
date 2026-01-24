package toolxml

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type scriptedClient struct {
	responses []string
	index     int
}

func (c *scriptedClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	_ = messages
	_ = opts
	if c.index >= len(c.responses) {
		return "", errors.New("no scripted response")
	}
	resp := c.responses[c.index]
	c.index++
	return resp, nil
}

func (c *scriptedClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	_ = ctx
	_ = messages
	_ = opts
	if c.index >= len(c.responses) {
		return errors.New("no scripted response")
	}
	resp := c.responses[c.index]
	c.index++
	return callback(resp)
}

func TestBuildToolArgs_RunCommand_XMLFields(t *testing.T) {
	input := `<tool_data>
  <call>
    <tool_name>run_command</tool_name>
    <action>start</action>
    <command>echo hi</command>
    <wait_seconds>2</wait_seconds>
    <max_runtime_seconds>600</max_runtime_seconds>
    <stdout_offset>0</stdout_offset>
    <stderr_offset>0</stderr_offset>
  </call>
</tool_data>`

	calls, err := ParseToolData(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	payload, _, err := buildToolArgs(calls[0].ToolName, calls[0].Fields)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if decoded["action"] != "start" {
		t.Fatalf("expected action=start, got %v", decoded["action"])
	}
	if decoded["command"] != "echo hi" {
		t.Fatalf("expected command=echo hi, got %v", decoded["command"])
	}
	if decoded["wait_seconds"] != float64(2) {
		t.Fatalf("expected wait_seconds=2, got %v", decoded["wait_seconds"])
	}
	if decoded["max_runtime_seconds"] != float64(600) {
		t.Fatalf("expected max_runtime_seconds=600, got %v", decoded["max_runtime_seconds"])
	}
}

func TestRunLoop_XMLRunCommand_Executes(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	defs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("expected tool mount ok, got %v", err)
	}

	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name>run_command</tool_name>
    <action>start</action>
    <command>echo hi</command>
    <wait_seconds>2</wait_seconds>
  </call>
</tool_data>`,
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
	if len(steps) != 1 {
		t.Fatalf("expected 1 tool step, got %d", len(steps))
	}
	if len(steps[0].ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(steps[0].ToolResults))
	}
	if !steps[0].ToolResults[0].OK {
		t.Fatalf("expected tool result OK, got error %v", steps[0].ToolResults[0].Error)
	}

	var output map[string]any
	if err := json.Unmarshal([]byte(steps[0].ToolResults[0].OutputJSON), &output); err != nil {
		t.Fatalf("expected json output, got %v", err)
	}
	stdout, _ := output["stdout_delta"].(string)
	if !strings.Contains(stdout, "hi") {
		t.Fatalf("expected stdout_delta to contain %q, got %q", "hi", stdout)
	}
}
