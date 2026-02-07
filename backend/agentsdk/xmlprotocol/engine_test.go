package xmlprotocol

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/agentsdk"
)

type scriptedClient struct {
	responses []string
	index     int
}

func (c *scriptedClient) ChatCompletionStream(ctx context.Context, messages []agentsdk.Message, opts *agentsdk.ChatCompletionOptions, callback agentsdk.StreamCallback) error {
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

type funcExecutor func(context.Context, ToolCall) (any, error)

func (f funcExecutor) Execute(ctx context.Context, call ToolCall) (any, error) {
	return f(ctx, call)
}

type collectSink struct {
	events []agentsdk.Event
}

func (s *collectSink) OnEvent(ctx context.Context, event agentsdk.Event) {
	_ = ctx
	s.events = append(s.events, event)
}

func TestRunLoop_EmitsEventSink(t *testing.T) {
	client := &scriptedClient{
		responses: []string{`done`},
	}
	sink := &collectSink{}

	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			return nil, nil
		}),
		Callbacks: Callbacks{
			EventSink: sink,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected %q, got %q", "done", combined)
	}

	var sawReq bool
	var sawResp bool
	for _, ev := range sink.events {
		if ev.Protocol != "xml" {
			t.Fatalf("expected protocol=xml, got %q", ev.Protocol)
		}
		switch ev.Kind {
		case agentsdk.EventKindLLMRequest:
			sawReq = true
			payload, ok := ev.Payload.(agentsdk.LLMRequestEvent)
			if !ok {
				t.Fatalf("expected LLMRequestEvent payload, got %T", ev.Payload)
			}
			if len(payload.Messages) == 0 || payload.Messages[0].Role != "user" {
				t.Fatalf("unexpected request messages: %#v", payload.Messages)
			}
		case agentsdk.EventKindLLMResponse:
			sawResp = true
			payload, ok := ev.Payload.(agentsdk.LLMResponseEvent)
			if !ok {
				t.Fatalf("expected LLMResponseEvent payload, got %T", ev.Payload)
			}
			if strings.TrimSpace(payload.Visible) != "done" {
				t.Fatalf("unexpected visible response: %q", payload.Visible)
			}
			if strings.TrimSpace(payload.Raw) != "done" {
				t.Fatalf("unexpected raw response: %q", payload.Raw)
			}
		}
	}
	if !sawReq {
		t.Fatalf("expected at least one llm_request event")
	}
	if !sawResp {
		t.Fatalf("expected at least one llm_response event")
	}
}

func TestRunLoop_EmitsTraceEvents_AndEventsAreJSONSerializable(t *testing.T) {
	client := &scriptedClient{
		responses: []string{
			`<tool_data><call><tool_name>bash</tool_name><command>echo hi</command></call></tool_data>`,
			`done`,
		},
	}
	sink := &collectSink{}

	var traces []string
	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			return map[string]any{"stdout": "hi"}, nil
		}),
		Callbacks: Callbacks{
			OnTrace: func(msg string) { traces = append(traces, msg) },
			EventSink: sink,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected %q, got %q", "done", combined)
	}

	var sawTrace bool
	var sawToolCall bool
	var sawToolResult bool
	for _, ev := range sink.events {
		if _, err := json.Marshal(ev); err != nil {
			t.Fatalf("expected event to be json-serializable (kind=%s), got %v", ev.Kind, err)
		}

		switch ev.Kind {
		case agentsdk.EventKindTrace:
			sawTrace = true
			payload, ok := ev.Payload.(agentsdk.TraceEvent)
			if !ok {
				t.Fatalf("expected TraceEvent payload, got %T", ev.Payload)
			}
			if payload.Message == "" {
				t.Fatalf("expected trace message to be non-empty")
			}
		case agentsdk.EventKindToolCall:
			sawToolCall = true
		case agentsdk.EventKindToolResult:
			sawToolResult = true
		}
	}

	if len(traces) == 0 {
		t.Fatalf("expected OnTrace to be called")
	}
	if !sawTrace {
		t.Fatalf("expected at least one trace event")
	}
	if !sawToolCall {
		t.Fatalf("expected at least one tool_call event")
	}
	if !sawToolResult {
		t.Fatalf("expected at least one tool_result event")
	}
}

func TestRunLoop_StopsAfterMaxSteps(t *testing.T) {
	client := &scriptedClient{
		responses: []string{
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
		},
	}

	_, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			return nil, errors.New("unknown tool")
		}),
		MaxSteps: 3,
	})
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

func TestRunLoop_SelfHeal_TruncatedToolData(t *testing.T) {
	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name>run_command</tool_name>`,
			`done`,
		},
	}

	var steps []StepRecord
	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			return map[string]any{"ok": true}, nil
		}),
		Callbacks: Callbacks{
			ObserveStep: func(step StepRecord) { steps = append(steps, step) },
		},
	})
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
	if steps[0].ToolResults[0].ToolName != "tool_protocol" {
		t.Fatalf("expected tool_protocol result, got %q", steps[0].ToolResults[0].ToolName)
	}
}

func TestRunLoop_SelfHeal_ParseToolDataError(t *testing.T) {
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

	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			return map[string]any{"ok": true}, nil
		}),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected final content %q, got %q", "done", combined)
	}
}

func TestBuildToolResultMessage_EscapesXMLText(t *testing.T) {
	msg := buildToolResultMessage([]ToolResult{{
		ToolName:   "bash",
		ToolCallID: "xml_0_0",
		OK:         false,
		OutputJSON: `{"out":"<x>&"}`,
		Error:      `bad <error> & value`,
	}})
	if strings.Contains(msg, "<![CDATA[") {
		t.Fatalf("expected message to not contain CDATA, got %q", msg)
	}
	if !strings.Contains(msg, "&lt;x>") {
		t.Fatalf("expected output to escape '<', got %q", msg)
	}
	if !strings.Contains(msg, "&amp;") {
		t.Fatalf("expected output to escape '&', got %q", msg)
	}
}

func TestRunLoop_TruncatedToolData_WriteFileAppend_IsRecoveredAndWritten(t *testing.T) {
	root := t.TempDir()

	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name>write_file</tool_name>
    <filePath>out.md</filePath>
    <append>true</append>
    <content>
hello
world`,
			`done`,
		},
	}

	var steps []StepRecord
	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(ctx context.Context, call ToolCall) (any, error) {
			_ = ctx
			if canonicalToolName(call.Name) != "write_file" {
				return nil, errors.New("unknown tool")
			}
			rel := strings.TrimSpace(call.Fields["filePath"])
			if rel == "" {
				return nil, errors.New("missing filePath")
			}
			content := call.Fields["content"]
			appendMode := parseBool(call.Fields["append"])
			target := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return nil, err
			}
			if appendMode {
				f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				if _, err := f.WriteString(content); err != nil {
					return nil, err
				}
			} else {
				if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
					return nil, err
				}
			}
			return map[string]any{"ok": true}, nil
		}),
		Callbacks: Callbacks{
			ObserveStep: func(step StepRecord) { steps = append(steps, step) },
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected final content %q, got %q", "done", combined)
	}

	if len(steps) != 1 {
		t.Fatalf("expected 1 tool step, got %d", len(steps))
	}

	var gotWrite bool
	var gotProtocol bool
	for _, r := range steps[0].ToolResults {
		switch r.ToolName {
		case "write_file":
			gotWrite = true
			if !r.OK {
				t.Fatalf("expected write_file OK, got error %v", r.Error)
			}
		case "tool_protocol":
			gotProtocol = true
		}
	}
	if !gotWrite {
		t.Fatalf("expected write_file tool result, got %#v", steps[0].ToolResults)
	}
	if !gotProtocol {
		t.Fatalf("expected tool_protocol warning result, got %#v", steps[0].ToolResults)
	}

	content, err := os.ReadFile(filepath.Join(root, "out.md"))
	if err != nil {
		t.Fatalf("read out.md: %v", err)
	}
	if string(content) != "hello\nworld" {
		t.Fatalf("unexpected file content: %q", string(content))
	}
}

func TestRunLoop_TruncatedToolData_WriteFileWithoutAppend_IsNotRecovered(t *testing.T) {
	root := t.TempDir()

	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name>write_file</tool_name>
    <filePath>out.md</filePath>
    <content>
hello`,
			`done`,
		},
	}

	var steps []StepRecord
	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			t.Fatalf("executor should not be called for non-recoverable truncated write_file")
			return nil, nil
		}),
		Callbacks: Callbacks{
			ObserveStep: func(step StepRecord) { steps = append(steps, step) },
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(combined) != "done" {
		t.Fatalf("expected final content %q, got %q", "done", combined)
	}

	if _, err := os.Stat(filepath.Join(root, "out.md")); err == nil {
		t.Fatalf("expected out.md to not be created")
	}

	if len(steps) != 1 {
		t.Fatalf("expected 1 tool step, got %d", len(steps))
	}
	if len(steps[0].ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(steps[0].ToolResults))
	}
	if steps[0].ToolResults[0].ToolName != "tool_protocol" {
		t.Fatalf("expected tool_protocol result, got %q", steps[0].ToolResults[0].ToolName)
	}
	if steps[0].ToolResults[0].OK {
		t.Fatalf("expected tool_protocol ok=false")
	}
	if !strings.Contains(steps[0].ToolResults[0].Error, "truncated <tool_data> block") {
		t.Fatalf("expected truncated error, got %q", steps[0].ToolResults[0].Error)
	}
}

func TestRecoverTruncatedWriteFileAppend_ParsesFields(t *testing.T) {
	recovered, ok := recoverTruncatedWriteFileAppend(`<tool_data>
  <call>
    <tool_name>write_file</tool_name>
    <filePath>out.md</filePath>
    <append>true</append>
    <content>
hello
world`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if recovered.FilePath != "out.md" {
		t.Fatalf("expected filePath=%q, got %q", "out.md", recovered.FilePath)
	}
	if recovered.Content != "hello\nworld" {
		t.Fatalf("expected content %q, got %q", "hello\\nworld", recovered.Content)
	}
	if !strings.Contains(recovered.RepairedAssistantContent, "</tool_data>") {
		t.Fatalf("expected repaired assistant content to include </tool_data>")
	}
}

func TestRunLoop_ExecutesToolAndReturnsJSONOutput(t *testing.T) {
	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name>run_command</tool_name>
    <command>echo hi</command>
  </call>
</tool_data>`,
			`done`,
		},
	}

	var steps []StepRecord
	combined, err := RunLoop(context.Background(), RunLoopInput{
		Client:   client,
		Messages: []agentsdk.Message{{Role: "user", Content: "run"}},
		Executor: funcExecutor(func(context.Context, ToolCall) (any, error) {
			return map[string]any{"stdout_delta": "hi\n"}, nil
		}),
		Callbacks: Callbacks{
			ObserveStep: func(step StepRecord) { steps = append(steps, step) },
		},
	})
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
