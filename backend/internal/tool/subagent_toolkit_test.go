package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/skill"
)

type capturingSubagentClient struct {
	ToolsSeen         [][]string
	SystemPromptsSeen []string
	Result            llm.ChatCompletionResult
}

func (c *capturingSubagentClient) ChatCompletion(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (string, error) {
	return "", errors.New("not implemented")
}

func (c *capturingSubagentClient) ChatCompletionStream(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions, llm.StreamCallback) error {
	return errors.New("not implemented")
}

func (c *capturingSubagentClient) ChatCompletionWithTools(_ context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	tools := make([]string, 0, len(opts.Tools))
	for _, t := range opts.Tools {
		tools = append(tools, strings.TrimSpace(t.Function.Name))
	}
	c.ToolsSeen = append(c.ToolsSeen, tools)

	for _, m := range messages {
		if m.Role != "system" {
			continue
		}
		c.SystemPromptsSeen = append(c.SystemPromptsSeen, m.Content)
		break
	}

	return c.Result, nil
}

func TestSubagentTool_DefaultToolIDsInheritMountedToolIDsFromContext(t *testing.T) {
	home := t.TempDir()
	layout, err := oneruntime.EnsureLayout(home)
	if err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	workspaceRoot := t.TempDir()

	client := &capturingSubagentClient{
		Result: llm.ChatCompletionResult{Content: `<subagent_handoff><summary>ok</summary></subagent_handoff>`},
	}

	base := context.Background()
	base = ContextWithSessionID(base, "sess-1")
	base = ContextWithUserID(base, "u1")
	base = ContextWithRuntimeLayout(base, layout)
	base = ContextWithLLMClient(base, client)
	base = ContextWithPromptBaseOverride(base, "CUSTOM PROMPT")
	base = ContextWithMountedToolIDs(base, []string{ToolIDSubagent, ToolIDReadFile})
	base = ContextWithWorkspace(base, WorkspaceConfig{Enabled: true, Root: workspaceRoot})

	anyOut, err := runSubagentTool(base, json.RawMessage(`{"task":"step1","k_skills":0}`))
	if err != nil {
		t.Fatalf("runSubagentTool: %v", err)
	}
	out, ok := anyOut.(subagentToolResult)
	if !ok || !out.OK {
		t.Fatalf("expected ok result, got %#v", anyOut)
	}

	if len(client.ToolsSeen) != 1 || len(client.ToolsSeen[0]) != 1 || client.ToolsSeen[0][0] != "read_file" {
		t.Fatalf("expected tools [read_file], got %+v", client.ToolsSeen)
	}
	if len(client.SystemPromptsSeen) == 0 {
		t.Fatalf("expected to capture system prompt")
	}
	sys := client.SystemPromptsSeen[0]
	if !strings.Contains(sys, "CUSTOM PROMPT") {
		t.Fatalf("expected system prompt to include base override, got %q", sys)
	}
	if !strings.Contains(sys, "read_file 工具使用说明") {
		t.Fatalf("expected read_file manual present, got %q", sys)
	}
	if strings.Contains(sys, "rg 工具使用说明") {
		t.Fatalf("expected rg manual excluded, got %q", sys)
	}
}

func TestSubagentTool_ToolkitSkillInfersToolIDsWhenToolIDsOmitted(t *testing.T) {
	home := t.TempDir()
	layout, err := oneruntime.EnsureLayout(home)
	if err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	workspaceRoot := t.TempDir()
	toolkitPath := filepath.Join(workspaceRoot, ".oneagent", "skills", "repo-toolkit", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(toolkitPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `---
name: repo-toolkit
description: Repo toolkit
tool_ids:
  - rg
  - read_file
---
`
	if err := os.WriteFile(toolkitPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &capturingSubagentClient{
		Result: llm.ChatCompletionResult{Content: `<subagent_handoff><summary>ok</summary></subagent_handoff>`},
	}

	base := context.Background()
	base = ContextWithSessionID(base, "sess-1")
	base = ContextWithUserID(base, "u1")
	base = ContextWithRuntimeLayout(base, layout)
	base = ContextWithLLMClient(base, client)
	base = ContextWithSkillManager(base, skill.NewManager(0))
	base = ContextWithPromptBaseOverride(base, "CUSTOM PROMPT")
	base = ContextWithMountedToolIDs(base, []string{ToolIDSubagent})
	base = ContextWithWorkspace(base, WorkspaceConfig{Enabled: true, Root: workspaceRoot})

	anyOut, err := runSubagentTool(base, json.RawMessage(`{"task":"step1","skill_ids":["repo-toolkit"],"k_skills":0}`))
	if err != nil {
		t.Fatalf("runSubagentTool: %v", err)
	}
	out, ok := anyOut.(subagentToolResult)
	if !ok || !out.OK {
		t.Fatalf("expected ok result, got %#v", anyOut)
	}

	if len(client.ToolsSeen) != 1 || len(client.ToolsSeen[0]) != 2 || client.ToolsSeen[0][0] != "read_file" || client.ToolsSeen[0][1] != "rg" {
		t.Fatalf("expected tools [read_file rg], got %+v", client.ToolsSeen)
	}
	if len(client.SystemPromptsSeen) == 0 {
		t.Fatalf("expected to capture system prompt")
	}
	sys := client.SystemPromptsSeen[0]
	if !strings.Contains(sys, "read_file 工具使用说明") || !strings.Contains(sys, "rg 工具使用说明") {
		t.Fatalf("expected tool manuals present, got %q", sys)
	}
	if strings.Contains(sys, "write_file 工具使用说明") {
		t.Fatalf("expected write_file manual excluded, got %q", sys)
	}
}
