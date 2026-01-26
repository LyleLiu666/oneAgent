package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
