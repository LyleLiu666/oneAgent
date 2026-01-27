package taskqueue

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/scope"
)

func TestStore_CreateTask_PersistsTaskAndEvents(t *testing.T) {
	base := time.Date(2026, 1, 27, 10, 0, 0, 0, time.UTC)

	origNow := Now
	origNewID := NewID
	t.Cleanup(func() {
		Now = origNow
		NewID = origNewID
	})
	Now = func() time.Time { return base }

	ids := []string{"task-1", "attempt-1"}
	NewID = func() string {
		if len(ids) == 0 {
			return "unexpected"
		}
		next := ids[0]
		ids = ids[1:]
		return next
	}

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	workspace := t.TempDir()
	normalizedWorkspace, err := scope.NormalizeWorkspaceRoot(workspace)
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}
	task, err := store.CreateTask("local", workspace, "hello", "do the thing", "model-1", Limits{MaxSteps: 5})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if task.ID != "task-1" {
		t.Fatalf("expected task id task-1, got %q", task.ID)
	}
	if task.UserID != "local" || task.Workspace != normalizedWorkspace || task.Title != "hello" || task.Prompt != "do the thing" || task.ModelID != "model-1" {
		t.Fatalf("unexpected task fields: %+v", task)
	}
	if task.CreatedAt != base || task.UpdatedAt != base {
		t.Fatalf("expected timestamps = %v, got created=%v updated=%v", base, task.CreatedAt, task.UpdatedAt)
	}
	if len(task.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(task.Attempts))
	}
	if task.Attempts[0].ID != "attempt-1" || task.Attempts[0].Status != AttemptQueued {
		t.Fatalf("unexpected attempt: %+v", task.Attempts[0])
	}

	if _, err := os.Stat(filepath.Join(store.TasksDir(), "task-1", "task.json")); err != nil {
		t.Fatalf("expected task.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.TasksDir(), "task-1", "events.jsonl")); err != nil {
		t.Fatalf("expected events.jsonl to exist: %v", err)
	}

	loaded, err := store.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if loaded.ID != task.ID || loaded.Workspace != task.Workspace || len(loaded.Attempts) != 1 || loaded.Attempts[0].ID != "attempt-1" {
		t.Fatalf("unexpected loaded task: %+v", loaded)
	}

	evs, err := store.ReadEvents("task-1")
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	if evs[0].Type != "task.created" || evs[0].TaskID != "task-1" || evs[0].AttemptID != "attempt-1" {
		t.Fatalf("unexpected event: %+v", evs[0])
	}
}

func TestStore_ListTasks_SortsByCreatedAtAscending(t *testing.T) {
	base := time.Date(2026, 1, 27, 10, 0, 0, 0, time.UTC)

	origNow := Now
	origNewID := NewID
	t.Cleanup(func() {
		Now = origNow
		NewID = origNewID
	})

	var tick int64
	Now = func() time.Time {
		out := base.Add(time.Duration(tick) * time.Second)
		tick++
		return out
	}

	ids := []string{"task-1", "attempt-1", "task-2", "attempt-2"}
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
	normalizedWorkspace, err := scope.NormalizeWorkspaceRoot(workspace)
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}
	if _, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{}); err != nil {
		t.Fatalf("CreateTask A: %v", err)
	}
	if _, err := store.CreateTask("local", workspace, "B", "task B", "", Limits{}); err != nil {
		t.Fatalf("CreateTask B: %v", err)
	}

	list, err := store.ListTasks("local", normalizedWorkspace)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(list))
	}
	if list[0].ID != "task-1" || list[1].ID != "task-2" {
		t.Fatalf("expected ordering task-1, task-2; got %q, %q", list[0].ID, list[1].ID)
	}
}

func TestStore_UpdateTask_PersistsChanges(t *testing.T) {
	base := time.Date(2026, 1, 27, 10, 0, 0, 0, time.UTC)

	origNow := Now
	origNewID := NewID
	t.Cleanup(func() {
		Now = origNow
		NewID = origNewID
	})
	Now = func() time.Time { return base }

	ids := []string{"task-1", "attempt-1"}
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
	if _, err := store.CreateTask("local", workspace, "A", "task A", "", Limits{}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	now2 := base.Add(5 * time.Minute)
	Now = func() time.Time { return now2 }

	updated, err := store.UpdateTask("task-1", func(tk *Task) error {
		attempt := tk.LatestAttempt()
		if attempt == nil {
			return fmt.Errorf("expected attempt")
		}
		attempt.Status = AttemptRunning
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.UpdatedAt != now2 {
		t.Fatalf("expected updated_at=%v, got %v", now2, updated.UpdatedAt)
	}

	reloaded, err := store.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if reloaded.LatestAttempt() == nil || reloaded.LatestAttempt().Status != AttemptRunning {
		t.Fatalf("expected latest attempt status running, got %+v", reloaded.LatestAttempt())
	}
}
