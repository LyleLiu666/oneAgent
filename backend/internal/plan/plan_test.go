package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_ExtractsTasksWithScopeAndAcceptance(t *testing.T) {
	content := `# PLAN

- [ ] Task one <!-- id: 1 -->
  - scope:
    - backend/**
  - acceptance:
    - files:
      - backend/a.txt
    - must_contain:
      - backend/a.txt: "hello"

- [x] Task two <!-- id: two -->
  - acceptance:
    - files:
      - README.md
`

	p := Parse(content)
	if len(p.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(p.Tasks))
	}

	if p.Tasks[0].ID != "1" || p.Tasks[0].Title != "Task one" || p.Tasks[0].Status != "todo" {
		t.Fatalf("unexpected task1: %+v", p.Tasks[0])
	}
	if len(p.Tasks[0].Scope) != 1 || p.Tasks[0].Scope[0] != "backend/**" {
		t.Fatalf("unexpected task1 scope: %+v", p.Tasks[0].Scope)
	}
	if len(p.Tasks[0].Acceptance.Files) != 1 || p.Tasks[0].Acceptance.Files[0] != "backend/a.txt" {
		t.Fatalf("unexpected task1 files: %+v", p.Tasks[0].Acceptance.Files)
	}
	if len(p.Tasks[0].Acceptance.MustContain) != 1 {
		t.Fatalf("unexpected task1 must_contain: %+v", p.Tasks[0].Acceptance.MustContain)
	}
	if p.Tasks[0].Acceptance.MustContain[0].Path != "backend/a.txt" || p.Tasks[0].Acceptance.MustContain[0].Text != "hello" {
		t.Fatalf("unexpected task1 must_contain item: %+v", p.Tasks[0].Acceptance.MustContain[0])
	}

	if p.Tasks[1].ID != "two" || p.Tasks[1].Title != "Task two" || p.Tasks[1].Status != "done" {
		t.Fatalf("unexpected task2: %+v", p.Tasks[1])
	}
}

func TestSetTaskDone_PreservesFormatting(t *testing.T) {
	content := "- [ ] Task one <!-- id: 1 -->\n  - scope:\n    - backend/**\n"

	updated, changed, err := SetTaskDone(content, "1", true)
	if err != nil {
		t.Fatalf("set done: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if !strings.HasPrefix(updated, "- [x] Task one") {
		t.Fatalf("unexpected updated content: %q", updated)
	}
	if !strings.Contains(updated, "\n  - scope:\n") {
		t.Fatalf("expected scope block preserved, got %q", updated)
	}
}

func TestMarkDone_AtomicOnFailureThenSuccess(t *testing.T) {
	root := t.TempDir()
	planPath := DefaultPlanPath(root)
	if err := os.MkdirAll(filepath.Dir(planPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `# PLAN
- [ ] Create file <!-- id: 1 -->
  - scope:
    - backend/**
  - acceptance:
    - files:
      - backend/a.txt
    - must_contain:
      - backend/a.txt: "ok"
`
	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	// Should fail (file missing) and not flip checkbox.
	res, err := MarkDone(root, "1")
	if err != nil {
		t.Fatalf("mark done: %v", err)
	}
	if res.Pass {
		t.Fatalf("expected pass=false, got %+v", res)
	}
	rawAfterFail, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	if !strings.Contains(string(rawAfterFail), "- [ ] Create file") {
		t.Fatalf("expected checkbox to remain unchecked, got %q", string(rawAfterFail))
	}

	// Satisfy acceptance.
	target := filepath.Join(root, "backend", "a.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("mkdir backend: %v", err)
	}
	if err := os.WriteFile(target, []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write backend/a.txt: %v", err)
	}

	res, err = MarkDone(root, "1")
	if err != nil {
		t.Fatalf("mark done: %v", err)
	}
	if !res.Pass || !res.Updated {
		t.Fatalf("expected pass=true updated=true, got %+v", res)
	}
	rawAfterPass, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	if !strings.Contains(string(rawAfterPass), "- [x] Create file") {
		t.Fatalf("expected checkbox checked, got %q", string(rawAfterPass))
	}
}

