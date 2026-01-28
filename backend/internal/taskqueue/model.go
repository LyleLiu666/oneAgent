package taskqueue

import (
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

type AttemptStatus string

const (
	AttemptQueued      AttemptStatus = "queued"
	AttemptRunning     AttemptStatus = "running"
	AttemptSucceeded   AttemptStatus = "succeeded"
	AttemptFailed      AttemptStatus = "failed"
	AttemptCanceled    AttemptStatus = "canceled"
	AttemptTimedOut    AttemptStatus = "timed_out"
	AttemptInterrupted AttemptStatus = "interrupted"
)

type Limits struct {
	MaxSteps          int `json:"max_steps,omitempty"`
	MaxRuntimeSeconds int `json:"max_runtime_seconds,omitempty"`
}

type ObserverDecision struct {
	Pass     bool     `json:"pass"`
	Reason   string   `json:"reason,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

type Attempt struct {
	ID string `json:"id"`

	Status AttemptStatus `json:"status"`

	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	ResumedFromAttemptID string `json:"resumed_from_attempt_id,omitempty"`

	PrincipalID    string                `json:"principal_id,omitempty"`
	PolicySnapshot *permissions.Snapshot `json:"policy_snapshot,omitempty"`

	RunID        string `json:"run_id,omitempty"`
	Summary      string `json:"summary,omitempty"`
	FindingsPath string `json:"findings_path,omitempty"`
	TraceLogPath string `json:"trace_log_path,omitempty"`

	Observer *ObserverDecision `json:"observer,omitempty"`

	Error string `json:"error,omitempty"`
}

type Task struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	Workspace string `json:"workspace"`
	Title     string `json:"title"`
	Prompt    string `json:"prompt"`
	ModelID   string `json:"model_id,omitempty"`

	Limits Limits `json:"limits,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Attempts []Attempt `json:"attempts"`
}

func (t *Task) LatestAttempt() *Attempt {
	if t == nil || len(t.Attempts) == 0 {
		return nil
	}
	return &t.Attempts[len(t.Attempts)-1]
}

type Event struct {
	TS time.Time `json:"ts"`

	TaskID    string `json:"task_id"`
	AttemptID string `json:"attempt_id,omitempty"`

	Type    string         `json:"type"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// Now returns current time; overrideable in tests.
var Now = time.Now
