package secretary

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
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

func TestDispatchPlanAsSW_TaskActions_CancelBulk_InferWorkspaceWhenUnset(t *testing.T) {
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

	o := &Orchestrator{Tasks: store, Runner: runner}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{Action: "cancel"}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", "", plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 2 {
		t.Fatalf("expected 2 canceled_task_ids, got %+v", got.CanceledTaskIDs)
	}
	joined := strings.Join(got.CanceledTaskIDs, ",")
	if !strings.Contains(joined, t1.ID) || !strings.Contains(joined, t2.ID) {
		t.Fatalf("expected canceled_task_ids to include %q and %q, got %+v", t1.ID, t2.ID, got.CanceledTaskIDs)
	}

	updated1, err := store.GetTask(t1.ID)
	if err != nil {
		t.Fatalf("GetTask(t1): %v", err)
	}
	updated2, err := store.GetTask(t2.ID)
	if err != nil {
		t.Fatalf("GetTask(t2): %v", err)
	}
	if updated1.LatestAttempt() == nil || updated1.LatestAttempt().Status != taskqueue.AttemptCanceled {
		t.Fatalf("expected t1 canceled, got %+v", updated1.LatestAttempt())
	}
	if updated2.LatestAttempt() == nil || updated2.LatestAttempt().Status != taskqueue.AttemptCanceled {
		t.Fatalf("expected t2 canceled, got %+v", updated2.LatestAttempt())
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

func TestDispatchPlanAsSW_TaskActions_CancelBulk_GlobalWhenMultipleWorkspacesAndUnset(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws1, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot(ws1): %v", err)
	}
	ws2, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot(ws2): %v", err)
	}

	t1, err := store.CreateTask("local", ws1, "t1", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask(t1): %v", err)
	}
	t2, err := store.CreateTask("local", ws2, "t2", "p2", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask(t2): %v", err)
	}

	o := &Orchestrator{Tasks: store, Runner: &taskqueue.TaskRunner{Store: store}}
	plan := triagePlan{TaskActions: []triageTaskAction{{Action: "cancel"}}}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", "", plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 2 {
		t.Fatalf("expected 2 canceled_task_ids, got %+v (questions=%+v)", got.CanceledTaskIDs, got.Questions)
	}
	joined := strings.Join(got.CanceledTaskIDs, ",")
	if !strings.Contains(joined, t1.ID) || !strings.Contains(joined, t2.ID) {
		t.Fatalf("expected canceled_task_ids to include %q and %q, got %+v", t1.ID, t2.ID, got.CanceledTaskIDs)
	}

	ut1, err := store.GetTask(t1.ID)
	if err != nil {
		t.Fatalf("GetTask(t1): %v", err)
	}
	ut2, err := store.GetTask(t2.ID)
	if err != nil {
		t.Fatalf("GetTask(t2): %v", err)
	}
	if ut1.LatestAttempt() == nil || ut1.LatestAttempt().Status != taskqueue.AttemptCanceled {
		t.Fatalf("expected t1 canceled, got %+v", ut1.LatestAttempt())
	}
	if ut2.LatestAttempt() == nil || ut2.LatestAttempt().Status != taskqueue.AttemptCanceled {
		t.Fatalf("expected t2 canceled, got %+v", ut2.LatestAttempt())
	}
}

func TestDispatchPlanAsSW_TaskActions_CancelByWrappedIDPrefix(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}

	created, err := store.CreateTask("local", ws, "t1", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	o := &Orchestrator{Tasks: store, Runner: &taskqueue.TaskRunner{Store: store}}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{
			Action: "cancel",
			TaskID: fmt.Sprintf("（追踪号 %s）", created.ID[:8]),
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 1 || got.CanceledTaskIDs[0] != created.ID {
		t.Fatalf("expected canceled_task_ids [%q], got %+v (questions=%+v)", created.ID, got.CanceledTaskIDs, got.Questions)
	}
}

func TestDispatchPlanAsSW_TaskActions_CancelByAttemptIDPrefix(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}

	created, err := store.CreateTask("local", ws, "t1", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	latest := created.LatestAttempt()
	if latest == nil {
		t.Fatalf("expected created task to have an attempt")
	}

	o := &Orchestrator{Tasks: store, Runner: &taskqueue.TaskRunner{Store: store}}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{
			Action: "cancel",
			TaskID: latest.ID[:8],
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 1 || got.CanceledTaskIDs[0] != created.ID {
		t.Fatalf("expected canceled_task_ids [%q], got %+v (questions=%+v)", created.ID, got.CanceledTaskIDs, got.Questions)
	}
}

type fakeLLMClient struct {
	Respond func(messages []llm.ChatMessage) (string, error)
	Calls   int
}

func (f *fakeLLMClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, _ *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	f.Calls++
	if f.Respond == nil {
		return "", nil
	}
	return f.Respond(messages)
}

func (f *fakeLLMClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	out, err := f.ChatCompletion(ctx, messages, opts)
	if err != nil {
		return err
	}
	return callback(out)
}

func TestDispatchPlanAsSW_TaskActions_CancelByNaturalRef_UsesLLMResolver(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}

	created, err := store.CreateTask("local", ws, "开发管理系统", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	client := &fakeLLMClient{
		Respond: func(_ []llm.ChatMessage) (string, error) {
			return fmt.Sprintf(`{"match":"single","task_id":%q}`, created.ID), nil
		},
	}
	o := &Orchestrator{
		Tasks:  store,
		Runner: &taskqueue.TaskRunner{Store: store},
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			_, _, _ = ctx, userID, modelID
			return client, "mock", nil
		},
	}
	plan := triagePlan{
		TaskActions: []triageTaskAction{{
			Action: "cancel",
			TaskID: "那个开发管理系统的任务",
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 1 || got.CanceledTaskIDs[0] != created.ID {
		t.Fatalf("expected canceled_task_ids [%q], got %+v (questions=%+v)", created.ID, got.CanceledTaskIDs, got.Questions)
	}
	if client.Calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", client.Calls)
	}
}

func TestDispatchPlanAsSW_TaskActions_CancelByNaturalRef_WithDigits_PreservesFullRef(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}

	created, err := store.CreateTask("local", ws, "写2026文章", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	client := &fakeLLMClient{
		Respond: func(messages []llm.ChatMessage) (string, error) {
			lastUser := ""
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == "user" {
					lastUser = messages[i].Content
					break
				}
			}
			if !strings.Contains(lastUser, "写2026文章的任务") {
				return `{"match":"not_found"}`, nil
			}
			return fmt.Sprintf(`{"match":"single","task_id":%q}`, created.ID), nil
		},
	}
	o := &Orchestrator{
		Tasks:  store,
		Runner: &taskqueue.TaskRunner{Store: store},
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			_, _, _ = ctx, userID, modelID
			return client, "mock", nil
		},
	}
	plan := triagePlan{TaskActions: []triageTaskAction{{Action: "cancel", TaskID: "写2026文章的任务"}}}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 1 || got.CanceledTaskIDs[0] != created.ID {
		t.Fatalf("expected canceled_task_ids [%q], got %+v (questions=%+v)", created.ID, got.CanceledTaskIDs, got.Questions)
	}
}

func TestDispatchPlanAsSW_TaskActions_AmbiguousNaturalRef_YieldsDisambiguationQuestion(t *testing.T) {
	store, err := taskqueue.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ws, err := scope.NormalizeWorkspaceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NormalizeWorkspaceRoot: %v", err)
	}

	t1, err := store.CreateTask("local", ws, "开发管理系统 - backend", "p1", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	t2, err := store.CreateTask("local", ws, "开发管理系统 - frontend", "p2", "", taskqueue.Limits{})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	client := &fakeLLMClient{
		Respond: func(_ []llm.ChatMessage) (string, error) {
			return fmt.Sprintf(`{"match":"ambiguous","candidates":[%q,%q]}`, t1.ID, t2.ID), nil
		},
	}
	o := &Orchestrator{
		Tasks:  store,
		Runner: &taskqueue.TaskRunner{Store: store},
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			_, _, _ = ctx, userID, modelID
			return client, "mock", nil
		},
	}
	plan := triagePlan{TaskActions: []triageTaskAction{{Action: "cancel", TaskID: "那个开发管理系统的任务"}}}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", ws, plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}
	if len(got.CanceledTaskIDs) != 0 {
		t.Fatalf("expected no canceled_task_ids, got %+v", got.CanceledTaskIDs)
	}
	joined := strings.Join(got.Questions, "\n")
	if !strings.Contains(joined, "匹配到多个任务") {
		t.Fatalf("expected ambiguous question, got %+v", got.Questions)
	}
	if !strings.Contains(joined, "backend") || !strings.Contains(joined, "frontend") {
		t.Fatalf("expected candidate hints in question, got %+v", got.Questions)
	}
}
