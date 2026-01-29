package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/liu_y/oneAgent/backend/internal/usage"
)

type DecisionMaker interface {
	Decide(ctx context.Context, in ObserveInput) (ObserverDecision, error)
}

type AttemptResult struct {
	RunID              string
	Summary            string
	FindingsPath       string
	TraceLogPath       string
	TestReportPath     string
	DiffPatchPath      string
	ChangedFilesPath   string
	ReviewCommentsPath string

	WorktreeRoot  string
	BaseCommitSHA string
	BaseRef       string

	ProjectConfigPath    string
	CopyFilesLogPath     string
	SetupScriptLogPath   string
	TestScriptLogPath    string
	CleanupScriptLogPath string

	Usage *usage.Totals
}

type ExecuteAttemptFunc func(ctx context.Context, task Task, attempt Attempt, resumedFrom *Attempt) (AttemptResult, error)

type TaskRunner struct {
	Store *Store

	DecideOutcome func(ctx context.Context, task Task, attempt Attempt) (ObserverDecision, error)

	ExecuteAttempt ExecuteAttemptFunc

	started atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc

	mu      sync.Mutex
	workers map[string]*workspaceWorker
	wg      sync.WaitGroup

	running sync.Map // map[string]context.CancelFunc (key=task_id)
}

type workspaceWorker struct {
	workspace string
	queue     chan string
}

func (r *TaskRunner) Start() error {
	if r == nil {
		return errors.New("runner is nil")
	}
	if r.Store == nil {
		return errors.New("store is required")
	}
	if r.DecideOutcome == nil {
		return errors.New("decide outcome function is required")
	}
	if r.ExecuteAttempt == nil {
		return errors.New("execute attempt function is required")
	}

	if r.started.Swap(true) {
		return nil
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.workers = make(map[string]*workspaceWorker)

	// Recovery on (re)start:
	// - running -> interrupted (no auto-resume)
	// - queued  -> enqueue
	tasks, err := r.Store.ListTasks("", "")
	if err != nil {
		return err
	}

	for _, task := range tasks {
		latest := task.LatestAttempt()
		if latest == nil {
			continue
		}
		switch latest.Status {
		case AttemptRunning:
			attemptID := latest.ID
			now := Now()
			_, err := r.Store.UpdateTask(task.ID, func(tk *Task) error {
				a := tk.LatestAttempt()
				if a == nil || a.ID != attemptID || a.Status != AttemptRunning {
					return nil
				}
				a.Status = AttemptInterrupted
				a.FinishedAt = &now
				if a.Error == "" {
					a.Error = "interrupted: server restarted"
				}
				return nil
			})
			if err == nil {
				_ = r.Store.AppendEvent(Event{
					TaskID:    task.ID,
					AttemptID: attemptID,
					Type:      "attempt.interrupted",
					Message:   "Server restarted; attempt marked interrupted",
				})
			}

		case AttemptQueued:
			_ = r.Enqueue(task.ID)
		}
	}

	return nil
}

func (r *TaskRunner) Stop() {
	if r == nil {
		return
	}
	if r.cancel != nil {
		r.cancel()
	}

	r.running.Range(func(_, v any) bool {
		if cancel, ok := v.(context.CancelFunc); ok {
			cancel()
		}
		return true
	})

	r.wg.Wait()
}

func (r *TaskRunner) Enqueue(taskID string) error {
	if r == nil || r.Store == nil {
		return errors.New("runner not initialized")
	}
	if r.ctx == nil || r.ctx.Err() != nil {
		return errors.New("runner is stopped")
	}
	task, err := r.Store.GetTask(taskID)
	if err != nil {
		return err
	}
	latest := task.LatestAttempt()
	if latest == nil {
		return errors.New("task has no attempts")
	}
	if latest.Status != AttemptQueued {
		return nil
	}
	ws := task.Workspace
	if ws == "" {
		return errors.New("task workspace is required")
	}

	worker := r.ensureWorker(ws)
	select {
	case worker.queue <- taskID:
		return nil
	case <-r.ctx.Done():
		return errors.New("runner is stopped")
	}
}

func (r *TaskRunner) Cancel(taskID string) (Task, error) {
	if r == nil || r.Store == nil {
		return Task{}, errors.New("runner not initialized")
	}

	task, err := r.Store.GetTask(taskID)
	if err != nil {
		return Task{}, err
	}
	latest := task.LatestAttempt()
	if latest == nil {
		return Task{}, errors.New("task has no attempts")
	}
	attemptID := latest.ID

	switch latest.Status {
	case AttemptQueued:
		now := Now()
		updated, err := r.Store.UpdateTask(taskID, func(tk *Task) error {
			a := tk.LatestAttempt()
			if a == nil || a.ID != attemptID || a.Status != AttemptQueued {
				return nil
			}
			a.Status = AttemptCanceled
			a.FinishedAt = &now
			return nil
		})
		if err != nil {
			return Task{}, err
		}
		_ = r.Store.AppendEvent(Event{
			TaskID:    taskID,
			AttemptID: attemptID,
			Type:      "attempt.canceled",
			Message:   "Attempt canceled while queued",
		})
		return updated, nil

	case AttemptRunning:
		if v, ok := r.running.Load(taskID); ok {
			if cancel, ok := v.(context.CancelFunc); ok {
				cancel()
			}
		}
		_ = r.Store.AppendEvent(Event{
			TaskID:    taskID,
			AttemptID: attemptID,
			Type:      "attempt.cancel_requested",
			Message:   "Cancel requested",
		})
		return task, nil

	default:
		return task, nil
	}
}

func (r *TaskRunner) Resume(taskID string, reviewNotes string) (Task, error) {
	return r.enqueueAttempt(taskID, reviewNotes, "resume", false)
}

func truncateReviewNotes(reviewNotes string) string {
	notes := strings.TrimSpace(reviewNotes)
	if notes == "" {
		return ""
	}
	if len([]rune(notes)) > 2000 {
		return string([]rune(notes)[:2000]) + "…"
	}
	return notes
}

func (r *TaskRunner) enqueueAttempt(taskID string, reviewNotes string, source string, auto bool) (Task, error) {
	if r == nil || r.Store == nil {
		return Task{}, errors.New("runner not initialized")
	}

	var (
		newAttemptID string
		fromAttempt  string
	)

	normalizedNotes := truncateReviewNotes(reviewNotes)

	updated, err := r.Store.UpdateTask(taskID, func(tk *Task) error {
		latest := tk.LatestAttempt()
		if latest == nil {
			return errors.New("task has no attempts")
		}
		switch latest.Status {
		case AttemptSucceeded, AttemptFailed, AttemptCanceled, AttemptTimedOut, AttemptInterrupted, AttemptLimitExceeded:
			// ok
		case AttemptQueued, AttemptRunning:
			return fmt.Errorf("resume not allowed from status %q", latest.Status)
		default:
			return fmt.Errorf("resume not allowed from status %q", latest.Status)
		}

		now := Now()
		newAttemptID = NewID()
		fromAttempt = latest.ID
		tk.Attempts = append(tk.Attempts, Attempt{
			ID:                   newAttemptID,
			Status:               AttemptQueued,
			CreatedAt:            now,
			ResumedFromAttemptID: latest.ID,
			Auto:                 auto,
			PrincipalID:          tk.UserID,
			ReviewNotes:          normalizedNotes,
		})
		return nil
	})
	if err != nil {
		return Task{}, err
	}

	data := map[string]any{
		"resumed_from_attempt_id": fromAttempt,
		"source":                  strings.TrimSpace(source),
		"auto":                    auto,
	}
	if strings.TrimSpace(normalizedNotes) != "" {
		data["review_notes"] = normalizedNotes
	}

	_ = r.Store.AppendEvent(Event{
		TaskID:    taskID,
		AttemptID: newAttemptID,
		Type:      "attempt.queued",
		Message:   "Attempt queued",
		Data:      data,
	})

	_ = r.Enqueue(taskID)
	return updated, nil
}

func (r *TaskRunner) ensureWorker(workspace string) *workspaceWorker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if w, ok := r.workers[workspace]; ok {
		return w
	}

	w := &workspaceWorker{
		workspace: workspace,
		queue:     make(chan string, 64),
	}
	r.workers[workspace] = w

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.runWorker(workspace, w.queue)
	}()

	return w
}

func (r *TaskRunner) runWorker(workspace string, queue <-chan string) {
	for {
		select {
		case <-r.ctx.Done():
			return
		case taskID := <-queue:
			r.processTask(workspace, taskID)
		}
	}
}

func (r *TaskRunner) processTask(workspace string, taskID string) {
	task, err := r.Store.GetTask(taskID)
	if err != nil {
		return
	}
	if task.Workspace != workspace {
		return
	}

	latest := task.LatestAttempt()
	if latest == nil {
		_ = r.Store.AppendEvent(Event{TaskID: taskID, Type: "task.error", Message: "task has no attempts"})
		return
	}
	attemptID := latest.ID

	// Transition queued -> running (idempotent).
	startedAt := Now()
	updated, err := r.Store.UpdateTask(taskID, func(tk *Task) error {
		a := tk.LatestAttempt()
		if a == nil || a.ID != attemptID || a.Status != AttemptQueued {
			return nil
		}
		a.Status = AttemptRunning
		a.StartedAt = &startedAt
		return nil
	})
	if err != nil {
		return
	}
	after := updated.LatestAttempt()
	if after == nil || after.ID != attemptID || after.Status != AttemptRunning {
		return
	}

	_ = r.Store.AppendEvent(Event{
		TaskID:    taskID,
		AttemptID: attemptID,
		Type:      "attempt.running",
		Message:   "Attempt started",
	})

	attemptCtx, cancel := context.WithCancel(r.ctx)
	r.running.Store(taskID, cancel)
	defer func() {
		r.running.Delete(taskID)
		cancel()
	}()

	resumedFrom := findAttemptByID(updated.Attempts, after.ResumedFromAttemptID)
	result, runErr := r.ExecuteAttempt(attemptCtx, updated, *after, resumedFrom)

	ranAttempt := *after
	ranAttempt.RunID = strings.TrimSpace(result.RunID)
	ranAttempt.Summary = strings.TrimSpace(result.Summary)
	ranAttempt.FindingsPath = strings.TrimSpace(result.FindingsPath)
	ranAttempt.TraceLogPath = strings.TrimSpace(result.TraceLogPath)
	ranAttempt.TestReportPath = strings.TrimSpace(result.TestReportPath)
		ranAttempt.DiffPatchPath = strings.TrimSpace(result.DiffPatchPath)
		ranAttempt.ChangedFilesPath = strings.TrimSpace(result.ChangedFilesPath)
		ranAttempt.ReviewCommentsPath = strings.TrimSpace(result.ReviewCommentsPath)
		ranAttempt.WorktreeRoot = strings.TrimSpace(result.WorktreeRoot)
		ranAttempt.BaseCommitSHA = strings.TrimSpace(result.BaseCommitSHA)
		ranAttempt.BaseRef = strings.TrimSpace(result.BaseRef)
		ranAttempt.ProjectConfigPath = strings.TrimSpace(result.ProjectConfigPath)
		ranAttempt.CopyFilesLogPath = strings.TrimSpace(result.CopyFilesLogPath)
	ranAttempt.SetupScriptLogPath = strings.TrimSpace(result.SetupScriptLogPath)
	ranAttempt.TestScriptLogPath = strings.TrimSpace(result.TestScriptLogPath)
	ranAttempt.CleanupScriptLogPath = strings.TrimSpace(result.CleanupScriptLogPath)
	ranAttempt.Usage = result.Usage

	finishedAt := Now()
	finalStatus := AttemptFailed
	finalError := ""
	observerFailed := false
	var decision ObserverDecision

	if runErr != nil {
		finalError = runErr.Error()
		var budgetErr *usage.BudgetExceededError
		if errors.As(runErr, &budgetErr) {
			finalStatus = AttemptLimitExceeded
		} else if errors.Is(runErr, context.Canceled) || errors.Is(attemptCtx.Err(), context.Canceled) {
			finalStatus = AttemptCanceled
		} else if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(attemptCtx.Err(), context.DeadlineExceeded) {
			finalStatus = AttemptTimedOut
		} else {
			finalStatus = AttemptFailed
		}
	} else {
		decision, err = r.DecideOutcome(attemptCtx, updated, ranAttempt)
		if err != nil {
			finalStatus = AttemptFailed
			finalError = err.Error()
		} else if decision.Pass {
			finalStatus = AttemptSucceeded
		} else {
			finalStatus = AttemptFailed
			observerFailed = true
		}
		ranAttempt.Observer = &decision

		// Enforce deliverable artifacts for succeeded attempts.
		if finalStatus == AttemptSucceeded {
			if strings.TrimSpace(ranAttempt.FindingsPath) == "" || !fileExists(ranAttempt.FindingsPath) {
				finalStatus = AttemptFailed
				finalError = "missing findings_path"
			} else if strings.TrimSpace(ranAttempt.TraceLogPath) == "" || !fileExists(ranAttempt.TraceLogPath) {
				finalStatus = AttemptFailed
				finalError = "missing trace_log_path"
			}
		}
	}

	ranAttempt.Status = finalStatus
	ranAttempt.FinishedAt = &finishedAt
	ranAttempt.Error = strings.TrimSpace(finalError)
	if ranAttempt.StartedAt == nil {
		ranAttempt.StartedAt = &startedAt
	}

	saved, err := r.Store.UpdateTask(taskID, func(tk *Task) error {
		a := tk.LatestAttempt()
		if a == nil || a.ID != attemptID {
			return nil
		}
		checkpointPath := a.CheckpointPath
		rolledBackAt := a.RolledBackAt
		rollbackError := a.RollbackError
		*a = ranAttempt
		a.CheckpointPath = checkpointPath
		a.RolledBackAt = rolledBackAt
		a.RollbackError = rollbackError
		return nil
	})
	if err != nil {
		return
	}

	_ = r.Store.AppendEvent(Event{
		TaskID:    taskID,
		AttemptID: attemptID,
		Type:      "attempt.finished",
		Message:   fmt.Sprintf("Attempt finished: %s", finalStatus),
		Data: map[string]any{
			"status": finalStatus,
			"error":  finalError,
		},
	})

	if observerFailed {
		latest := saved.LatestAttempt()
		if latest == nil || latest.ID != attemptID || latest.Status != AttemptFailed {
			return
		}
		if strings.TrimSpace(decision.NextSteps) == "" {
			return
		}
		if len(decision.QuestionsForUser) > 0 {
			return
		}

		autoAttempts := 0
		for _, a := range saved.Attempts {
			if a.Auto {
				autoAttempts++
			}
		}

		effectiveLimits := ResolveLimits(saved.Limits)
		if effectiveLimits.MaxAutoAttempts <= 0 {
			return
		}
		if autoAttempts >= effectiveLimits.MaxAutoAttempts {
			return
		}

		reviewNotes := strings.TrimSpace(decision.NextSteps)
		_, _ = r.enqueueAttempt(taskID, reviewNotes, "observer", true)
	}
}

func findAttemptByID(attempts []Attempt, id string) *Attempt {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for i := range attempts {
		if attempts[i].ID == id {
			return &attempts[i]
		}
	}
	return nil
}

func fileExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
