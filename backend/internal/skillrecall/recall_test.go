package skillrecall

import (
	"context"
	"errors"
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

	res, err := Search(context.Background(), cat, "review", Options{MaxResults: 8, Timeout: 2 * time.Second}, lookPath)
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

	res, err := Search(context.Background(), cat, "hello", Options{MaxResults: 8, Timeout: 2 * time.Second}, lookPath)
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

func writeFakeCountBinary(t *testing.T, path string) string {
	t.Helper()

	script := `#!/bin/sh
pattern=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--" ]; then
    shift
    pattern="$1"
    shift
    break
  fi
  shift
done

found=0
for f in "$@"; do
  if [ ! -f "$f" ]; then
    continue
  fi
  c=$(grep -oiF "$pattern" "$f" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$c" -gt 0 ]; then
    echo "$f:$c"
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

