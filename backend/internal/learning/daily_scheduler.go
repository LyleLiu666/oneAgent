package learning

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

// StartDailyScheduler starts a best-effort daily learning pipeline.
// It is intentionally isolated from task/subagent delivery and MUST NOT affect it.
//
// v1 policy:
// - single-user principal "local"
// - scheduled at 02:00 local time
// - runs at most once per day (no auto-retry for failed jobs)
// - if the server starts after schedule time and no successful job exists today, run shortly after startup
func StartDailyScheduler(rt *runtime.Runtime) {
	if rt == nil || rt.WorkLedger == nil {
		return
	}
	// Allow disabling in tests/controlled environments to avoid time-dependent behavior.
	if strings.TrimSpace(os.Getenv("ONEAGENT_DISABLE_DAILY_LEARNING")) == "1" {
		return
	}
	eval := &LLMCompressibilityEvaluator{Settings: rt.Settings}
	rt.GoOnceKey("learning.dailyScheduler", func(ctx context.Context) {
		runLoop(ctx, rt.WorkLedger, "local", eval, time.Now)
	})
}

func runLoop(ctx context.Context, store *workledger.Store, principalID string, eval CompressibilityEvaluator, nowFn func() time.Time) {
	const hour = 2

	for {
		now := nowFn()
		runAt := time.Date(now.In(time.Local).Year(), now.In(time.Local).Month(), now.In(time.Local).Day(), hour, 0, 0, 0, time.Local)

		dayKey := workledger.DayKey(now)
		shouldRun := now.After(runAt) || now.Equal(runAt)

		if shouldRun && shouldAutoRunToday(store, principalID, dayKey, now) {
			// run best-effort
			if _, err := RunDailyJobWithEvaluator(ctx, store, principalID, now, eval); err != nil {
				log.Printf("[learning] daily job failed: %v", err)
			}
			// sleep a little to avoid hot looping in case of clock issues
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
			continue
		}

		next := runAt
		if !now.Before(runAt) {
			next = runAt.Add(24 * time.Hour)
		}
		sleep := next.Sub(now)
		if sleep < time.Second {
			sleep = time.Second
		}

		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func shouldAutoRunToday(store *workledger.Store, principalID, dayKey string, now time.Time) bool {
	if store == nil {
		return false
	}
	job, err := store.GetLearningJob(principalID, dayKey)
	if err != nil {
		return true // no job yet
	}
	switch job.Status {
	case workledger.LearningJobStatusSucceeded:
		return false
	case workledger.LearningJobStatusFailed:
		return false // no auto retry; user can run manually
	case workledger.LearningJobStatusRunning:
		// If a previous run crashed, allow rerun when it becomes stale.
		if !job.StartedAt.IsZero() && now.Sub(job.StartedAt) > 6*time.Hour {
			return true
		}
		return false
	default:
		return true
	}
}
