package toolxml

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestRunLoop_XMLRead_AliasesToReadFile(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := tool.ContextWithWorkspace(context.Background(), tool.WorkspaceConfig{Enabled: true, Root: root})
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	defs, err := tool.Mount([]string{tool.ToolIDReadFile})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	client := &scriptedClient{
		responses: []string{
			`<tool_data>
  <call>
    <tool_name>read</tool_name>
    <filePath>a.txt</filePath>
  </call>
</tool_data>`,
			`done`,
		},
	}

	var steps []StepRecord
	combined, err := RunLoop(
		ctx,
		client,
		[]llm.ChatMessage{{Role: "user", Content: "read file"}},
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
	content, _ := output["content"].(string)
	if content != "hello\nworld\n" {
		t.Fatalf("expected content %q, got %q", "hello\nworld\n", content)
	}
}

