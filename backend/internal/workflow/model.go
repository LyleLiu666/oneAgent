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

