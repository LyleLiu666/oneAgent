package secretary

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestDispatchPlanAsSW_DoesNotInjectWorkspaceQuestion_WhenPlanAlreadyHasQuestions(t *testing.T) {
	o := &Orchestrator{
		Tasks:  &taskqueue.Store{},
		Runner: &taskqueue.TaskRunner{},
	}

	plan := triagePlan{
		Tasks: []triageTask{{
			Title:             "跑测试",
			Prompt:            "run tests",
			WorkspaceStrategy: "session",
		}},
		Questions: []string{"请把仓库根目录路径发我，我才能继续。"},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", "", plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}

	if len(got.CreatedTaskIDs) != 0 {
		t.Fatalf("expected no dispatched tasks, got %v", got.CreatedTaskIDs)
	}
	if len(got.Questions) != 1 {
		t.Fatalf("expected only plan question, got %v", got.Questions)
	}
	if got.Questions[0] != plan.Questions[0] {
		t.Fatalf("expected question %q, got %q", plan.Questions[0], got.Questions[0])
	}
}

func TestDispatchPlanAsSW_AddsFallbackQuestion_WhenWorkspaceIsRequiredButNoQuestionsProvided(t *testing.T) {
	o := &Orchestrator{
		Tasks:  &taskqueue.Store{},
		Runner: &taskqueue.TaskRunner{},
	}

	plan := triagePlan{
		Tasks: []triageTask{{
			Title:             "跑测试",
			Prompt:            "run tests",
			WorkspaceStrategy: "session",
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", "", plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}

	if len(got.CreatedTaskIDs) != 0 {
		t.Fatalf("expected no dispatched tasks, got %v", got.CreatedTaskIDs)
	}
	if len(got.Questions) == 0 {
		t.Fatalf("expected a fallback question, got none")
	}
}

func TestDispatchPlanAsSW_TaskActions_CancelByIDPrefix(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}
	_, err = store.UpdateGovernance(func(g *taskqueue.QueueGovernance) error {
		if g.Workspaces == nil {
			g.Workspaces = map[string]taskqueue.WorkspacePolicy{}
		}
		p := g.Workspaces[ws]
		p.Paused = true
		g.Workspaces[ws] = p
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	runner := &taskqueue.TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			return taskqueue.ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, resumedFrom *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			return taskqueue.AttemptResult{}, nil
		},
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("runner.Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	created, err := store.CreateTask("local", ws, "t1", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	o := &Orchestrator{Tasks: store, Runner: runner}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{
			Action: "cancel",
			TaskID: created.ID[:8],
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 1 || got.CanceledTaskIDs[0] != created.ID {
		t.Fatalf("expected canceled_task_ids [%q], got %+v", created.ID, got.CanceledTaskIDs)
	}

	updated, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if updated.LatestAttempt() == nil || updated.LatestAttempt().Status != taskqueue.AttemptCanceled {
		t.Fatalf("expected status canceled, got %+v", updated.LatestAttempt())
	}
}

func TestDispatchPlanAsSW_TaskActions_ResumeBulkInSessionWorkspace(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}
	_, err = store.UpdateGovernance(func(g *taskqueue.QueueGovernance) error {
		if g.Workspaces == nil {
			g.Workspaces = map[string]taskqueue.WorkspacePolicy{}
		}
		p := g.Workspaces[ws]
		p.Paused = true
		g.Workspaces[ws] = p
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	runner := &taskqueue.TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			return taskqueue.ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, resumedFrom *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			return taskqueue.AttemptResult{}, nil
		},
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("runner.Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	t1, err := store.CreateTask("local", ws, "t1", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask(t1): %v", err)
	}
	t2, err := store.CreateTask("local", ws, "t2", "p2", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask(t2): %v", err)
	}

	now := time.Now().UTC()
	_, err = store.UpdateTask(t1.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		if a == nil {
			return nil
		}
		a.Status = taskqueue.AttemptFailed
		a.FinishedAt = &now
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTask(t1): %v", err)
	}
	_, err = store.UpdateTask(t2.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		if a == nil {
			return nil
		}
		a.Status = taskqueue.AttemptInterrupted
		a.FinishedAt = &now
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTask(t2): %v", err)
	}

	o := &Orchestrator{Tasks: store, Runner: runner}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{
			Action:      "resume",
			ReviewNotes: "please continue",
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.ResumedTaskIDs) != 2 {
		t.Fatalf("expected 2 resumed tasks, got %+v", got.ResumedTaskIDs)
	}

	ut1, err := store.GetTask(t1.ID)
	if err != nil {
		t.Fatalf("GetTask(t1): %v", err)
	}
	if len(ut1.Attempts) != 2 || ut1.LatestAttempt() == nil || ut1.LatestAttempt().Status != taskqueue.AttemptQueued {
		t.Fatalf("expected t1 to have a queued resumed attempt, got %+v", ut1.Attempts)
	}
	if strings.TrimSpace(ut1.LatestAttempt().ReviewNotes) != "please continue" {
		t.Fatalf("expected t1 review_notes to be injected, got %q", ut1.LatestAttempt().ReviewNotes)
	}

	ut2, err := store.GetTask(t2.ID)
	if err != nil {
		t.Fatalf("GetTask(t2): %v", err)
	}
	if len(ut2.Attempts) != 2 || ut2.LatestAttempt() == nil || ut2.LatestAttempt().Status != taskqueue.AttemptQueued {
		t.Fatalf("expected t2 to have a queued resumed attempt, got %+v", ut2.Attempts)
	}
	if strings.TrimSpace(ut2.LatestAttempt().ReviewNotes) != "please continue" {
		t.Fatalf("expected t2 review_notes to be injected, got %q", ut2.LatestAttempt().ReviewNotes)
	}
}

func TestDispatchPlanAsSW_TaskActions_CancelDoesNotReportNonCancelableTask(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}
	_, err = store.UpdateGovernance(func(g *taskqueue.QueueGovernance) error {
		if g.Workspaces == nil {
			g.Workspaces = map[string]taskqueue.WorkspacePolicy{}
		}
		p := g.Workspaces[ws]
		p.Paused = true
		g.Workspaces[ws] = p
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	runner := &taskqueue.TaskRunner{
		Store: store,
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			return taskqueue.ObserverDecision{Pass: true}, nil
		},
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, resumedFrom *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			return taskqueue.AttemptResult{}, nil
		},
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("runner.Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	created, err := store.CreateTask("local", ws, "t1", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	now := time.Now().UTC()
	_, err = store.UpdateTask(created.ID, func(tk *taskqueue.Task) error {
		a := tk.LatestAttempt()
		if a == nil {
			return nil
		}
		a.Status = taskqueue.AttemptSucceeded
		a.FinishedAt = &now
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	o := &Orchestrator{Tasks: store, Runner: runner}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{
			Action: "cancel",
			TaskID: created.ID,
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 0 {
		t.Fatalf("expected no canceled_task_ids, got %+v", got.CanceledTaskIDs)
	}
	if len(got.Questions) == 0 {
		t.Fatalf("expected a user-facing message, got none")
	}
	joined := strings.Join(got.Questions, "\n")
	if !strings.Contains(joined, "取消未生效") || !strings.Contains(joined, created.ID) {
		t.Fatalf("unexpected questions: %+v", got.Questions)
	}
}

func TestDispatchPlanAsSW_TaskActions_UnsupportedAction_YieldsQuestion(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	runner := &taskqueue.TaskRunner{Store: store}
	o := &Orchestrator{Tasks: store, Runner: runner}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{
			Action: "delete",
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", "", plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CreatedTaskIDs) != 0 || len(got.CanceledTaskIDs) != 0 || len(got.ResumedTaskIDs) != 0 {
		t.Fatalf("expected no side effects, got %+v", got)
	}
	if len(got.Questions) == 0 {
		t.Fatalf("expected a question, got none")
	}
	if !strings.Contains(strings.Join(got.Questions, "\n"), "不支持的任务操作") {
		t.Fatalf("unexpected questions: %+v", got.Questions)
	}
}
