package workledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LearningJobStatus string

const (
	LearningJobStatusQueued    LearningJobStatus = "queued"
	LearningJobStatusRunning   LearningJobStatus = "running"
	LearningJobStatusSucceeded LearningJobStatus = "succeeded"
	LearningJobStatusFailed    LearningJobStatus = "failed"
)

type LearningJobStats struct {
	SuggestionsCreated       int `json:"suggestions_created"`
	CompressibilityEvaluated int `json:"compressibility_evaluated,omitempty"`
	CompressibilitySkipped   int `json:"compressibility_skipped,omitempty"`
}

type LearningJob struct {
	JobID       string `json:"job_id"`
	PrincipalID string `json:"principal_id"`
	DayKey      string `json:"day_key"`

	Status LearningJobStatus `json:"status"`

	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`

	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`

	Stats LearningJobStats `json:"stats,omitempty"`
}

func (s *Store) LearningJobsDir(principalID string) string {
	return filepath.Join(s.baseDir, "learning_jobs", strings.TrimSpace(principalID))
}

func (s *Store) LearningJobPath(principalID, dayKey string) string {
	return filepath.Join(s.LearningJobsDir(principalID), strings.TrimSpace(dayKey)+".json")
}

func (s *Store) GetLearningJob(principalID, dayKey string) (LearningJob, error) {
	if s == nil {
		return LearningJob{}, errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	dayKey = strings.TrimSpace(dayKey)
	if principalID == "" || dayKey == "" {
		return LearningJob{}, errors.New("principal_id and day_key are required")
	}
	path := s.LearningJobPath(principalID, dayKey)
	data, err := os.ReadFile(path)
	if err != nil {
		return LearningJob{}, err
	}
	var job LearningJob
	if err := json.Unmarshal(data, &job); err != nil {
		return LearningJob{}, fmt.Errorf("decode job: %w", err)
	}
	return job, nil
}

func (s *Store) WriteLearningJob(job LearningJob) error {
	if s == nil {
		return errors.New("store is nil")
	}
	if strings.TrimSpace(job.PrincipalID) == "" || strings.TrimSpace(job.DayKey) == "" {
		return errors.New("principal_id and day_key are required")
	}
	dir := s.LearningJobsDir(job.PrincipalID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir jobs dir: %w", err)
	}
	path := s.LearningJobPath(job.PrincipalID, job.DayKey)
	return writeJSONAtomic(path, job, 0o600)
}
