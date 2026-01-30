package workflow

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_WorkflowCreateListAndPublishVersion(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "workflows"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := filepath.Join(t.TempDir(), "project-a")
	ctx := context.Background()

	wf, err := store.CreateWorkflow(ctx, ws, "Demo")
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}
	if wf.WorkflowID == "" || wf.WorkspaceRoot == "" || wf.Name != "Demo" {
		t.Fatalf("unexpected workflow: %+v", wf)
	}

	list, err := store.ListWorkflows(ctx, ws)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(list) != 1 || list[0].WorkflowID != wf.WorkflowID {
		t.Fatalf("expected workflow in list, got %+v", list)
	}

	graph := Graph{
		Nodes: []Node{{NodeID: "a", Title: "A"}, {NodeID: "b", Title: "B"}},
		Edges: []Edge{{From: "a", To: "b"}},
	}

	v, err := store.PublishVersion(ctx, ws, wf.WorkflowID, graph)
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	if v.VersionID == "" || v.WorkflowID != wf.WorkflowID || !v.Published {
		t.Fatalf("unexpected version: %+v", v)
	}

	got, err := store.GetVersion(ctx, ws, wf.WorkflowID, v.VersionID)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.VersionID != v.VersionID || got.WorkflowID != wf.WorkflowID {
		t.Fatalf("unexpected get: %+v", got)
	}
	if len(got.Graph.Nodes) != 2 || got.Graph.Edges[0].From != "a" {
		t.Fatalf("unexpected graph: %+v", got.Graph)
	}
}

func TestStore_RunKeepsVersionSnapshot(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "workflows"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := filepath.Join(t.TempDir(), "project-a")
	ctx := context.Background()

	wf, err := store.CreateWorkflow(ctx, ws, "Demo")
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	v1, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{
		Nodes: []Node{{NodeID: "a", Title: "A"}, {NodeID: "b", Title: "B"}},
		Edges: []Edge{{From: "a", To: "b"}},
	})
	if err != nil {
		t.Fatalf("PublishVersion(v1): %v", err)
	}

	v2, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{
		Nodes: []Node{{NodeID: "x", Title: "X"}},
		Edges: nil,
	})
	if err != nil {
		t.Fatalf("PublishVersion(v2): %v", err)
	}

	run1, err := store.CreateRun(ctx, ws, wf.WorkflowID, v1.VersionID, map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("CreateRun(v1): %v", err)
	}
	if run1.VersionID != v1.VersionID {
		t.Fatalf("expected version_id=%q, got %+v", v1.VersionID, run1)
	}
	if len(run1.GraphSnapshot.Nodes) != len(v1.Graph.Nodes) {
		t.Fatalf("expected snapshot nodes=%d, got %+v", len(v1.Graph.Nodes), run1.GraphSnapshot)
	}
	if len(run1.NodeRuns) != 2 {
		t.Fatalf("expected 2 node_runs queued, got %+v", run1.NodeRuns)
	}

	run2, err := store.CreateRun(ctx, ws, wf.WorkflowID, v2.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun(v2): %v", err)
	}
	if len(run2.GraphSnapshot.Nodes) != 1 || run2.GraphSnapshot.Nodes[0].NodeID != "x" {
		t.Fatalf("unexpected v2 snapshot: %+v", run2.GraphSnapshot)
	}

	got, err := store.GetRun(ctx, ws, wf.WorkflowID, run1.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.VersionID != v1.VersionID || got.GraphSnapshot.Nodes[0].NodeID != "a" {
		t.Fatalf("unexpected got: %+v", got)
	}
}

func TestStore_RenameAndDeleteWorkflow(t *testing.T) {
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

	renamed, err := store.RenameWorkflow(ctx, ws, wf.WorkflowID, "Demo2")
	if err != nil {
		t.Fatalf("RenameWorkflow: %v", err)
	}
	if renamed.Name != "Demo2" {
		t.Fatalf("expected Demo2, got %+v", renamed)
	}

	if err := store.DeleteWorkflow(ctx, ws, wf.WorkflowID); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}
	list, err := store.ListWorkflows(ctx, ws)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete, got %+v", list)
	}
}
