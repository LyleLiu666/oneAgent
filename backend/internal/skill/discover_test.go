package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
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
tool_ids:
  - RG
  - read_file
  - skill.read
  - rg
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
	if len(got.ToolIDs) != 3 || got.ToolIDs[0] != "rg" || got.ToolIDs[1] != "read_file" || got.ToolIDs[2] != "skill.read" {
		t.Fatalf("unexpected tool_ids: %+v", got.ToolIDs)
	}
}

func TestParseSkill_RequiresAndInstall(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "apple-notes", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `---
name: apple-notes
description: Manage Apple Notes via memo
requires:
  os: darwin
  bins:
    - memo
  any_bins: [rg, grep]
  env: OPENAI_API_KEY
install:
  - kind: brew
    formula: antoniorodr/memo/memo
    bins: memo
---
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := parseSkill(path, SourceClaude)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.Requires == nil {
		t.Fatalf("expected requires to be parsed")
	}
	if len(got.Requires.OS) != 1 || got.Requires.OS[0] != "darwin" {
		t.Fatalf("unexpected requires.os: %+v", got.Requires.OS)
	}
	if len(got.Requires.Bins) != 1 || got.Requires.Bins[0] != "memo" {
		t.Fatalf("unexpected requires.bins: %+v", got.Requires.Bins)
	}
	if len(got.Requires.AnyBins) != 2 || got.Requires.AnyBins[0] != "rg" || got.Requires.AnyBins[1] != "grep" {
		t.Fatalf("unexpected requires.any_bins: %+v", got.Requires.AnyBins)
	}
	if len(got.Requires.Env) != 1 || got.Requires.Env[0] != "OPENAI_API_KEY" {
		t.Fatalf("unexpected requires.env: %+v", got.Requires.Env)
	}

	if len(got.Install) != 1 {
		t.Fatalf("expected 1 install spec, got %+v", got.Install)
	}
	if got.Install[0].Kind != "brew" || got.Install[0].Formula != "antoniorodr/memo/memo" {
		t.Fatalf("unexpected install spec: %+v", got.Install[0])
	}
	if len(got.Install[0].Bins) != 1 || got.Install[0].Bins[0] != "memo" {
		t.Fatalf("unexpected install bins: %+v", got.Install[0].Bins)
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
	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	home := t.TempDir()
	t.Setenv("HOME", home)

	oneagentHome := t.TempDir()
	t.Setenv("ONEAGENT_HOME", oneagentHome)
	config.AppConfig = &config.Config{Home: oneagentHome}

	workspace := t.TempDir()

	claude := filepath.Join(home, ".claude", "skills", "same", "SKILL.md")
	codex := filepath.Join(home, ".codex", "skills", "same", "SKILL.md")
	wsClaude := filepath.Join(workspace, ".claude", "skills", "same", "SKILL.md")
	wsSkills := filepath.Join(workspace, "skills", "same", "SKILL.md")
	oneagent := filepath.Join(workspace, ".oneagent", "skills", "same", "SKILL.md")
	homeOneAgent := filepath.Join(oneagentHome, ".oneagent", "skills", "same", "SKILL.md")

	for _, p := range []string{claude, codex, wsClaude, wsSkills, oneagent, homeOneAgent} {
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
	if err := os.WriteFile(wsClaude, []byte("---\nname: same\ndescription: from workspace claude\n---\n"), 0o644); err != nil {
		t.Fatalf("write workspace claude: %v", err)
	}
	if err := os.WriteFile(wsSkills, []byte("---\nname: same\ndescription: from workspace skills\n---\n"), 0o644); err != nil {
		t.Fatalf("write workspace skills: %v", err)
	}
	if err := os.WriteFile(oneagent, []byte("---\nname: same\ndescription: from oneagent\n---\n"), 0o644); err != nil {
		t.Fatalf("write oneagent: %v", err)
	}
	if err := os.WriteFile(homeOneAgent, []byte("---\nname: same\ndescription: from oneagent home\n---\n"), 0o644); err != nil {
		t.Fatalf("write home oneagent: %v", err)
	}

	cat, err := Discover(context.Background(), DiscoverOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	got, ok := cat.ByID("same")
	if !ok {
		t.Fatalf("expected skill to be discoverable by id")
	}
	if got.Description != "from oneagent" || got.Source != SourceOneAgent {
		t.Fatalf("expected .oneagent to win, got %+v", got)
	}
}

func TestDiscover_WorkspaceSkillsPrecedence(t *testing.T) {
	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	home := t.TempDir()
	t.Setenv("HOME", home)

	oneagentHome := t.TempDir()
	t.Setenv("ONEAGENT_HOME", oneagentHome)
	config.AppConfig = &config.Config{Home: oneagentHome}

	workspace := t.TempDir()

	homeClaude := filepath.Join(home, ".claude", "skills", "same", "SKILL.md")
	wsClaude := filepath.Join(workspace, ".claude", "skills", "same", "SKILL.md")
	wsSkills := filepath.Join(workspace, "skills", "same", "SKILL.md")
	homeOneAgent := filepath.Join(oneagentHome, ".oneagent", "skills", "same", "SKILL.md")

	for _, p := range []string{homeClaude, wsClaude, wsSkills, homeOneAgent} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	if err := os.WriteFile(homeClaude, []byte("---\nname: same\ndescription: from home claude\n---\n"), 0o644); err != nil {
		t.Fatalf("write home claude: %v", err)
	}
	if err := os.WriteFile(wsClaude, []byte("---\nname: same\ndescription: from workspace claude\n---\n"), 0o644); err != nil {
		t.Fatalf("write workspace claude: %v", err)
	}
	if err := os.WriteFile(wsSkills, []byte("---\nname: same\ndescription: from workspace skills\n---\n"), 0o644); err != nil {
		t.Fatalf("write workspace skills: %v", err)
	}
	if err := os.WriteFile(homeOneAgent, []byte("---\nname: same\ndescription: from oneagent home\n---\n"), 0o644); err != nil {
		t.Fatalf("write home oneagent: %v", err)
	}

	cat, err := Discover(context.Background(), DiscoverOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	got, ok := cat.ByID("same")
	if !ok {
		t.Fatalf("expected skill to be discoverable by id")
	}
	if got.Description != "from workspace skills" || got.Source != SourceWorkspace {
		t.Fatalf("expected workspace skills to win, got %+v", got)
	}
}

func TestDiscover_OneAgentHomePrecedence(t *testing.T) {
	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	home := t.TempDir()
	t.Setenv("HOME", home)

	oneagentHome := t.TempDir()
	t.Setenv("ONEAGENT_HOME", oneagentHome)
	config.AppConfig = &config.Config{Home: oneagentHome}

	homeClaude := filepath.Join(home, ".claude", "skills", "same", "SKILL.md")
	homeCodex := filepath.Join(home, ".codex", "skills", "same", "SKILL.md")
	homeOneAgent := filepath.Join(oneagentHome, ".oneagent", "skills", "same", "SKILL.md")

	for _, p := range []string{homeClaude, homeCodex, homeOneAgent} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	if err := os.WriteFile(homeCodex, []byte("---\nname: same\ndescription: from home codex\n---\n"), 0o644); err != nil {
		t.Fatalf("write home codex: %v", err)
	}
	if err := os.WriteFile(homeClaude, []byte("---\nname: same\ndescription: from home claude\n---\n"), 0o644); err != nil {
		t.Fatalf("write home claude: %v", err)
	}
	if err := os.WriteFile(homeOneAgent, []byte("---\nname: same\ndescription: from oneagent home\n---\n"), 0o644); err != nil {
		t.Fatalf("write home oneagent: %v", err)
	}

	cat, err := Discover(context.Background(), DiscoverOptions{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	got, ok := cat.ByID("same")
	if !ok {
		t.Fatalf("expected skill to be discoverable by id")
	}
	if got.Description != "from oneagent home" || got.Source != SourceOneAgent {
		t.Fatalf("expected ONEAGENT_HOME to win over ~/.claude/.codex, got %+v", got)
	}
}

func TestDiscover_IncludesBuiltinSkills(t *testing.T) {
	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	home := t.TempDir()
	t.Setenv("HOME", home)
	oneagentHome := t.TempDir()
	t.Setenv("ONEAGENT_HOME", oneagentHome)
	config.AppConfig = &config.Config{Home: oneagentHome}

	cat, err := Discover(context.Background(), DiscoverOptions{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	got, ok := cat.ByID("create-skill")
	if !ok {
		t.Fatalf("expected built-in create-skill to be discovered")
	}
	if got.Source != SourceBuiltin {
		t.Fatalf("expected source=.builtin, got %+v", got)
	}
	if got.Path == "" {
		t.Fatalf("expected builtin path to be set")
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
