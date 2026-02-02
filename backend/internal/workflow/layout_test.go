package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_NodeRoot_IsDeterministic(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "workflows"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := filepath.Join(t.TempDir(), "project-a")
	workflowID := "wf-1"
	runID := "run-1"
	nodeID := "node-a"

	got1 := store.NodeRoot(ws, workflowID, runID, nodeID)
	got2 := store.NodeRoot(ws, workflowID, runID, nodeID)
	if got1 != got2 {
		t.Fatalf("expected deterministic node root, got %q != %q", got1, got2)
	}

	expectPrefix := filepath.Join(store.Root(), workspaceKey(ws), "workflows", workflowID, "runs", runID, "nodes", nodeID)
	if got1 != expectPrefix {
		t.Fatalf("unexpected node root: want %q got %q", expectPrefix, got1)
	}
}

func TestStore_CleanupOldRuns_RemovesFinishedRuns(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "workflows"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	ws := filepath.Join(t.TempDir(), "ws")
	ctx := context.Background()

	wf, err := store.CreateWorkflow(ctx, ws, "Demo")
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}
	v, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{Nodes: []Node{{NodeID: "a"}}})
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	run, err := store.CreateRun(ctx, ws, wf.WorkflowID, v.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	now := time.Now().UTC()
	oldFinished := now.Add(-35 * 24 * time.Hour)
	_, err = store.UpdateRun(ctx, ws, wf.WorkflowID, run.RunID, func(cur *WorkflowRun) error {
		cur.Status = RunStatusSucceeded
		cur.FinishedAt = oldFinished
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}

	if err := store.CleanupOldRuns(ctx, 30, now); err != nil {
		t.Fatalf("CleanupOldRuns: %v", err)
	}

	if _, err := os.Stat(store.runJSONPath(ws, wf.WorkflowID, run.RunID)); err == nil {
		t.Fatalf("expected old run to be removed")
	}
}
