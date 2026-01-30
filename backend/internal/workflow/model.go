package workflow

import "time"

type Workflow struct {
	WorkflowID    string    `json:"workflow_id"`
	WorkspaceRoot string    `json:"workspace_root"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	NodeID string `json:"node_id"`
	Title  string `json:"title,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type WorkflowVersion struct {
	VersionID  string    `json:"version_id"`
	WorkflowID string    `json:"workflow_id"`
	Published  bool      `json:"published"`
	PublishedAt time.Time `json:"published_at,omitempty"`

	Graph Graph `json:"graph"`

	CreatedAt time.Time `json:"created_at"`
}

type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

type NodeStatus string

const (
	NodeStatusQueued    NodeStatus = "queued"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusSucceeded NodeStatus = "succeeded"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusCanceled  NodeStatus = "canceled"
	NodeStatusSkipped   NodeStatus = "skipped"
)

type Artifact struct {
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"` // e.g. "file"
}

type ArtifactManifest struct {
	Artifacts []Artifact `json:"artifacts"`
}

type NodeRun struct {
	NodeID string     `json:"node_id"`
	Status NodeStatus `json:"status"`

	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`

	Artifacts ArtifactManifest `json:"artifacts,omitempty"`

	HardGateReportPath string `json:"hard_gate_report_path,omitempty"`
	SoftGateReportPath string `json:"soft_gate_report_path,omitempty"`
}

type WorkflowRun struct {
	RunID         string `json:"run_id"`
	WorkflowID    string `json:"workflow_id"`
	VersionID     string `json:"version_id"`
	WorkspaceRoot string `json:"workspace_root"`

	GraphSnapshot Graph            `json:"graph_snapshot"`
	Inputs        map[string]any   `json:"inputs,omitempty"`
	NodeRuns      map[string]NodeRun `json:"node_runs,omitempty"`

	Status     RunStatus `json:"status"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
