package taskqueue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/usage"
)

type staticObserver struct {
	decisions map[string]ObserverDecision
}

func (o *staticObserver) Decide(ctx context.Context, in ObserveInput) (ObserverDecision, error) {
	if o == nil {
		return ObserverDecision{Pass: true, Reason: "ok"}, nil
	}
	if o.decisions != nil {
		if d, ok := o.decisions[in.TaskID]; ok {
			return d, nil
		}
	}
	return ObserverDecision{Pass: true, Reason: "ok", Evidence: []string{"FINDINGS.md"}}, nil
}

type execStub struct {
	t *testing.T

	artifactsDir string

	startedCh chan string

	mu     sync.Mutex
	blocks map[string]chan struct{}

	failIfNoResume bool
}

func (e *execStub) Execute(ctx context.Context, task Task, attempt Attempt, resumedFrom *Attempt) (AttemptResult, error) {
	if e.startedCh != nil {
		select {
		case e.startedCh <- task.ID:
		default:
		}
	}

	var block chan struct{}
	e.mu.Lock()
	if e.blocks != nil {
		block = e.blocks[task.ID]
	}
	e.mu.Unlock()

	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return AttemptResult{}, ctx.Err()
		}
	}

	res := e.writeArtifacts(task.ID, attempt.ID)
	if e.failIfNoResume && resumedFrom == nil {
		return res, errors.New("simulated failure")
	}
	return res, nil
}

func (e *execStub) writeArtifacts(taskID, attemptID string) AttemptResult {
	dir := filepath.Join(e.artifactsDir, taskID, attemptID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		e.t.Fatalf("mkdir artifacts: %v", err)
	}
	findings := filepath.Join(dir, "FINDINGS.md")
	trace := filepath.Join(dir, "trace.jsonl")
	if err := os.WriteFile(findings, []byte("# Findings\n- ok\n"), 0o600); err != nil {
		e.t.Fatalf("write findings: %v", err)
	}
	if err := os.WriteFile(trace, []byte("{\"type\":\"complete\"}\n"), 0o600); err != nil {
		e.t.Fatalf("write trace: %v", err)
	}
	return AttemptResult{
		RunID:        "run-" + attemptID,
		Summary:      "done",
		FindingsPath: findings,
		TraceLogPath: trace,
	}
}

func TestTaskRunner_FIFOPerWorkspace(t *testing.T) {
	origNow := Now
	origNewID := NewID
	t.Cleanup(func() {
		Now = origNow
		NewID = origNewID
	})

	base := time.Date(2026, 1, 27, 10, 0, 0, 0, time.UTC)
	var tick int64
	Now = func() time.Time {
		out := base.Add(time.Duration(tick) * time.Second)
		tick++
		return out
	}

	ids := []string{"task-a", "attempt-a1", "task-b", "attempt-b1"}
	NewID = func() string {
		next := ids[0]
		ids = ids[1:]
		return next
	}

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	workspace := t.TempDir()
	taskA, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask A: %v", err)
	}
	taskB, err := store.CreateTask("local", workspace, "B", "task B", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask B: %v", err)
	}

	startedCh := make(chan string, 10)
	blockA := make(chan struct{})
	exec := &execStub{
		t:            t,
		artifactsDir: t.TempDir(),
		startedCh:    startedCh,
		blocks: map[string]chan struct{}{
			taskA.ID: blockA,
		},
	}

	runner := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true, Reason: "ok", Evidence: []string{"FINDINGS.md"}}, nil
		},
		ExecuteAttempt: exec.Execute,
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	if err := runner.Enqueue(taskA.ID); err != nil {
		t.Fatalf("Enqueue A: %v", err)
	}
	if err := runner.Enqueue(taskB.ID); err != nil {
		t.Fatalf("Enqueue B: %v", err)
	}

	select {
	case got := <-startedCh:
		if got != taskA.ID {
			t.Fatalf("expected task A to start first, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for task A to start")
	}

	// While A is blocked running, B must stay queued.
	time.Sleep(100 * time.Millisecond)
	bAfter, err := store.GetTask(taskB.ID)
	if err != nil {
		t.Fatalf("GetTask B: %v", err)
	}
	if bAfter.LatestAttempt() == nil || bAfter.LatestAttempt().Status != AttemptQueued {
		t.Fatalf("expected B queued while A running, got %+v", bAfter.LatestAttempt())
	}

	close(blockA)

	select {
	case got := <-startedCh:
		if got != taskB.ID {
			t.Fatalf("expected task B to start after A, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for task B to start")
	}

	waitForStatus(t, store, taskA.ID, AttemptSucceeded, 2*time.Second)
	waitForStatus(t, store, taskB.ID, AttemptSucceeded, 2*time.Second)
}

func TestTaskRunner_ParallelAcrossWorkspaces(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	taskA, err := store.CreateTask("local", workspaceA, "A", "task A", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask A: %v", err)
	}
	taskB, err := store.CreateTask("local", workspaceB, "B", "task B", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask B: %v", err)
	}

	startedCh := make(chan string, 10)
	blockA := make(chan struct{})
	blockB := make(chan struct{})
	exec := &execStub{
		t:            t,
		artifactsDir: t.TempDir(),
		startedCh:    startedCh,
		blocks: map[string]chan struct{}{
			taskA.ID: blockA,
			taskB.ID: blockB,
		},
	}

	runner := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
		ExecuteAttempt: exec.Execute,
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	if err := runner.Enqueue(taskA.ID); err != nil {
		t.Fatalf("Enqueue A: %v", err)
	}
	if err := runner.Enqueue(taskB.ID); err != nil {
		t.Fatalf("Enqueue B: %v", err)
	}

	seen := make(map[string]bool)
	deadline := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case id := <-startedCh:
			seen[id] = true
		case <-deadline:
			t.Fatalf("timeout waiting for both tasks to start; seen=%v", seen)
		}
	}

	close(blockA)
	close(blockB)

	waitForStatus(t, store, taskA.ID, AttemptSucceeded, 2*time.Second)
	waitForStatus(t, store, taskB.ID, AttemptSucceeded, 2*time.Second)
}

func TestTaskRunner_CancelQueued(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	workspace := t.TempDir()
	taskA, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask A: %v", err)
	}
	taskB, err := store.CreateTask("local", workspace, "B", "task B", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask B: %v", err)
	}

	startedCh := make(chan string, 10)
	blockA := make(chan struct{})
	exec := &execStub{
		t:            t,
		artifactsDir: t.TempDir(),
		startedCh:    startedCh,
		blocks: map[string]chan struct{}{
			taskA.ID: blockA,
		},
	}

	runner := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
		ExecuteAttempt: exec.Execute,
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	_ = runner.Enqueue(taskA.ID)
	_ = runner.Enqueue(taskB.ID)

	select {
	case <-startedCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for task A to start")
	}

	canceled, err := runner.Cancel(taskB.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if canceled.LatestAttempt() == nil || canceled.LatestAttempt().Status != AttemptCanceled {
		t.Fatalf("expected canceled status, got %+v", canceled.LatestAttempt())
	}

	close(blockA)
	waitForStatus(t, store, taskA.ID, AttemptSucceeded, 2*time.Second)

	select {
	case got := <-startedCh:
		if got == taskB.ID {
			t.Fatalf("unexpected: canceled task B started")
		}
	default:
	}
}

func TestTaskRunner_CancelRunning(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	workspace := t.TempDir()
	taskA, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	startedCh := make(chan string, 10)
	blockA := make(chan struct{})
	exec := &execStub{
		t:            t,
		artifactsDir: t.TempDir(),
		startedCh:    startedCh,
		blocks: map[string]chan struct{}{
			taskA.ID: blockA,
		},
	}

	runner := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
		ExecuteAttempt: exec.Execute,
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	_ = runner.Enqueue(taskA.ID)
	select {
	case <-startedCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for task to start")
	}

	if _, err := runner.Cancel(taskA.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	waitForStatus(t, store, taskA.ID, AttemptCanceled, 2*time.Second)
}

func TestTaskRunner_ResumeCreatesNewAttempt(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	workspace := t.TempDir()
	task, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	exec := &execStub{
		t:             t,
		artifactsDir:  t.TempDir(),
		failIfNoResume: true,
	}

	runner := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
		ExecuteAttempt: exec.Execute,
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	_ = runner.Enqueue(task.ID)
	waitForStatus(t, store, task.ID, AttemptFailed, 2*time.Second)

	updated, err := runner.Resume(task.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(updated.Attempts) != 2 {
		t.Fatalf("expected 2 attempts after resume, got %d", len(updated.Attempts))
	}
	if updated.Attempts[1].ResumedFromAttemptID != updated.Attempts[0].ID {
		t.Fatalf("expected resumed_from=%q, got %q", updated.Attempts[0].ID, updated.Attempts[1].ResumedFromAttemptID)
	}

	waitForStatus(t, store, task.ID, AttemptSucceeded, 2*time.Second)
}

func TestTaskRunner_BudgetExceeded_IsTerminalAndResumable(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	workspace := t.TempDir()
	task, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	exec := &execStub{
		t:           t,
		artifactsDir: t.TempDir(),
	}

	runner := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task Task, attempt Attempt, resumedFrom *Attempt) (AttemptResult, error) {
			res := exec.writeArtifacts(task.ID, attempt.ID)
			if resumedFrom == nil {
				return res, &usage.BudgetExceededError{
					MaxTotalTokens: 10,
					Used:           usage.Totals{TotalTokens: 11},
				}
			}
			return res, nil
		},
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	_ = runner.Enqueue(task.ID)
	waitForStatus(t, store, task.ID, AttemptLimitExceeded, 2*time.Second)

	updated, err := runner.Resume(task.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(updated.Attempts) != 2 {
		t.Fatalf("expected 2 attempts after resume, got %d", len(updated.Attempts))
	}
	waitForStatus(t, store, task.ID, AttemptSucceeded, 2*time.Second)
}

func TestTaskRunner_Start_MarksRunningAsInterrupted(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	workspace := t.TempDir()
	task, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Simulate crash: mark attempt as running on disk.
	now := Now()
	_, err = store.UpdateTask(task.ID, func(tk *Task) error {
		a := tk.LatestAttempt()
		if a == nil {
			return errors.New("missing attempt")
		}
		a.Status = AttemptRunning
		a.StartedAt = &now
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	startedCh := make(chan string, 1)
	exec := &execStub{
		t:            t,
		artifactsDir: t.TempDir(),
		startedCh:    startedCh,
	}

	runner := &TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error) {
			return ObserverDecision{Pass: true, Reason: "ok"}, nil
		},
		ExecuteAttempt: exec.Execute,
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	got, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.LatestAttempt() == nil || got.LatestAttempt().Status != AttemptInterrupted {
		t.Fatalf("expected interrupted after start recovery, got %+v", got.LatestAttempt())
	}

	select {
	case <-startedCh:
		t.Fatalf("unexpected: interrupted task started automatically")
	case <-time.After(200 * time.Millisecond):
	}
}

func waitForStatus(t *testing.T, store *Store, taskID string, want AttemptStatus, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := store.GetTask(taskID)
		if err != nil {
			t.Fatalf("GetTask: %v", err)
		}
		if task.LatestAttempt() != nil && task.LatestAttempt().Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	task, _ := store.GetTask(taskID)
	t.Fatalf("timeout waiting for status %q; got %+v", want, task.LatestAttempt())
}
