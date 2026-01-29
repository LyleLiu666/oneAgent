package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/skill"
)

func TestSkillReadTool_ResolvesByNameWithPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()

	codex := filepath.Join(home, ".codex", "skills", "same", "SKILL.md")
	claude := filepath.Join(home, ".claude", "skills", "same", "SKILL.md")
	oneagent := filepath.Join(workspace, ".oneagent", "skills", "same", "SKILL.md")

	for _, p := range []string{codex, claude, oneagent} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(codex, []byte("---\nname: same\ndescription: codex\n---\n"), 0o644); err != nil {
		t.Fatalf("write codex: %v", err)
	}
	if err := os.WriteFile(claude, []byte("---\nname: same\ndescription: claude\n---\n"), 0o644); err != nil {
		t.Fatalf("write claude: %v", err)
	}
	if err := os.WriteFile(oneagent, []byte("---\nname: same\ndescription: oneagent\n---\n"), 0o644); err != nil {
		t.Fatalf("write oneagent: %v", err)
	}

	manager := skill.NewManager(0)
	ctx := context.Background()
	ctx = ContextWithSkillManager(ctx, manager)
	ctx = ContextWithWorkspace(ctx, WorkspaceConfig{Enabled: true, Root: workspace})

	raw, _ := json.Marshal(map[string]any{
		"name": "same",
	})
	gotAny, err := runSkillReadTool(ctx, raw)
	if err != nil {
		t.Fatalf("skill.read: %v", err)
	}
	got := gotAny.(skillReadResult)
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	if got.Source != skill.SourceOneAgent {
		t.Fatalf("expected source=.oneagent, got %+v", got)
	}
	if got.SkillID != "same" {
		t.Fatalf("expected skill_id=same, got %+v", got)
	}
	if got.Name != "same" {
		t.Fatalf("expected name=same, got %+v", got)
	}
	if got.SkillMD == "" {
		t.Fatalf("expected skill_md content")
	}
	want, _ := filepath.EvalSymlinks(oneagent)
	gotPath, _ := filepath.EvalSymlinks(got.Path)
	if want != "" && gotPath != "" && want != gotPath {
		t.Fatalf("expected path=%q, got %q", want, gotPath)
	}
}

func TestSkillReadTool_ReadsBuiltinSkillWithoutWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	manager := skill.NewManager(0)
	ctx := context.Background()
	ctx = ContextWithSkillManager(ctx, manager)

	raw, _ := json.Marshal(map[string]any{
		"name": "create-skill",
	})
	gotAny, err := runSkillReadTool(ctx, raw)
	if err != nil {
		t.Fatalf("skill.read: %v", err)
	}
	got := gotAny.(skillReadResult)
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	if got.Source != skill.SourceBuiltin {
		t.Fatalf("expected source=.builtin, got %+v", got)
	}
	if !strings.Contains(got.SkillMD, "# create-skill") {
		t.Fatalf("expected skill_md content, got=%q", got.SkillMD)
	}
}

func TestSkillReadTool_NotFoundIncludesSuggestionsAndNextSteps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("code-review-%d", i)
		p := filepath.Join(home, ".claude", "skills", id, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("---\nname: "+id+"\ndescription: test\n---\n"), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	manager := skill.NewManager(0)
	ctx := context.Background()
	ctx = ContextWithSkillManager(ctx, manager)

	raw, _ := json.Marshal(map[string]any{
		"name": "code review",
	})
	_, err := runSkillReadTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "normalized: code-review") {
		t.Fatalf("expected normalized id in error, got %q", msg)
	}
	if !strings.Contains(msg, "Did you mean:") {
		t.Fatalf("expected suggestions header in error, got %q", msg)
	}
	if !strings.Contains(msg, "/governance/skills") || !strings.Contains(msg, "oneagent skills status") {
		t.Fatalf("expected next steps in error, got %q", msg)
	}

	sectionStart := strings.Index(msg, "Did you mean:")
	sectionEnd := strings.Index(msg, "\n\nNext steps:")
	if sectionStart == -1 || sectionEnd == -1 || sectionEnd <= sectionStart {
		t.Fatalf("expected to find suggestions section, got %q", msg)
	}
	suggestionSection := msg[sectionStart:sectionEnd]
	lines := strings.Split(suggestionSection, "\n")
	suggestions := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(l, "- ") {
			suggestions = append(suggestions, l)
		}
	}
	if len(suggestions) == 0 {
		t.Fatalf("expected at least one suggestion, got %q", msg)
	}
	if len(suggestions) > 5 {
		t.Fatalf("expected <=5 suggestions, got %d: %q", len(suggestions), msg)
	}
}
