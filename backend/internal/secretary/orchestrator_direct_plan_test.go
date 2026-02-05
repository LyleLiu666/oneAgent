package secretary

import (
	"context"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestTriage_DirectCommand_CancelAll_DoesNotRequireLLM(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}
	tasks, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := t.TempDir()
	t1, err := tasks.CreateTask("local", ws, "T1", "task 1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	t2, err := tasks.CreateTask("local", ws, "T2", "task 2", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	now := time.Now().UTC()
	for _, id := range []string{t1.ID, t2.ID} {
		if _, err := tasks.UpdateTask(id, func(tk *taskqueue.Task) error {
			a := tk.LatestAttempt()
			a.Status = taskqueue.AttemptFailed
			a.StartedAt = &now
			a.FinishedAt = &now
			a.Error = "boom"
			return nil
		}); err != nil {
			t.Fatalf("UpdateTask: %v", err)
		}
	}

	runner := &taskqueue.TaskRunner{Store: tasks}
	o := &Orchestrator{
		Sessions: sessions,
		Tasks:    tasks,
		Runner:   runner,
		// No ResolveModel: secretary MUST NOT execute any user-requested actions without an LLM plan.
	}

	if _, err := o.AppendInboxMessage(context.Background(), "local", "session-1", "都删掉", ""); err != nil {
		t.Fatalf("AppendInboxMessage: %v", err)
	}

	_, err = o.Triage(context.Background(), "local", "session-1", nil)
	if err == nil {
		t.Fatalf("expected Triage to fail without ResolveModel/LLM")
	}

	got1, err := tasks.GetTask(t1.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got1.LatestAttempt() == nil || got1.LatestAttempt().Status != taskqueue.AttemptFailed {
		t.Fatalf("expected task1 to remain failed, got %+v", got1.LatestAttempt())
	}

	got2, err := tasks.GetTask(t2.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got2.LatestAttempt() == nil || got2.LatestAttempt().Status != taskqueue.AttemptFailed {
		t.Fatalf("expected task2 to remain failed, got %+v", got2.LatestAttempt())
	}
}
