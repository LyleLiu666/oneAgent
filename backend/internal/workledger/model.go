package workledger

import "time"

type ReceiptKind string

const (
	ReceiptKindSubagentRun ReceiptKind = "subagent_run"
)

type ReceiptStatus string

const (
	ReceiptStatusSucceeded   ReceiptStatus = "succeeded"
	ReceiptStatusFailed      ReceiptStatus = "failed"
	ReceiptStatusCanceled    ReceiptStatus = "canceled"
	ReceiptStatusTimedOut    ReceiptStatus = "timed_out"
	ReceiptStatusInterrupted ReceiptStatus = "interrupted"
)

type ReceiptArtifacts struct {
	FindingsPath   string `json:"findings_path,omitempty"`
	TraceLogPath   string `json:"trace_log_path,omitempty"`
	TestReportPath string `json:"test_report_path,omitempty"`
	DiffRef        string `json:"diff_ref,omitempty"`
}

type ReceiptSignals struct {
	DurationMs int64 `json:"duration_ms,omitempty"`
}

type Receipt struct {
	ReceiptID string `json:"receipt_id"`

	PrincipalID   string `json:"principal_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`

	Kind   ReceiptKind   `json:"kind"`
	Status ReceiptStatus `json:"status"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`

	Summary   string          `json:"summary"`
	Artifacts ReceiptArtifacts `json:"artifacts,omitempty"`
	Signals   ReceiptSignals   `json:"signals,omitempty"`
}

