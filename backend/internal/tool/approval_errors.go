package tool

import "fmt"

type ApprovalRequiredError struct {
	ApprovalID string
	ToolID     string
	ScopeID    string
}

func (e *ApprovalRequiredError) Error() string {
	if e == nil {
		return "approval required"
	}
	if e.ToolID != "" && e.ApprovalID != "" {
		return fmt.Sprintf("approval required: tool=%s approval_id=%s", e.ToolID, e.ApprovalID)
	}
	if e.ToolID != "" {
		return fmt.Sprintf("approval required: tool=%s", e.ToolID)
	}
	if e.ApprovalID != "" {
		return fmt.Sprintf("approval required: approval_id=%s", e.ApprovalID)
	}
	return "approval required"
}

type ApprovalDeniedError struct {
	ApprovalID string
	ToolID     string
	ScopeID    string
	Reason     string
}

func (e *ApprovalDeniedError) Error() string {
	if e == nil {
		return "approval denied"
	}
	if e.ToolID != "" && e.ApprovalID != "" {
		return fmt.Sprintf("approval denied: tool=%s approval_id=%s", e.ToolID, e.ApprovalID)
	}
	if e.ToolID != "" {
		return fmt.Sprintf("approval denied: tool=%s", e.ToolID)
	}
	if e.ApprovalID != "" {
		return fmt.Sprintf("approval denied: approval_id=%s", e.ApprovalID)
	}
	return "approval denied"
}
