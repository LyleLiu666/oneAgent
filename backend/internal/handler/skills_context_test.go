package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/skill"
)

func TestBuildSkillSuggestionTurnContext_IncludesSkillsHelpWhenAsked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("skill-%02d", i)
		p := filepath.Join(home, ".claude", "skills", id, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("---\nname: "+id+"\ndescription: test\n---\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	manager := skill.NewManager(0)
	out := buildSkillSuggestionTurnContext(context.Background(), manager, "", "有哪些技能可用？", skillTurnContextCaps{})
	if !strings.Contains(out, "## 技能帮助") {
		t.Fatalf("expected skills help header, got %q", out)
	}
	if !strings.Contains(out, "/governance/skills") || !strings.Contains(out, "oneagent skills status") {
		t.Fatalf("expected next steps, got %q", out)
	}

	sectionStart := strings.Index(out, "可用技能摘要（Top-10）：")
	sectionEnd := strings.Index(out, "\n\n提示：")
	if sectionStart == -1 || sectionEnd == -1 || sectionEnd <= sectionStart {
		t.Fatalf("expected to find Top-10 section, got %q", out)
	}
	section := out[sectionStart:sectionEnd]
	lines := strings.Split(section, "\n")
	count := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "- ") {
			count++
		}
	}
	if count == 0 || count > 10 {
		t.Fatalf("expected 1..10 skills in preview, got %d: %q", count, out)
	}
	if !strings.Contains(out, "- skill-00") {
		t.Fatalf("expected preview to include a generated skill, got %q", out)
	}
}

func TestBuildSkillSuggestionTurnContext_ToolkitSkillIncludesToolIDsAndSubagentHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".claude", "skills", "repo-toolkit", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `---
name: repo-toolkit
description: Repo maintenance toolkit
tool_ids: [rg, read_file]
---
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	manager := skill.NewManager(0)
	out := buildSkillSuggestionTurnContext(context.Background(), manager, "", "请使用 repo-toolkit 这个技能", skillTurnContextCaps{
		HasSkillRead: true,
		HasSubagent:  true,
	})
	if !strings.Contains(out, "工具包 (tool_ids): rg, read_file") {
		t.Fatalf("expected tool_ids to be surfaced, got %q", out)
	}
	if !strings.Contains(out, "`subagent`") {
		t.Fatalf("expected subagent hint, got %q", out)
	}
	if !strings.Contains(out, "`skill_read`") {
		t.Fatalf("expected skill_read hint, got %q", out)
	}
}
