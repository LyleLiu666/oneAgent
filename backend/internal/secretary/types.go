package secretary

import "time"

type State struct {
	CursorMessageID uint       `json:"cursor_message_id"`
	TriageRuns      []TriageRun `json:"triage_runs,omitempty"`
}

type TriageRun struct {
	FromCursor     uint      `json:"from_cursor"`
	ToMessageID    uint      `json:"to_message_id"`
	InputMessageIDs []uint    `json:"input_message_ids,omitempty"`
	SummaryMessageID uint     `json:"summary_message_id,omitempty"`
	SummaryMessage  string   `json:"summary_message,omitempty"`
	CreatedTaskIDs  []string `json:"created_task_ids,omitempty"`
	Questions       []string `json:"questions,omitempty"`
	WorkspacesCreated []string `json:"workspaces_created,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
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
	Questions         []string
	WorkspacesCreated []string
}

type StateResult struct {
	CursorMessageID uint       `json:"cursor_message_id"`
	TriageRuns      []TriageRun `json:"triage_runs,omitempty"`
}

