package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	WorktreeRoot          string
	BaseCommitSHA         string
	BaseRef               string
	WorktreeMode          string
	WorktreeCleanupStatus string
	WorktreeCleanupError  string
	WorktreeCleanupHint   string

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

	// OnAttemptFinished is a best-effort hook invoked after an attempt transitions to a terminal status.
	// It MUST NOT panic; panics are recovered by the runner.
	OnAttemptFinished func(ctx context.Context, task Task, attempt Attempt)

	started atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc

	mu sync.Mutex
	wg sync.WaitGroup

	notify chan struct{}

	queues             map[string][]string
	runningWorkspaces  map[string]bool
	workspaceAges      map[string]int
	deferEventLast     map[string]time.Time
	lastDeferredScanAt time.Time

	running sync.Map // map[string]context.CancelFunc (key=task_id)
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
	r.notify = make(chan struct{}, 1)
	r.queues = make(map[string][]string)
	r.runningWorkspaces = make(map[string]bool)
	r.workspaceAges = make(map[string]int)
	r.deferEventLast = make(map[string]time.Time)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.runScheduler()
	}()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.runSchedules()
	}()

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
			updatedTask, err := r.Store.UpdateTask(task.ID, func(tk *Task) error {
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

				finalAttempt := updatedTask.LatestAttempt()
				if finalAttempt != nil && finalAttempt.ID == attemptID {
					normalized := *finalAttempt
					manifestErr := EnsureArtifactManifestV1(r.Store.TasksDir(), updatedTask, &normalized)
					if manifestErr == nil {
						updatedTask, _ = r.Store.UpdateTask(task.ID, func(tk *Task) error {
							a := tk.LatestAttempt()
							if a == nil || a.ID != attemptID || a.Status != AttemptInterrupted {
								return nil
							}
							a.ArtifactManifestVersion = normalized.ArtifactManifestVersion
							a.ArtifactManifestPath = normalized.ArtifactManifestPath
							if strings.TrimSpace(a.ChangedFilesPath) == "" {
								a.ChangedFilesPath = normalized.ChangedFilesPath
							}
							if strings.TrimSpace(a.DiffPatchPath) == "" {
								a.DiffPatchPath = normalized.DiffPatchPath
							}
							if strings.TrimSpace(a.ReviewCommentsPath) == "" {
								a.ReviewCommentsPath = normalized.ReviewCommentsPath
							}
							return nil
						})
					} else {
						_ = r.Store.AppendEvent(Event{
							TaskID:    task.ID,
							AttemptID: attemptID,
							Type:      "attempt.artifact_manifest.failed",
							Message:   "artifact manifest write failed",
							Data: map[string]any{
								"error": manifestErr.Error(),
							},
						})
					}

					if r.OnAttemptFinished != nil {
						func() {
							defer func() { _ = recover() }()
							r.OnAttemptFinished(context.Background(), updatedTask, normalized)
						}()
					}
				}
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

	r.mu.Lock()
	r.queues[ws] = append(r.queues[ws], taskID)
	r.mu.Unlock()
	r.signal()
	return nil
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
	latestStatus := latest.Status

	switch latestStatus {
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

		finalAttempt := updated.LatestAttempt()
		if finalAttempt != nil && finalAttempt.ID == attemptID {
			normalized := *finalAttempt
			manifestErr := EnsureArtifactManifestV1(r.Store.TasksDir(), updated, &normalized)
			if manifestErr == nil {
				updated, _ = r.Store.UpdateTask(taskID, func(tk *Task) error {
					a := tk.LatestAttempt()
					if a == nil || a.ID != attemptID || a.Status != AttemptCanceled {
						return nil
					}
					a.ArtifactManifestVersion = normalized.ArtifactManifestVersion
					a.ArtifactManifestPath = normalized.ArtifactManifestPath
					return nil
				})
			} else {
				_ = r.Store.AppendEvent(Event{
					TaskID:    taskID,
					AttemptID: attemptID,
					Type:      "attempt.artifact_manifest.failed",
					Message:   "artifact manifest write failed",
					Data: map[string]any{
						"error": manifestErr.Error(),
					},
				})
			}
			if r.OnAttemptFinished != nil {
				func() {
					defer func() { _ = recover() }()
					r.OnAttemptFinished(context.Background(), updated, normalized)
				}()
			}
		}
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

	case AttemptFailed, AttemptLimitExceeded, AttemptTimedOut, AttemptInterrupted:
		now := Now()
		updated, err := r.Store.UpdateTask(taskID, func(tk *Task) error {
			a := tk.LatestAttempt()
			if a == nil || a.ID != attemptID || a.Status != latestStatus {
				return nil
			}
			a.Status = AttemptCanceled
			if a.FinishedAt == nil {
				a.FinishedAt = &now
			}
			return nil
		})
		if err != nil {
			return Task{}, err
		}
		_ = r.Store.AppendEvent(Event{
			TaskID:    taskID,
			AttemptID: attemptID,
			Type:      "attempt.canceled",
			Message:   "Attempt canceled after failure",
		})

		finalAttempt := updated.LatestAttempt()
		if finalAttempt != nil && finalAttempt.ID == attemptID {
			normalized := *finalAttempt
			manifestErr := EnsureArtifactManifestV1(r.Store.TasksDir(), updated, &normalized)
			if manifestErr == nil {
				updated, _ = r.Store.UpdateTask(taskID, func(tk *Task) error {
					a := tk.LatestAttempt()
					if a == nil || a.ID != attemptID || a.Status != AttemptCanceled {
						return nil
					}
					a.ArtifactManifestVersion = normalized.ArtifactManifestVersion
					a.ArtifactManifestPath = normalized.ArtifactManifestPath
					if strings.TrimSpace(a.ChangedFilesPath) == "" {
						a.ChangedFilesPath = normalized.ChangedFilesPath
					}
					if strings.TrimSpace(a.DiffPatchPath) == "" {
						a.DiffPatchPath = normalized.DiffPatchPath
					}
					if strings.TrimSpace(a.ReviewCommentsPath) == "" {
						a.ReviewCommentsPath = normalized.ReviewCommentsPath
					}
					return nil
				})
			} else {
				_ = r.Store.AppendEvent(Event{
					TaskID:    taskID,
					AttemptID: attemptID,
					Type:      "attempt.artifact_manifest.failed",
					Message:   "artifact manifest write failed",
					Data: map[string]any{
						"error": manifestErr.Error(),
					},
				})
			}
			if r.OnAttemptFinished != nil {
				func() {
					defer func() { _ = recover() }()
					r.OnAttemptFinished(context.Background(), updated, normalized)
				}()
			}
		}
		return updated, nil

	default:
		return task, nil
	}
}

func (r *TaskRunner) Resume(taskID string, reviewNotes string) (Task, error) {
	return r.ResumeWithSource(taskID, reviewNotes, "resume")
}

func (r *TaskRunner) ResumeWithSource(taskID string, reviewNotes string, source string) (Task, error) {
	src := strings.TrimSpace(source)
	if src == "" {
		src = "resume"
	}
	return r.enqueueAttempt(taskID, reviewNotes, src, false)
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

func (r *TaskRunner) signal() {
	if r == nil || r.ctx == nil || r.ctx.Err() != nil {
		return
	}
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (r *TaskRunner) runScheduler() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-r.notify:
			// attempt schedule loop
		case <-ticker.C:
			// periodic re-evaluation (governance changes, pause/resume, etc.)
		}

		for {
			workspace, taskID := r.pickNextRunnable()
			if workspace == "" || taskID == "" {
				break
			}

			r.wg.Add(1)
			go func(ws, id string) {
				defer r.wg.Done()
				r.processTask(ws, id)
				r.mu.Lock()
				delete(r.runningWorkspaces, ws)
				r.mu.Unlock()
				r.signal()
			}(workspace, taskID)
		}

		// Best-effort deferred events (low noise): record why a queued task isn't running.
		r.emitDeferredEvents(Now())
	}
}

func (r *TaskRunner) emitDeferredEvents(now time.Time) {
	if r == nil || r.Store == nil {
		return
	}
	now = now.UTC()

	r.mu.Lock()
	if !r.lastDeferredScanAt.IsZero() && now.Sub(r.lastDeferredScanAt) < 1*time.Second {
		r.mu.Unlock()
		return
	}
	r.lastDeferredScanAt = now
	if r.deferEventLast == nil {
		r.deferEventLast = make(map[string]time.Time)
	}
	r.mu.Unlock()

	snap := r.GovernanceSnapshot(now)
	runningCount := len(snap.RunningWorkspaces)

	for ws, wsSnap := range snap.Workspaces {
		if wsSnap.Decision != ScheduleDecisionDeferred || wsSnap.QueuedTasks == 0 {
			continue
		}

		reason := wsSnap.ReasonCode
		switch reason {
		case ScheduleReasonGlobalCap, ScheduleReasonWorkspacePaused:
			// ok
		default:
			continue
		}

		r.mu.Lock()
		q := r.queues[ws]
		taskID := ""
		if len(q) > 0 {
			taskID = q[0]
		}
		r.mu.Unlock()
		if strings.TrimSpace(taskID) == "" {
			continue
		}

		key := taskID + ":" + reason
		r.mu.Lock()
		last, ok := r.deferEventLast[key]
		if ok && now.Sub(last) < 10*time.Second {
			r.mu.Unlock()
			continue
		}
		r.deferEventLast[key] = now
		r.mu.Unlock()

		attemptID := ""
		if task, err := r.Store.GetTask(taskID); err == nil {
			if a := task.LatestAttempt(); a != nil {
				attemptID = a.ID
			}
		}

		_ = r.Store.AppendEvent(Event{
			TaskID:    taskID,
			AttemptID: attemptID,
			Type:      "scheduler.deferred",
			Message:   "Task deferred by governance",
			Data: map[string]any{
				"workspace":              ws,
				"reason_code":            reason,
				"max_running_workspaces": snap.Global.MaxRunningWorkspaces,
				"running_workspaces":     runningCount,
				"deferred_workspaces":    snap.DeferredWorkspaces,
				"paused_workspaces":      snap.PausedWorkspaces,
			},
		})
	}
}

func (r *TaskRunner) pickNextRunnable() (string, string) {
	if r == nil || r.Store == nil {
		return "", ""
	}

	g, _ := r.Store.GetGovernance()

	r.mu.Lock()
	defer r.mu.Unlock()

	maxWS := g.Global.MaxRunningWorkspaces
	if maxWS > 0 {
		running := 0
		for _, v := range r.runningWorkspaces {
			if v {
				running++
			}
		}
		if running >= maxWS {
			return "", ""
		}
	}

	var eligible []string

	bestWS := ""
	bestScore := -1 << 30
	for ws, q := range r.queues {
		if len(q) == 0 {
			delete(r.workspaceAges, ws)
			continue
		}
		if r.runningWorkspaces[ws] {
			continue
		}
		p := g.Workspaces[ws]
		if p.Paused {
			r.workspaceAges[ws] = 0
			continue
		}

		eligible = append(eligible, ws)

		age := r.workspaceAges[ws]
		score := p.Priority + age
		if bestWS == "" || score > bestScore || (score == bestScore && ws < bestWS) {
			bestWS = ws
			bestScore = score
		}
	}
	if bestWS == "" {
		return "", ""
	}

	for _, ws := range eligible {
		if ws == bestWS {
			r.workspaceAges[ws] = 0
			continue
		}
		age := r.workspaceAges[ws]
		if age < 1_000_000 {
			age++
		}
		r.workspaceAges[ws] = age
	}

	taskID := r.queues[bestWS][0]
	r.queues[bestWS] = r.queues[bestWS][1:]
	r.runningWorkspaces[bestWS] = true
	return bestWS, taskID
}

func (r *TaskRunner) runSchedules() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case now := <-ticker.C:
			r.runSchedulesOnce(now)
		}
	}
}

func (r *TaskRunner) runSchedulesOnce(now time.Time) {
	if r == nil || r.Store == nil {
		return
	}
	now = now.UTC()

	g, err := r.Store.GetGovernance()
	if err != nil {
		return
	}
	if len(g.Schedules) == 0 {
		return
	}

	type scheduleUpdate struct {
		MisfirePolicy      string
		NextRunAt          time.Time
		UpdatedAt          time.Time
		LastTriggerKey     string
		LastEnqueueAt      time.Time
		LastEnqueueError   string
		LastEnqueueErrorAt time.Time
	}

	const maxCatchUpRuns = 25
	updates := make(map[string]scheduleUpdate, len(g.Schedules))

	for _, sc := range g.Schedules {
		if !sc.Enabled {
			continue
		}
		if strings.TrimSpace(sc.Workspace) == "" || strings.TrimSpace(sc.Prompt) == "" {
			continue
		}
		if sc.EverySeconds <= 0 {
			continue
		}

		userID := strings.TrimSpace(sc.UserID)
		if userID == "" {
			userID = "local"
		}
		title := strings.TrimSpace(sc.Title)
		if title == "" {
			title = "Scheduled task"
		}

		sc.MisfirePolicy = normalizeMisfirePolicy(sc.MisfirePolicy)
		windows, nextAfter, due := dueWindowsForSchedule(sc, now, maxCatchUpRuns)
		if !due {
			continue
		}

		up := scheduleUpdate{
			MisfirePolicy:  sc.MisfirePolicy,
			NextRunAt:      nextAfter,
			UpdatedAt:      now,
			LastTriggerKey: strings.TrimSpace(sc.LastTriggerKey),
			LastEnqueueAt:  sc.LastEnqueueAt.UTC(),
		}

		if len(windows) == 0 {
			// Skip policy misfire: advance without enqueuing, and clear stale errors.
			up.LastEnqueueError = ""
			up.LastEnqueueErrorAt = time.Time{}
			updates[sc.ID] = up
			continue
		}

		for _, windowStart := range windows {
			windowStart = windowStart.UTC()
			triggerKey := scheduleTriggerKey(sc.ID, windowStart)
			taskID := scheduleTaskID(sc.ID, windowStart)

			task, created, createErr := r.Store.CreateTaskWithID(
				taskID,
				userID,
				sc.Workspace,
				title,
				sc.Prompt,
				sc.ModelID,
				ResolveLimits(sc.Limits),
			)
			if createErr != nil {
				up.NextRunAt = windowStart
				up.LastEnqueueError = createErr.Error()
				up.LastEnqueueErrorAt = now
				updates[sc.ID] = up
				break
			}

			up.LastTriggerKey = triggerKey
			up.LastEnqueueAt = now
			up.LastEnqueueError = ""
			up.LastEnqueueErrorAt = time.Time{}

			if created {
				attemptID := ""
				if a := task.LatestAttempt(); a != nil {
					attemptID = a.ID
				}
				_ = r.Store.AppendEvent(Event{
					TaskID:    task.ID,
					AttemptID: attemptID,
					Type:      "schedule.triggered",
					Message:   "Scheduled task enqueued",
					Data: map[string]any{
						"schedule_id":     sc.ID,
						"trigger_key":     triggerKey,
						"window_start_ts": windowStart.Format(time.RFC3339Nano),
						"misfire_policy":  sc.MisfirePolicy,
					},
				})
			}

			// Best-effort: if the runner is running, enqueue so it executes.
			_ = r.Enqueue(task.ID)
			updates[sc.ID] = up
		}
	}

	if len(updates) == 0 {
		return
	}

	_, _ = r.Store.UpdateGovernance(func(g *QueueGovernance) error {
		for i := range g.Schedules {
			sc := g.Schedules[i]
			up, ok := updates[sc.ID]
			if !ok {
				continue
			}

			sc.MisfirePolicy = strings.TrimSpace(up.MisfirePolicy)
			sc.NextRunAt = up.NextRunAt.UTC()
			sc.UpdatedAt = up.UpdatedAt.UTC()
			sc.LastTriggerKey = strings.TrimSpace(up.LastTriggerKey)
			sc.LastEnqueueAt = up.LastEnqueueAt.UTC()
			sc.LastEnqueueError = strings.TrimSpace(up.LastEnqueueError)
			sc.LastEnqueueErrorAt = up.LastEnqueueErrorAt.UTC()
			g.Schedules[i] = sc
		}
		return nil
	})
}

func (r *TaskRunner) processTask(workspace string, taskID string) {
	task, err := r.Store.GetTask(taskID)
	if err != nil {
		return
	}
	if task.Workspace != workspace {
		_ = r.Store.AppendEvent(Event{
			TaskID:  taskID,
			Type:    "scheduler.skipped",
			Message: "Task skipped by scheduler (workspace mismatch)",
			Data: map[string]any{
				"workspace":              workspace,
				"task_workspace":         task.Workspace,
				"reason_code":            "workspace_mismatch",
				"max_running_workspaces": func() int { g, _ := r.Store.GetGovernance(); return g.Global.MaxRunningWorkspaces }(),
			},
		})
		return
	}

	latest := task.LatestAttempt()
	if latest == nil {
		_ = r.Store.AppendEvent(Event{TaskID: taskID, Type: "task.error", Message: "task has no attempts"})
		return
	}
	if latest.Status != AttemptQueued {
		_ = r.Store.AppendEvent(Event{
			TaskID:    taskID,
			AttemptID: latest.ID,
			Type:      "scheduler.skipped",
			Message:   "Task skipped by scheduler (attempt not queued)",
			Data: map[string]any{
				"reason_code": "attempt_not_queued",
				"status":      string(latest.Status),
			},
		})
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
		Type:      "scheduler.picked",
		Message:   "Task picked for execution",
		Data: func() map[string]any {
			g, _ := r.Store.GetGovernance()
			p := g.Workspaces[workspace]
			return map[string]any{
				"workspace":              workspace,
				"priority":               p.Priority,
				"max_running_workspaces": g.Global.MaxRunningWorkspaces,
				"reason_code":            ScheduleReasonPicked,
			}
		}(),
	})

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
	ranAttempt.WorktreeMode = strings.TrimSpace(result.WorktreeMode)
	ranAttempt.WorktreeCleanupStatus = strings.TrimSpace(result.WorktreeCleanupStatus)
	ranAttempt.WorktreeCleanupError = strings.TrimSpace(result.WorktreeCleanupError)
	ranAttempt.WorktreeCleanupHint = strings.TrimSpace(result.WorktreeCleanupHint)
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
		if r.Store != nil && err == nil {
			_ = r.Store.AppendEvent(Event{
				TaskID:    taskID,
				AttemptID: attemptID,
				Type:      "attempt.observer.decided",
				Message:   "Outcome observer decided",
				Data: map[string]any{
					"pass":               decision.Pass,
					"reason":             decision.Reason,
					"next_steps":         decision.NextSteps,
					"questions_for_user": decision.QuestionsForUser,
				},
			})
		}

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

	manifestErr := error(nil)
	if r.Store != nil {
		manifestErr = EnsureArtifactManifestV1(r.Store.TasksDir(), updated, &ranAttempt)
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
	if manifestErr != nil {
		_ = r.Store.AppendEvent(Event{
			TaskID:    taskID,
			AttemptID: attemptID,
			Type:      "attempt.artifact_manifest.failed",
			Message:   "artifact manifest write failed",
			Data: map[string]any{
				"error": manifestErr.Error(),
			},
		})
	}

	if r.OnAttemptFinished != nil {
		func() {
			defer func() { _ = recover() }()
			finalAttempt := saved.LatestAttempt()
			if finalAttempt != nil && finalAttempt.ID == attemptID {
				r.OnAttemptFinished(context.Background(), saved, *finalAttempt)
			} else {
				r.OnAttemptFinished(context.Background(), saved, ranAttempt)
			}
		}()
	}

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
