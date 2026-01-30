package skillrecall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/skill"
)

func TestParseExplicitSkill_MatchesSkillNameToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".claude", "skills", "code-review-excellence", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("---\nname: code-review-excellence\ndescription: review\n---\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cat, err := skill.Discover(context.Background(), skill.DiscoverOptions{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	got, ok := ParseExplicitSkill(cat, "请使用 code-review-excellence 这个技能帮我 review")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got.ID != "code-review-excellence" {
		t.Fatalf("expected id, got %+v", got)
	}
}

func TestSearch_UsesRgWhenAvailable(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a", "SKILL.md")
	b := filepath.Join(root, "b", "SKILL.md")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(a, []byte("hello review hello\n"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("no match\n"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	cat := &skill.Catalog{
		Skills: []skill.Skill{
			{ID: "a", Name: "a", Description: "a", Path: a, Source: skill.SourceClaude},
			{ID: "b", Name: "b", Description: "b", Path: b, Source: skill.SourceClaude},
		},
	}

	fakeDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeDir, 0o700); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	fakeRg := writeFakeCountBinary(t, filepath.Join(fakeDir, "rg"))

	lookPath := func(name string) (string, error) {
		if name == "rg" {
			return fakeRg, nil
		}
		return "", errors.New("not found")
	}

	res, err := Search(context.Background(), cat, "review", Options{MaxResults: 8, Timeout: 5 * time.Second}, lookPath)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.Backend != "rg" {
		t.Fatalf("expected backend=rg, got %+v", res)
	}
	if len(res.Candidates) == 0 || res.Candidates[0].Skill.ID != "a" {
		t.Fatalf("expected top candidate a, got %+v", res.Candidates)
	}
	if res.Candidates[0].ContentScore <= 0 {
		t.Fatalf("expected content score > 0, got %+v", res.Candidates[0])
	}
}

func TestSearch_TokenizesMultiWordQuery(t *testing.T) {
	root := t.TempDir()
	apple := filepath.Join(root, "apple-notes", "SKILL.md")
	bear := filepath.Join(root, "bear-notes", "SKILL.md")
	for _, p := range []string{apple, bear} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(apple, []byte("---\nname: apple-notes\ndescription: Manage Apple Notes via memo\n---\nUse memo notes\n"), 0o644); err != nil {
		t.Fatalf("write apple: %v", err)
	}
	if err := os.WriteFile(bear, []byte("---\nname: bear-notes\ndescription: Manage Bear notes\n---\n"), 0o644); err != nil {
		t.Fatalf("write bear: %v", err)
	}

	cat := &skill.Catalog{
		Skills: []skill.Skill{
			{ID: "apple-notes", Name: "apple-notes", Description: "Manage Apple Notes via memo", Path: apple, Source: skill.SourceClaude},
			{ID: "bear-notes", Name: "bear-notes", Description: "Manage Bear notes", Path: bear, Source: skill.SourceClaude},
		},
	}

	fakeDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeDir, 0o700); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	fakeRg := writeFakeCountBinary(t, filepath.Join(fakeDir, "rg"))

	lookPath := func(name string) (string, error) {
		if name == "rg" {
			return fakeRg, nil
		}
		return "", errors.New("not found")
	}

	res, err := Search(context.Background(), cat, "apple notes memo", Options{MaxResults: 8, Timeout: 5 * time.Second}, lookPath)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Candidates) == 0 || res.Candidates[0].Skill.ID != "apple-notes" {
		t.Fatalf("expected top candidate apple-notes, got %+v", res.Candidates)
	}
	if res.Candidates[0].Score <= 0 {
		t.Fatalf("expected score > 0, got %+v", res.Candidates[0])
	}
	if res.Candidates[0].MetadataScore <= 0 && res.Candidates[0].ContentScore <= 0 {
		t.Fatalf("expected metadata or content score > 0, got %+v", res.Candidates[0])
	}
}

func TestSearch_FallsBackToGrepWhenRgMissing(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(a), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(a, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cat := &skill.Catalog{
		Skills: []skill.Skill{
			{ID: "a", Name: "a", Description: "a", Path: a, Source: skill.SourceClaude},
		},
	}

	fakeDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeDir, 0o700); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	fakeGrep := writeFakeCountBinary(t, filepath.Join(fakeDir, "grep"))

	lookPath := func(name string) (string, error) {
		switch name {
		case "rg":
			return "", errors.New("rg missing")
		case "grep":
			return fakeGrep, nil
		default:
			return "", errors.New("not found")
		}
	}

	res, err := Search(context.Background(), cat, "hello", Options{MaxResults: 8, Timeout: 5 * time.Second}, lookPath)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.Backend != "grep" {
		t.Fatalf("expected backend=grep, got %+v", res)
	}
	if res.NotAvailableReason == "" {
		t.Fatalf("expected not_available_reason to mention rg missing")
	}
}

func TestSearch_FallsBackToGoWhenRgAndGrepMissing(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(a), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(a, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cat := &skill.Catalog{
		Skills: []skill.Skill{
			{ID: "a", Name: "a", Description: "a", Path: a, Source: skill.SourceClaude},
		},
	}

	lookPath := func(name string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := Search(context.Background(), cat, "hello", Options{MaxResults: 8, Timeout: 5 * time.Second}, lookPath)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.Backend != "go" {
		t.Fatalf("expected backend=go, got %+v", res)
	}
	if res.NotAvailableReason == "" {
		t.Fatalf("expected not_available_reason to mention rg/grep missing")
	}
	if len(res.Candidates) == 0 || res.Candidates[0].Skill.ID != "a" {
		t.Fatalf("expected top candidate a, got %+v", res.Candidates)
	}
	if res.Candidates[0].ContentScore <= 0 {
		t.Fatalf("expected content score > 0, got %+v", res.Candidates[0])
	}
}

func TestSearch_DoesNotReturnArchivedOneAgentSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillPath := filepath.Join(home, ".oneagent", "skills", "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("---\nname: demo-skill\ndescription: demo\n---\nhello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Archive (move) the skill directory, matching runtime behavior.
	srcDir := filepath.Dir(skillPath)
	dstDir := filepath.Join(home, ".oneagent", "skills-archived", "demo-skill")
	if err := os.MkdirAll(filepath.Dir(dstDir), 0o700); err != nil {
		t.Fatalf("mkdir archived: %v", err)
	}
	if err := os.Rename(srcDir, dstDir); err != nil {
		t.Fatalf("rename: %v", err)
	}

	cat, err := skill.Discover(context.Background(), skill.DiscoverOptions{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if _, ok := cat.ByID("demo-skill"); ok {
		t.Fatalf("expected archived skill to be absent from catalog")
	}

	lookPath := func(name string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := Search(context.Background(), cat, "demo-skill", Options{MaxResults: 8, Timeout: 2 * time.Second}, lookPath)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, cand := range res.Candidates {
		if cand.Skill.ID == "demo-skill" {
			t.Fatalf("expected demo-skill to be excluded from recall candidates")
		}
	}
}

func TestSearch_WithManySkills_ReturnsTop8(t *testing.T) {
	skills := make([]skill.Skill, 0, 1200)
	for i := 0; i < 1200; i++ {
		id := fmt.Sprintf("skill-%04d", i)
		skills = append(skills, skill.Skill{
			ID:          id,
			Name:        id,
			Description: "desc",
			Path:        "/tmp/" + id + "/SKILL.md",
			Source:      skill.SourceClaude,
		})
	}

	cat := &skill.Catalog{Skills: skills}
	lookPath := func(name string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := Search(context.Background(), cat, "please use skill-0999", Options{MaxResults: 8, Timeout: 200 * time.Millisecond}, lookPath)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Candidates) != 8 {
		t.Fatalf("expected 8 candidates, got %d", len(res.Candidates))
	}
	if res.Candidates[0].Skill.ID != "skill-0999" {
		t.Fatalf("expected top=skill-0999, got %q", res.Candidates[0].Skill.ID)
	}
}

func TestSearch_FindsBuiltinSkillWithoutRgOrGrep(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cat, err := skill.Discover(context.Background(), skill.DiscoverOptions{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	lookPath := func(name string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := Search(context.Background(), cat, "create-skill", Options{MaxResults: 8, Timeout: 200 * time.Millisecond}, lookPath)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.Backend != "none" {
		t.Fatalf("expected backend=none, got %+v", res)
	}
	if len(res.Candidates) == 0 || res.Candidates[0].Skill.ID != "create-skill" {
		t.Fatalf("expected top candidate create-skill, got %+v", res.Candidates)
	}
	if res.Candidates[0].ContentScore <= 0 {
		t.Fatalf("expected content score > 0, got %+v", res.Candidates[0])
	}
}

func writeFakeCountBinary(t *testing.T, path string) string {
	t.Helper()

	script := `#!/bin/sh
patterns=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--" ]; then
    shift
    break
  fi
  if [ "$1" = "-e" ]; then
    shift
    if [ $# -gt 0 ]; then
      patterns="$patterns $1"
      shift
    fi
    continue
  fi
  shift
done

found=0
for f in "$@"; do
  if [ ! -f "$f" ]; then
    continue
  fi
  total=0
  for p in $patterns; do
    c=$(grep -oiF "$p" "$f" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$c" -gt 0 ]; then
      total=$((total + c))
    fi
  done
  if [ "$total" -gt 0 ]; then
    echo "$f:$total"
    found=1
  fi
done

if [ "$found" -eq 1 ]; then
  exit 0
fi
exit 1
`

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}
