// Package model contains all database models for the application.
//
// MULTI-MODULE SUPPORT:
// This application supports multiple chat modules (e.g., "assistant", "reader", "guide").
// Each module shares the same tables but is distinguished by the Module field.
// Query by module: db.Where("module = ?", "reader").Find(&sessions)
//
// EXTENSION POINTS:
//   - Add new model types in this file or create separate files per domain
//   - For completely separate table schemas, create module-specific files
//     (e.g., reader_chat.go with ReaderChatSession struct)
//
// DATABASE TABLES:
// - users: User accounts from OAuth (Keycloak)
// - chat_sessions: Conversation sessions (multi-module)
// - chat_messages: Individual messages with trace data
package model

import (
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// USER MODEL
// ============================================================================

// User represents a user from Keycloak or other OAuth provider.
// EXTENSION: Add profile fields, preferences, or link to other user data.
type User struct {
	ID        string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Email     string         `gorm:"uniqueIndex;type:varchar(255)" json:"email"`
	Username  string         `gorm:"type:varchar(255)" json:"username"`
	Name      string         `gorm:"type:varchar(255)" json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for User.
func (User) TableName() string {
	return "users"
}

// ============================================================================
// CHAT SESSION MODEL
// ============================================================================

// ChatSession represents a conversation session.
//
// MULTI-MODULE USAGE:
// Set the Module field to categorize sessions by feature area.
// Built-in modules: "assistant" (default), "reader", "guide"
// EXTENSION: Add your own module identifiers as needed.
//
// METADATA USAGE:
// Store session-specific data in the Metadata field (JSONB).
// Examples: book_id for reader, custom_prompt for guide, etc.
type ChatSession struct {
	ID        string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
	UserID    string         `gorm:"type:varchar(255);index" json:"user_id"`
	Title     string         `gorm:"type:varchar(500)" json:"title"`
	Module    string         `gorm:"type:varchar(50);index;default:'assistant'" json:"module"` // EXTENSION: Module identifier
	Metadata  JSONB          `gorm:"type:jsonb" json:"metadata,omitempty"`                     // EXTENSION: Custom session data
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Messages []ChatMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// TableName returns the table name for ChatSession.
func (ChatSession) TableName() string {
	return "chat_sessions"
}

// ============================================================================
// CHAT MESSAGE MODEL
// ============================================================================

// MessageRole defines the role of a message sender.
const (
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
	MessageRoleSystem    = "system"
	MessageRoleTool      = "tool" // EXTENSION: For tool/function call results
)

// MessageType defines the type of message.
// EXTENSION: Add new message types for complex workflows.
const (
	MessageTypeText       = "text"        // Normal text message
	MessageTypeToolCall   = "tool_call"   // Tool/function invocation
	MessageTypeToolResult = "tool_result" // Tool execution result
)

// ChatMessage represents a single message in a conversation.
//
// TRACE DATA:
// The Trace field stores structured trace data (LangSmith-like).
// Use TraceDataJSON type for proper GORM serialization.
//
// PARENT-CHILD MESSAGES:
// Use ParentID to create message hierarchies (e.g., tool calls and results).
// A tool_result message should have ParentID pointing to the tool_call message.
type ChatMessage struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	SessionID string         `gorm:"type:varchar(36);index" json:"session_id"`
	ParentID  *uint          `gorm:"index" json:"parent_id,omitempty"`            // EXTENSION: For sub-messages
	Role      string         `gorm:"type:varchar(20)" json:"role"`                // MessageRole constant
	Type      string         `gorm:"type:varchar(20);default:'text'" json:"type"` // MessageType constant
	Content   string         `gorm:"type:text" json:"content"`
	Trace     TraceDataJSON  `gorm:"type:jsonb" json:"trace,omitempty"` // EXTENSION: Structured trace data
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Children []ChatMessage `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

// TableName returns the table name for ChatMessage.
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// GetSessionsByModule retrieves sessions for a user filtered by module.
func GetSessionsByModule(db *gorm.DB, userID, module string) ([]ChatSession, error) {
	var sessions []ChatSession
	err := db.Where("user_id = ? AND module = ?", userID, module).
		Order("updated_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// GetSessionWithMessages retrieves a session with all its messages.
func GetSessionWithMessages(db *gorm.DB, sessionID, userID string) (*ChatSession, error) {
	var session ChatSession
	err := db.Where("id = ? AND user_id = ?", sessionID, userID).
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}
