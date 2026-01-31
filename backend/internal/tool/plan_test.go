package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/scope"
)

func TestPlanTool_InitGetMarkDone_EndToEnd(t *testing.T) {
	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	template := `# PLAN
- [ ] Create file <!-- id: 1 -->
  - scope:
    - backend/**
  - acceptance:
    - files:
      - backend/a.txt
    - must_contain:
      - backend/a.txt: "ok"
`

	rawInit, _ := json.Marshal(map[string]any{
		"action":   "init",
		"template": template,
	})
	if _, err := runPlanTool(ctx, rawInit); err != nil {
		t.Fatalf("plan init: %v", err)
	}

	rawGet, _ := json.Marshal(map[string]any{"action": "get"})
	gotAny, err := runPlanTool(ctx, rawGet)
	if err != nil {
		t.Fatalf("plan get: %v", err)
	}
	got := gotAny.(planToolResult)
	if !got.OK || len(got.Tasks) != 1 || got.Tasks[0].ID != "1" {
		t.Fatalf("unexpected get result: %+v", got)
	}

	rawDone, _ := json.Marshal(map[string]any{
		"action":  "mark_done",
		"task_id": "1",
	})
	doneAny, err := runPlanTool(ctx, rawDone)
	if err != nil {
		t.Fatalf("plan mark_done: %v", err)
	}
	done := doneAny.(planToolResult)
	if done.OK || done.Pass {
		t.Fatalf("expected mark_done to fail when file missing, got %+v", done)
	}

	// Create required file.
	if err := os.MkdirAll(filepath.Join(root, "backend"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "a.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	doneAny, err = runPlanTool(ctx, rawDone)
	if err != nil {
		t.Fatalf("plan mark_done: %v", err)
	}
	done = doneAny.(planToolResult)
	if !done.OK || !done.Pass || !done.Updated {
		t.Fatalf("expected mark_done pass, got %+v", done)
	}
}

func TestPlanTool_AcceptsLegacyActions_StartUpdateComplete(t *testing.T) {
	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	template := `# PLAN
- [ ] Create file <!-- id: 1 -->
  - scope:
    - backend/**
  - acceptance:
    - files:
      - backend/a.txt
    - must_contain:
      - backend/a.txt: "ok"
`

	rawStart, _ := json.Marshal(map[string]any{
		"action":   "start",
		"template": template,
	})
	if _, err := runPlanTool(ctx, rawStart); err != nil {
		t.Fatalf("plan start: %v", err)
	}

	rawUpdate, _ := json.Marshal(map[string]any{"action": "update"})
	gotAny, err := runPlanTool(ctx, rawUpdate)
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	got := gotAny.(planToolResult)
	if !got.OK || len(got.Tasks) != 1 || got.Tasks[0].ID != "1" {
		t.Fatalf("unexpected update result: %+v", got)
	}

	rawComplete, _ := json.Marshal(map[string]any{
		"action":  "complete",
		"task_id": "1",
	})
	doneAny, err := runPlanTool(ctx, rawComplete)
	if err != nil {
		t.Fatalf("plan complete: %v", err)
	}
	done := doneAny.(planToolResult)
	if done.OK || done.Pass {
		t.Fatalf("expected complete to fail when file missing, got %+v", done)
	}

	// Create required file.
	if err := os.MkdirAll(filepath.Join(root, "backend"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "a.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	doneAny, err = runPlanTool(ctx, rawComplete)
	if err != nil {
		t.Fatalf("plan complete: %v", err)
	}
	done = doneAny.(planToolResult)
	if !done.OK || !done.Pass || !done.Updated {
		t.Fatalf("expected complete pass, got %+v", done)
	}
}

func TestPlanScope_RejectsOutOfScopeWriteThenAllowsRetry(t *testing.T) {
	root := t.TempDir()

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{
		Enabled:    true,
		Root:       root,
		WriteScope: []string{"backend/**"},
	})

	_, err := runWriteFileTool(ctx, mustJSON(map[string]any{
		"filePath": "frontend/out.txt",
		"content":  "nope\n",
	}))
	if err == nil {
		t.Fatalf("expected scope error")
	}
	if !errors.Is(err, scope.ErrPathOutsideScope) {
		t.Fatalf("expected ErrPathOutsideScope, got %v", err)
	}

	if _, err := runWriteFileTool(ctx, mustJSON(map[string]any{
		"filePath": "backend/out.txt",
		"content":  "ok\n",
	})); err != nil {
		t.Fatalf("expected write to succeed, got %v", err)
	}
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func TestPlanTool_UpsertTask_ThenGet(t *testing.T) {
	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	// Start from an empty plan.
	if _, err := runPlanTool(ctx, mustJSON(map[string]any{
		"action":    "init",
		"template":  "# PLAN\n",
		"overwrite": true,
	})); err != nil {
		t.Fatalf("plan init: %v", err)
	}

	anyRes, err := runPlanTool(ctx, mustJSON(map[string]any{
		"action":  "upsert_task",
		"task_id": "rust-ebook-reader",
		"title":   "Build rust ebook reader",
		"scope":   []string{"**"},
		"acceptance": map[string]any{
			"files": []string{"Cargo.toml"},
		},
	}))
	if err != nil {
		t.Fatalf("plan upsert_task: %v", err)
	}
	res := anyRes.(planToolResult)
	if !res.OK || !res.Updated || res.TaskID != "rust-ebook-reader" {
		t.Fatalf("unexpected upsert result: %+v", res)
	}

	anyGet, err := runPlanTool(ctx, mustJSON(map[string]any{
		"action": "get",
	}))
	if err != nil {
		t.Fatalf("plan get: %v", err)
	}
	got := anyGet.(planToolResult)
	if !got.OK || len(got.Tasks) != 1 || got.Tasks[0].ID != "rust-ebook-reader" {
		t.Fatalf("unexpected get result: %+v", got)
	}
}

func TestPlanTool_MarkDone_TaskNotFound_ReturnsTasks(t *testing.T) {
	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	template := `# PLAN
- [ ] Task A <!-- id: a -->
  - acceptance:
    - files:
      - a.txt
`

	if _, err := runPlanTool(ctx, mustJSON(map[string]any{
		"action":    "init",
		"template":  template,
		"overwrite": true,
	})); err != nil {
		t.Fatalf("plan init: %v", err)
	}

	anyRes, err := runPlanTool(ctx, mustJSON(map[string]any{
		"action":  "mark_done",
		"task_id": "missing",
	}))
	if err != nil {
		t.Fatalf("plan mark_done: %v", err)
	}
	res := anyRes.(planToolResult)
	if res.OK {
		t.Fatalf("expected ok=false, got %+v", res)
	}
	if len(res.Tasks) != 1 || res.Tasks[0].ID != "a" {
		t.Fatalf("expected tasks returned, got %+v", res)
	}
}
