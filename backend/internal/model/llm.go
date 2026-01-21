// Package model contains database models for LLM providers and calls.
package model

import (
	"time"

	"gorm.io/gorm"
)

// LLMProvider represents an API provider configuration.
type LLMProvider struct {
	ID           string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
	UserID       string         `gorm:"type:varchar(255);index" json:"user_id"`
	Name         string         `gorm:"type:varchar(255)" json:"name"`
	ProviderType string         `gorm:"type:varchar(50);index" json:"provider_type"`
	BaseURL      string         `gorm:"type:varchar(500)" json:"base_url"`
	APIKey       string         `gorm:"type:text" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Models []LLMModel `gorm:"foreignKey:ProviderID" json:"models,omitempty"`
}

// TableName returns the table name for LLMProvider.
func (LLMProvider) TableName() string {
	return "llm_providers"
}

// LLMModel represents a model attached to a provider.
type LLMModel struct {
	ID            string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
	ProviderID    string         `gorm:"type:varchar(36);index" json:"provider_id"`
	UserID        string         `gorm:"type:varchar(255);index" json:"user_id"`
	Name          string         `gorm:"type:varchar(255)" json:"name"`
	Model         string         `gorm:"type:varchar(255)" json:"model"`
	IsDefault     bool           `gorm:"default:false" json:"is_default"`
	EnableKVCache bool           `gorm:"default:true" json:"enable_kv_cache"`
	Options       JSONB          `gorm:"type:jsonb" json:"options,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	Provider LLMProvider `gorm:"foreignKey:ProviderID" json:"provider,omitempty"`
}

// TableName returns the table name for LLMModel.
func (LLMModel) TableName() string {
	return "llm_models"
}

// LLMCall stores a full message payload for each LLM request.
type LLMCall struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	SessionID        string    `gorm:"type:varchar(36);index" json:"session_id"`
	UserID           string    `gorm:"type:varchar(255);index" json:"user_id"`
	ProviderID       string    `gorm:"type:varchar(36);index" json:"provider_id"`
	ModelID          string    `gorm:"type:varchar(36);index" json:"model_id"`
	ModelName        string    `gorm:"type:varchar(255)" json:"model_name"`
	Messages         string    `gorm:"type:text" json:"messages"`
	Response         string    `gorm:"type:text" json:"response,omitempty"`
	Error            string    `gorm:"type:text" json:"error,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CreatedAt        time.Time `json:"created_at"`
}

// TableName returns the table name for LLMCall.
func (LLMCall) TableName() string {
	return "llm_calls"
}
