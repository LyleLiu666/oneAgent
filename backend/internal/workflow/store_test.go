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

