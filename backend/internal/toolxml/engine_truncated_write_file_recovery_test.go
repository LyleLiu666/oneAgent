package toolxml

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestRunLoop_TruncatedToolData_WriteFileAppend_IsRecoveredAndWritten(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := tool.ContextWithWorkspace(context.Background(), tool.WorkspaceConfig{Enabled: true, Root: root})

	defs, err := tool.Mount([]string{tool.ToolIDWriteFile})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

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
	combined, err := RunLoop(
		ctx,
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
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := tool.ContextWithWorkspace(context.Background(), tool.WorkspaceConfig{Enabled: true, Root: root})

	defs, err := tool.Mount([]string{tool.ToolIDWriteFile})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

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
	combined, err := RunLoop(
		ctx,
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
