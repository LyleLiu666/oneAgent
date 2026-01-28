package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestRunToolLoop_SkillRead_ByName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillPath := filepath.Join(home, ".claude", "skills", "code-review-excellence", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("---\nname: code-review-excellence\ndescription: review\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	defs, err := tool.Mount([]string{tool.ToolIDSkillRead})
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
						Name:      "skill.read",
						Arguments: `{"name":"code-review-excellence"}`,
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

	store, err := sessionstore.New(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	if _, err := store.GetOrCreateSession("session-1", "user-1", ChatModule, "title"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	ctx = tool.ContextWithSkillManager(ctx, skill.NewManager(0))

	combined, _, err := runToolLoop(
		ctx,
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
		store,
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
	last := second[len(second)-1]
	if last.Role != model.MessageRoleTool {
		t.Fatalf("expected last role %q, got %q", model.MessageRoleTool, last.Role)
	}
	if last.ToolCallID != "call_0" {
		t.Fatalf("expected tool_call_id=%q, got %q", "call_0", last.ToolCallID)
	}
	if last.Name != "skill.read" {
		t.Fatalf("expected tool name=%q, got %q", "skill.read", last.Name)
	}
	if !strings.Contains(last.Content, "BEGIN_UNTRUSTED_CONTENT") || !strings.Contains(last.Content, "END_UNTRUSTED_CONTENT") {
		t.Fatalf("expected untrusted boundary markers, got %q", last.Content)
	}
	if !strings.Contains(last.Content, "skill_md") {
		t.Fatalf("expected skill_md in tool output, got %q", last.Content)
	}
	if !strings.Contains(last.Content, "code-review-excellence") {
		t.Fatalf("expected skill name in tool output, got %q", last.Content)
	}
}
