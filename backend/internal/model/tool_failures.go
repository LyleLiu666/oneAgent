// Package model contains database models for tool call failures.
package model

import "time"

// ToolCallFailure stores a failed tool invocation for later analysis.
type ToolCallFailure struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SessionID  string    `gorm:"type:varchar(36);index" json:"session_id"`
	UserID     string    `gorm:"type:varchar(255);index" json:"user_id"`
	ProviderID string    `gorm:"type:varchar(36);index" json:"provider_id"`
	ModelID    string    `gorm:"type:varchar(36);index" json:"model_id"`
	ModelName  string    `gorm:"type:varchar(255)" json:"model_name"`
	ToolName   string    `gorm:"type:varchar(255);index" json:"tool_name"`
	ToolCallID string    `gorm:"type:varchar(255)" json:"tool_call_id"`
	Arguments  string    `gorm:"type:text" json:"arguments"`
	Error      string    `gorm:"type:text" json:"error"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName returns the table name for ToolCallFailure.
func (ToolCallFailure) TableName() string {
	return "tool_call_failures"
}
