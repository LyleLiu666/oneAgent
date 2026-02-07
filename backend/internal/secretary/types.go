package secretary

import "time"

type State struct {
	CursorMessageID uint        `json:"cursor_message_id"`
	TriageRuns      []TriageRun `json:"triage_runs,omitempty"`

	RecoveryFocus *RecoveryFocus `json:"recovery_focus,omitempty"`
}

type RecoveryFocus struct {
	TaskID    string `json:"task_id"`
	AttemptID string `json:"attempt_id"`
}

type TriageRun struct {
	FromCursor        uint           `json:"from_cursor"`
	ToMessageID       uint           `json:"to_message_id"`
	InputMessageIDs   []uint         `json:"input_message_ids,omitempty"`
	SummaryMessageID  uint           `json:"summary_message_id,omitempty"`
	SummaryMessage    string         `json:"summary_message,omitempty"`
	CreatedTaskIDs    []string       `json:"created_task_ids,omitempty"`
	CanceledTaskIDs   []string       `json:"canceled_task_ids,omitempty"`
	ResumedTaskIDs    []string       `json:"resumed_task_ids,omitempty"`
	Questions         []string       `json:"questions,omitempty"`
	WorkspacesCreated []string       `json:"workspaces_created,omitempty"`
	SearchContext     *SearchContext `json:"search_context,omitempty"`
	CreatedAt         time.Time      `json:"created_at,omitempty"`
}

type SearchContext struct {
	DefaultRoot string   `json:"default_root,omitempty"`
	FinalPhase  string   `json:"final_phase,omitempty"`
	Expanded    bool     `json:"expanded,omitempty"`
	TimeoutHit  bool     `json:"timeout_hit,omitempty"`
	ScannedRoot []string `json:"scanned_root,omitempty"`
	Candidates  []string `json:"candidates,omitempty"`
	Omitted     int      `json:"omitted,omitempty"`
}

type InboxAppendResult struct {
	SessionID    string
	MessageID    uint
	AckMessageID uint
	AckText      string
}

type TriageResult struct {
	SummaryMessage    string
	SummaryMessageID  uint
	CursorMessageID   uint
	CreatedTaskIDs    []string
	CanceledTaskIDs   []string
	ResumedTaskIDs    []string
	Questions         []string
	WorkspacesCreated []string
}

type StateResult struct {
	CursorMessageID uint        `json:"cursor_message_id"`
	TriageRuns      []TriageRun `json:"triage_runs,omitempty"`

	RecoveryFocus *RecoveryFocus `json:"recovery_focus,omitempty"`
}
