package taskqueue

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskRunner_Governance_ScheduleMisfirePolicies(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := t0.Add(35 * time.Second) // missed 3 intervals for EverySeconds=10

	cases := []struct {
		name      string
		policy    string
		wantTasks int
		wantNext  time.Time
	}{
		{name: "one_shot", policy: "one_shot", wantTasks: 1, wantNext: t0.Add(40 * time.Second)},
		{name: "catch_up", policy: "catch_up", wantTasks: 4, wantNext: t0.Add(40 * time.Second)},
		{name: "skip", policy: "skip", wantTasks: 0, wantNext: t0.Add(40 * time.Second)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			store, err := NewStore(filepath.Join(base, "tasks"))
			if err != nil {
				t.Fatalf("NewStore: %v", err)
			}

			ws := filepath.Join(base, "ws")
			mustMkdir(t, ws)

			_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
				g.Schedules = []Schedule{{
					ID:            "s1",
					Enabled:       true,
					UserID:        "local",
					Workspace:     ws,
					Title:         "scheduled",
					Prompt:        "run scheduled",
					EverySeconds:  10,
					NextRunAt:     t0,
					MisfirePolicy: tc.policy,
					CreatedAt:     t0,
					UpdatedAt:     t0,
				}}
				return nil
			})
			if err != nil {
				t.Fatalf("UpdateGovernance: %v", err)
			}

			r := &TaskRunner{Store: store}
			r.runSchedulesOnce(now)

			tasks, err := store.ListTasks("local", ws)
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			if len(tasks) != tc.wantTasks {
				t.Fatalf("expected %d tasks, got %d", tc.wantTasks, len(tasks))
			}

			g, err := store.GetGovernance()
			if err != nil {
				t.Fatalf("GetGovernance: %v", err)
			}
			if len(g.Schedules) != 1 {
				t.Fatalf("expected 1 schedule, got %d", len(g.Schedules))
			}
			gotNext := g.Schedules[0].NextRunAt.UTC()
			if !gotNext.Equal(tc.wantNext.UTC()) {
				t.Fatalf("expected next_run_at=%s, got %s", tc.wantNext.UTC().Format(time.RFC3339Nano), gotNext.Format(time.RFC3339Nano))
			}
		})
	}
}

func TestTaskRunner_Governance_ScheduleTriggerIsIdempotentForWindow(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "tasks"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := filepath.Join(base, "ws")
	mustMkdir(t, ws)

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := t0.Add(1 * time.Second)

	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Schedules = []Schedule{{
			ID:            "s1",
			Enabled:       true,
			UserID:        "local",
			Workspace:     ws,
			Title:         "scheduled",
			Prompt:        "run scheduled",
			EverySeconds:  10,
			NextRunAt:     t0,
			MisfirePolicy: "one_shot",
			CreatedAt:     t0,
			UpdatedAt:     t0,
		}}
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	r := &TaskRunner{Store: store}
	r.runSchedulesOnce(now)

	tasks, err := store.ListTasks("local", ws)
	if err != nil {
		t.Fatalf("ListTasks(after first run): %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after first run, got %d", len(tasks))
	}

	// Simulate a crash/rollback where schedule state did not persist (window triggered twice).
	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Schedules[0].NextRunAt = t0
		g.Schedules[0].LastTriggerKey = ""
		g.Schedules[0].LastEnqueueAt = time.Time{}
		g.Schedules[0].LastEnqueueError = ""
		g.Schedules[0].LastEnqueueErrorAt = time.Time{}
		return nil
	})
	if err != nil {
		t.Fatalf("reset schedule: %v", err)
	}

	r.runSchedulesOnce(now)

	tasks, err = store.ListTasks("local", ws)
	if err != nil {
		t.Fatalf("ListTasks(after second run): %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected idempotent trigger to keep 1 task, got %d", len(tasks))
	}
}

func TestTaskRunner_Governance_ScheduleRetriesOnEnqueueFailure(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "tasks"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws := filepath.Join(base, "ws")
	mustMkdir(t, ws)

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := t0.Add(1 * time.Second)

	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Schedules = []Schedule{{
			ID:            "s1",
			Enabled:       true,
			UserID:        "local",
			Workspace:     ws,
			Title:         "scheduled",
			Prompt:        "run scheduled",
			EverySeconds:  10,
			NextRunAt:     t0,
			MisfirePolicy: "one_shot",
			CreatedAt:     t0,
			UpdatedAt:     t0,
		}}
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	// Workspace becomes unavailable after schedule is configured.
	if err := os.RemoveAll(ws); err != nil {
		t.Fatalf("RemoveAll(workspace): %v", err)
	}

	r := &TaskRunner{Store: store}
	r.runSchedulesOnce(now)

	tasks, err := store.ListTasks("local", ws)
	if err == nil && len(tasks) != 0 {
		t.Fatalf("expected no tasks created while workspace missing; got %d", len(tasks))
	}

	g, err := store.GetGovernance()
	if err != nil {
		t.Fatalf("GetGovernance: %v", err)
	}
	sc := g.Schedules[0]
	if !sc.NextRunAt.UTC().Equal(t0.UTC()) {
		t.Fatalf("expected next_run_at to remain due for retry; got %s", sc.NextRunAt.UTC().Format(time.RFC3339Nano))
	}
	if sc.LastEnqueueError == "" {
		t.Fatalf("expected last_enqueue_error to be set")
	}
	if sc.LastEnqueueErrorAt.IsZero() {
		t.Fatalf("expected last_enqueue_error_at to be set")
	}

	// Once workspace is restored, schedule should eventually enqueue and clear the error.
	mustMkdir(t, ws)
	r.runSchedulesOnce(now.Add(1 * time.Second))

	tasks, err = store.ListTasks("local", ws)
	if err != nil {
		t.Fatalf("ListTasks(after restore): %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after workspace restored, got %d", len(tasks))
	}

	g, err = store.GetGovernance()
	if err != nil {
		t.Fatalf("GetGovernance(after restore): %v", err)
	}
	sc = g.Schedules[0]
	if sc.LastEnqueueError != "" {
		t.Fatalf("expected last_enqueue_error to be cleared after success, got %q", sc.LastEnqueueError)
	}
}
