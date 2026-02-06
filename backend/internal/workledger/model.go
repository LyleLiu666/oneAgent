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

type EvidenceCompleteness string

const (
	EvidenceCompletenessComplete     EvidenceCompleteness = "complete"
	EvidenceCompletenessPartial      EvidenceCompleteness = "partial"
	EvidenceCompletenessInsufficient EvidenceCompleteness = "insufficient"
)

type ReceiptArtifacts struct {
	FindingsPath       string `json:"findings_path,omitempty"`
	TraceLogPath       string `json:"trace_log_path,omitempty"`
	TestReportPath     string `json:"test_report_path,omitempty"`
	DiffPatchPath      string `json:"diff_patch_path,omitempty"`
	ChangedFilesPath   string `json:"changed_files_path,omitempty"`
	ReviewCommentsPath string `json:"review_comments_path,omitempty"`
	DiffRef            string `json:"diff_ref,omitempty"`

	WorktreeRoot  string `json:"worktree_root,omitempty"`
	BaseCommitSHA string `json:"base_commit_sha,omitempty"`
	BaseRef       string `json:"base_ref,omitempty"`
}

type ReceiptSignals struct {
	DurationMs int64 `json:"duration_ms,omitempty"`

	Calls            int     `json:"calls,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

type Receipt struct {
	ReceiptID string `json:"receipt_id"`

	PrincipalID   string `json:"principal_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`

	Kind   ReceiptKind   `json:"kind"`
	Status ReceiptStatus `json:"status"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`

	Summary                 string               `json:"summary"`
	ArtifactManifestVersion string               `json:"artifact_manifest_version,omitempty"`
	ArtifactManifestPath    string               `json:"artifact_manifest_path,omitempty"`
	EvidenceCompleteness    EvidenceCompleteness `json:"evidence_completeness,omitempty"`
	Artifacts               ReceiptArtifacts     `json:"artifacts,omitempty"`
	Signals                 ReceiptSignals       `json:"signals,omitempty"`
}
