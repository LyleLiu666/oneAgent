package taskqueue

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskRunner_Governance_MaxRunningWorkspaces(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "tasks"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Global.MaxRunningWorkspaces = 2
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	ws1 := filepath.Join(base, "ws1")
	ws2 := filepath.Join(base, "ws2")
	ws3 := filepath.Join(base, "ws3")
	mustMkdir(t, ws1)
	mustMkdir(t, ws2)
	mustMkdir(t, ws3)

	t1, err := store.CreateTask("local", ws1, "t1", "do 1", "", ResolveLimits(Limits{}))
	if err != nil {
		t.Fatalf("CreateTask(t1): %v", err)
	}
	t2, err := store.CreateTask("local", ws2, "t2", "do 2", "", ResolveLimits(Limits{}))
	if err != nil {
		t.Fatalf("CreateTask(t2): %v", err)
	}
	t3, err := store.CreateTask("local", ws3, "t3", "do 3", "", ResolveLimits(Limits{}))
	if err != nil {
		t.Fatalf("CreateTask(t3): %v", err)
	}

	started := make(chan string, 8)
	release := make(chan struct{})

	r := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task Task, attempt Attempt, resumedFrom *Attempt) (AttemptResult, error) {
			started <- task.ID
			findings := filepath.Join(base, "findings-"+task.ID+".md")
			trace := filepath.Join(base, "trace-"+task.ID+".jsonl")
			mustWrite(t, findings, "ok")
			mustWrite(t, trace, "{}\n")
			select {
			case <-release:
			case <-ctx.Done():
				return AttemptResult{}, ctx.Err()
			}
			return AttemptResult{Summary: "ok", FindingsPath: findings, TraceLogPath: trace}, nil
		},
	}
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(r.Stop)

	waitForN(t, started, 2, 2*time.Second)
	select {
	case <-started:
		t.Fatalf("expected workspace cap to prevent third start")
	case <-time.After(500 * time.Millisecond):
	}

	close(release)
	waitForN(t, started, 1, 2*time.Second)

	_ = t1
	_ = t2
	_ = t3
}

func TestTaskRunner_Governance_PausedWorkspace(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "tasks"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := filepath.Join(base, "ws")
	mustMkdir(t, ws)

	task, err := store.CreateTask("local", ws, "t", "do", "", ResolveLimits(Limits{}))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Workspaces[task.Workspace] = WorkspacePolicy{Paused: true, Priority: 0}
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	started := make(chan string, 8)
	r := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task Task, attempt Attempt, resumedFrom *Attempt) (AttemptResult, error) {
			started <- task.ID
			findings := filepath.Join(base, "findings.md")
			trace := filepath.Join(base, "trace.jsonl")
			mustWrite(t, findings, "ok")
			mustWrite(t, trace, "{}\n")
			return AttemptResult{Summary: "ok", FindingsPath: findings, TraceLogPath: trace}, nil
		},
	}
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(r.Stop)

	// Should not start while paused.
	select {
	case <-started:
		t.Fatalf("unexpected start while paused")
	case <-time.After(600 * time.Millisecond):
	}

	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		p := g.Workspaces[task.Workspace]
		p.Paused = false
		g.Workspaces[task.Workspace] = p
		return nil
	})
	if err != nil {
		t.Fatalf("unpause: %v", err)
	}

	waitForN(t, started, 1, 2*time.Second)
}

func TestTaskRunner_Governance_ScheduleTriggersEnqueue(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "tasks"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := filepath.Join(base, "ws")
	mustMkdir(t, ws)

	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Schedules = []Schedule{{
			ID:           "s1",
			Enabled:      true,
			UserID:       "local",
			Workspace:    ws,
			Title:        "scheduled",
			Prompt:       "run scheduled",
			EverySeconds: 1,
			NextRunAt:    time.Time{}, // run ASAP
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}}
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance(schedule): %v", err)
	}

	done := make(chan struct{}, 8)
	r := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task Task, attempt Attempt, resumedFrom *Attempt) (AttemptResult, error) {
			findings := filepath.Join(base, "findings-"+task.ID+".md")
			trace := filepath.Join(base, "trace-"+task.ID+".jsonl")
			mustWrite(t, findings, "ok")
			mustWrite(t, trace, "{}\n")
			done <- struct{}{}
			return AttemptResult{Summary: "ok", FindingsPath: findings, TraceLogPath: trace}, nil
		},
	}
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(r.Stop)

	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatalf("expected schedule to run")
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func waitForN(t *testing.T, ch <-chan string, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	got := 0
	for got < n {
		select {
		case <-deadline.C:
			t.Fatalf("timeout waiting for %d items, got %d", n, got)
		case <-ch:
			got++
		}
	}
}
