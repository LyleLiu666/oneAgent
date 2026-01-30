package workflow

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeExecutor struct {
	started chan string
	block   map[string]chan struct{}
	mu      sync.Mutex
}

func (e *fakeExecutor) ExecuteNode(ctx context.Context, _ WorkflowRun, node Node) (ArtifactManifest, error) {
	id := node.NodeID
	select {
	case e.started <- id:
	default:
	}
	e.mu.Lock()
	ch := e.block[id]
	e.mu.Unlock()
	if ch != nil {
		select {
		case <-ctx.Done():
			return ArtifactManifest{}, ctx.Err()
		case <-ch:
		}
	}
	return ArtifactManifest{Artifacts: []Artifact{{Path: "out/" + id + ".txt", Kind: "file"}}}, nil
}

func TestRunWorkflow_RespectsDependencies(t *testing.T) {
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
	v, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{
		Nodes: []Node{{NodeID: "a"}, {NodeID: "b"}},
		Edges: []Edge{{From: "a", To: "b"}},
	})
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	run, err := store.CreateRun(ctx, ws, wf.WorkflowID, v.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	releaseA := make(chan struct{})
	exec := &fakeExecutor{
		started: make(chan string, 8),
		block:   map[string]chan struct{}{"a": releaseA},
	}

	done := make(chan WorkflowRun, 1)
	go func() {
		r, _ := store.RunWorkflow(context.Background(), ws, wf.WorkflowID, run.RunID, exec, RunOptions{Concurrency: 2})
		done <- r
	}()

	start1 := <-exec.started
	if start1 != "a" {
		t.Fatalf("expected first started node=a, got %q", start1)
	}

	select {
	case got := <-exec.started:
		t.Fatalf("expected b to not start before a finishes, but got %q", got)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseA)
	final := <-done
	if final.Status != RunStatusSucceeded {
		t.Fatalf("expected succeeded, got %+v", final)
	}
}

func TestRunWorkflow_ConcurrencyLimitSerializes(t *testing.T) {
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
	v, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{
		Nodes: []Node{{NodeID: "a"}, {NodeID: "b"}},
	})
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	run, err := store.CreateRun(ctx, ws, wf.WorkflowID, v.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	exec := &fakeExecutor{
		started: make(chan string, 8),
		block:   map[string]chan struct{}{"a": releaseA, "b": releaseB},
	}

	done := make(chan WorkflowRun, 1)
	go func() {
		r, _ := store.RunWorkflow(context.Background(), ws, wf.WorkflowID, run.RunID, exec, RunOptions{Concurrency: 1})
		done <- r
	}()

	start1 := <-exec.started
	select {
	case got := <-exec.started:
		t.Fatalf("expected second node to not start while first is blocked (concurrency=1), got %q", got)
	case <-time.After(150 * time.Millisecond):
	}

	if start1 == "a" {
		close(releaseA)
	} else if start1 == "b" {
		close(releaseB)
	} else {
		t.Fatalf("unexpected first node id %q", start1)
	}

	start2 := <-exec.started
	if start2 == start1 {
		t.Fatalf("expected a different second node, got %q", start2)
	}
	if start2 == "a" {
		close(releaseA)
	} else if start2 == "b" {
		close(releaseB)
	} else {
		t.Fatalf("unexpected second node id %q", start2)
	}

	final := <-done
	if final.Status != RunStatusSucceeded {
		t.Fatalf("expected succeeded, got %+v", final)
	}
}

func TestRunWorkflow_CanBeCanceled(t *testing.T) {
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
	v, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{
		Nodes: []Node{{NodeID: "a"}},
	})
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	run, err := store.CreateRun(ctx, ws, wf.WorkflowID, v.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	releaseA := make(chan struct{})
	exec := &fakeExecutor{
		started: make(chan string, 8),
		block:   map[string]chan struct{}{"a": releaseA},
	}

	runErr := make(chan error, 1)
	runOut := make(chan WorkflowRun, 1)
	go func() {
		r, err := store.RunWorkflow(context.Background(), ws, wf.WorkflowID, run.RunID, exec, RunOptions{Concurrency: 1})
		runOut <- r
		runErr <- err
	}()

	if got := <-exec.started; got != "a" {
		t.Fatalf("expected a started, got %q", got)
	}

	_, err = store.CancelRun(ctx, ws, wf.WorkflowID, run.RunID, "user canceled")
	if err != nil {
		t.Fatalf("CancelRun: %v", err)
	}

	close(releaseA)
	final := <-runOut
	_ = <-runErr
	if final.Status != RunStatusCanceled {
		t.Fatalf("expected canceled, got %+v", final)
	}

	got, err := store.GetRun(ctx, ws, wf.WorkflowID, run.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != RunStatusCanceled {
		t.Fatalf("expected persisted canceled status, got %+v", got)
	}
	if got.Error == "" {
		t.Fatalf("expected cancel reason to be recorded")
	}
}

func TestRunWorkflow_FailsOnCycle(t *testing.T) {
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
	v, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{
		Nodes: []Node{{NodeID: "a"}, {NodeID: "b"}},
		Edges: []Edge{{From: "a", To: "b"}, {From: "b", To: "a"}},
	})
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	run, err := store.CreateRun(ctx, ws, wf.WorkflowID, v.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	exec := &fakeExecutor{started: make(chan string, 8), block: map[string]chan struct{}{}}
	_, err = store.RunWorkflow(ctx, ws, wf.WorkflowID, run.RunID, exec, RunOptions{Concurrency: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
	got, gerr := store.GetRun(ctx, ws, wf.WorkflowID, run.RunID)
	if gerr != nil {
		t.Fatalf("GetRun: %v", gerr)
	}
	if got.Status != RunStatusFailed {
		t.Fatalf("expected failed status after cycle, got %+v", got)
	}
}

func TestRunWorkflow_ResumeSkipsSucceededNodes(t *testing.T) {
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
	v, err := store.PublishVersion(ctx, ws, wf.WorkflowID, Graph{
		Nodes: []Node{{NodeID: "a"}, {NodeID: "b"}},
		Edges: []Edge{{From: "a", To: "b"}},
	})
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	run, err := store.CreateRun(ctx, ws, wf.WorkflowID, v.VersionID, nil)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Simulate a partial run: A already succeeded, B still queued.
	_, err = store.UpdateRun(ctx, ws, wf.WorkflowID, run.RunID, func(cur *WorkflowRun) error {
		nr := cur.NodeRuns["a"]
		nr.Status = NodeStatusSucceeded
		nr.FinishedAt = time.Now().UTC()
		cur.NodeRuns["a"] = nr
		cur.Status = RunStatusQueued
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}

	exec := &fakeExecutor{
		started: make(chan string, 8),
		block:   map[string]chan struct{}{},
	}

	final, err := store.RunWorkflow(ctx, ws, wf.WorkflowID, run.RunID, exec, RunOptions{Concurrency: 2})
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if final.Status != RunStatusSucceeded {
		t.Fatalf("expected succeeded, got %+v", final)
	}

	// Only B should have executed.
	started := []string{}
	for {
		select {
		case id := <-exec.started:
			started = append(started, id)
		default:
			goto done
		}
	}
done:
	if len(started) != 1 || started[0] != "b" {
		t.Fatalf("expected only b to execute, got %+v", started)
	}
}
