package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSkill_Frontmatter(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "translator", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `---
name: Translator
description: Expert in translation
tags:
  - translation
keywords: i18n
---

# Translator

Body.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := parseSkill(path, SourceClaude)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.ID != "translator" {
		t.Fatalf("expected id=translator, got %q", got.ID)
	}
	if got.Name != "Translator" {
		t.Fatalf("expected name=Translator, got %q", got.Name)
	}
	if got.Description != "Expert in translation" {
		t.Fatalf("expected description, got %q", got.Description)
	}
	if got.Source != SourceClaude {
		t.Fatalf("expected source=.claude, got %q", got.Source)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "translation" {
		t.Fatalf("unexpected tags: %+v", got.Tags)
	}
	if len(got.Keywords) != 1 || got.Keywords[0] != "i18n" {
		t.Fatalf("unexpected keywords: %+v", got.Keywords)
	}
}

func TestParseSkill_FallbackNameAndDescription(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "code-review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `# Code Review

First paragraph line 1.
Line 2.

Second paragraph.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := parseSkill(path, SourceCodex)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Name != "code-review" {
		t.Fatalf("expected fallback name from dir, got %q", got.Name)
	}
	if !strings.Contains(got.Description, "First paragraph") {
		t.Fatalf("expected description from first paragraph, got %q", got.Description)
	}
	if strings.Contains(got.Description, "\n") {
		t.Fatalf("expected description normalized to single line, got %q", got.Description)
	}
}

func TestDiscover_MultiSourcePrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()

	claude := filepath.Join(home, ".claude", "skills", "same", "SKILL.md")
	codex := filepath.Join(home, ".codex", "skills", "same", "SKILL.md")
	oneagent := filepath.Join(workspace, ".oneagent", "skills", "same", "SKILL.md")

	for _, p := range []string{claude, codex, oneagent} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	if err := os.WriteFile(codex, []byte("---\nname: same\ndescription: from codex\n---\n"), 0o644); err != nil {
		t.Fatalf("write codex: %v", err)
	}
	if err := os.WriteFile(claude, []byte("---\nname: same\ndescription: from claude\n---\n"), 0o644); err != nil {
		t.Fatalf("write claude: %v", err)
	}
	if err := os.WriteFile(oneagent, []byte("---\nname: same\ndescription: from oneagent\n---\n"), 0o644); err != nil {
		t.Fatalf("write oneagent: %v", err)
	}

	cat, err := Discover(context.Background(), DiscoverOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(cat.Skills) != 1 {
		t.Fatalf("expected 1 deduped skill, got %d", len(cat.Skills))
	}
	if cat.Skills[0].Description != "from oneagent" || cat.Skills[0].Source != SourceOneAgent {
		t.Fatalf("expected .oneagent to win, got %+v", cat.Skills[0])
	}
}

func TestScanSkillFiles_FollowsSymlinkAndAvoidsCycles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(filepath.Join(real, "skill-a"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(real, "skill-a", "SKILL.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	skills := filepath.Join(root, "skills")
	if err := os.MkdirAll(skills, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(filepath.Join(real, "skill-a"), filepath.Join(skills, "skill-a")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	// Create a cycle symlink (skills/cycle -> skills).
	_ = os.Symlink(skills, filepath.Join(skills, "cycle"))

	files, err := scanSkillFiles(ctx, skills)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 SKILL.md, got %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "SKILL.md" {
		t.Fatalf("unexpected file: %q", files[0])
	}
}

