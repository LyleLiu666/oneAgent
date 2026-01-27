package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

// RunDailyJob executes a single daily learning job for the given principal.
// It is idempotent: repeated runs for the same day will not create duplicate suggestions
// because suggestions are deduped by evidence signature within the day.
func RunDailyJob(ctx context.Context, store *workledger.Store, principalID string, now time.Time) (workledger.LearningJob, error) {
	if store == nil {
		return workledger.LearningJob{}, fmt.Errorf("store is nil")
	}
	if principalID == "" {
		principalID = "local"
	}

	dayKey := workledger.DayKey(now)

	// If already succeeded today, no-op.
	if existing, err := store.GetLearningJob(principalID, dayKey); err == nil {
		if existing.Status == workledger.LearningJobStatusSucceeded {
			return existing, nil
		}
	}

	job := workledger.LearningJob{
		JobID:       fmt.Sprintf("%s-%s", principalID, dayKey),
		PrincipalID: principalID,
		DayKey:      dayKey,
		Status:      workledger.LearningJobStatusRunning,
		StartedAt:   time.Now().UTC(),
	}
	_ = store.WriteLearningJob(job)

	created, err := store.GenerateSuggestionsV1(ctx, workledger.GenerateSuggestionsInput{
		PrincipalID:  principalID,
		LookbackDays: 7,
		Count:        3,
		DayKey:       dayKey,
	})
	if err != nil {
		job.Status = workledger.LearningJobStatusFailed
		job.Error = err.Error()
		job.FinishedAt = time.Now().UTC()
		_ = store.WriteLearningJob(job)
		return job, err
	}

	job.Status = workledger.LearningJobStatusSucceeded
	job.FinishedAt = time.Now().UTC()
	job.Stats = workledger.LearningJobStats{
		SuggestionsCreated: len(created),
	}
	_ = store.WriteLearningJob(job)
	return job, nil
}
