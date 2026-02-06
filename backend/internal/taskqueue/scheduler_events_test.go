package taskqueue

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskRunner_Governance_EmitsPickedEvent(t *testing.T) {
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

	started := make(chan string, 8)
	release := make(chan struct{})
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

	waitForN(t, started, 1, 2*time.Second)

	evs, err := store.ReadEvents(task.ID)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	found := false
	for _, ev := range evs {
		if ev.Type != "scheduler.picked" {
			continue
		}
		found = true
		if ev.Data == nil || ev.Data["reason_code"] != ScheduleReasonPicked {
			t.Fatalf("expected scheduler.picked reason_code=%q, got %+v", ScheduleReasonPicked, ev.Data)
		}
		break
	}
	if !found {
		t.Fatalf("expected scheduler.picked event, got %d events", len(evs))
	}

	close(release)
}

func TestTaskRunner_Governance_EmitsDeferredEventForPausedWorkspace(t *testing.T) {
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

	r := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task Task, attempt Attempt, resumedFrom *Attempt) (AttemptResult, error) {
			t.Fatalf("ExecuteAttempt unexpectedly invoked for paused workspace")
			return AttemptResult{}, nil
		},
	}
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(r.Stop)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		evs, err := store.ReadEvents(task.ID)
		if err != nil {
			t.Fatalf("ReadEvents: %v", err)
		}
		for _, ev := range evs {
			if ev.Type != "scheduler.deferred" {
				continue
			}
			if ev.Data == nil || ev.Data["reason_code"] != ScheduleReasonWorkspacePaused {
				t.Fatalf("expected scheduler.deferred reason_code=%q, got %+v", ScheduleReasonWorkspacePaused, ev.Data)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("expected scheduler.deferred event before timeout")
}
