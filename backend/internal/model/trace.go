// Package model contains trace-related data types for LLM observability.
//
// This implements a simplified LangSmith-like trace system for:
// - LLM call tracking (model, tokens, latency)
// - Tool/function call logging
// - Sub-agent orchestration traces
// - Resource references (documents, URLs, etc.)
//
// EXTENSION POINTS:
// - Add new TraceType constants for custom trace types
// - Extend TraceEntry with provider-specific metadata
// - Implement trace exporters (e.g., to LangSmith, OpenTelemetry)
package model

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ============================================================================
// TRACE TYPES - Add new types here for custom agents
// ============================================================================

// TraceType defines the type of trace entry.
// EXTENSION: Add new trace types for your use cases.
const (
	TraceTypeLLMCall    = "llm_call"    // LLM API call
	TraceTypeToolCall   = "tool_call"   // Function/tool invocation
	TraceTypeToolResult = "tool_result" // Tool execution result
	TraceTypeRetrieval  = "retrieval"   // RAG/document retrieval
	TraceTypeSubAgent   = "subagent"    // Sub-agent invocation
	TraceTypeCustom     = "custom"      // Custom trace type
)

// ============================================================================
// TRACE DATA STRUCTURES
// ============================================================================

// TraceEntry represents a single trace step in the execution.
// EXTENSION: Add fields for your specific observability needs.
type TraceEntry struct {
	ID        string         `json:"id"`                 // Unique trace ID
	Type      string         `json:"type"`               // TraceType constant
	Name      string         `json:"name"`               // Human-readable name
	Input     any            `json:"input,omitempty"`    // Input to this step
	Output    any            `json:"output,omitempty"`   // Output from this step
	Error     string         `json:"error,omitempty"`    // Error message if failed
	StartTime time.Time      `json:"start_time"`         // When this step started
	EndTime   time.Time      `json:"end_time,omitempty"` // When this step ended
	Metadata  map[string]any `json:"metadata,omitempty"` // Additional metadata
	Children  []TraceEntry   `json:"children,omitempty"` // Nested traces (for agents)

	// LLM-specific fields (populated for TraceTypeLLMCall)
	Model            string `json:"model,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`

	// Resource reference (for TraceTypeRetrieval)
	// EXTENSION: Expand for your resource types
	ResourceType string `json:"resource_type,omitempty"` // "document", "url", "database"
	ResourceID   string `json:"resource_id,omitempty"`   // Reference to the resource
}

// TraceData wraps the complete trace for a message.
// This is stored as JSONB in the database.
type TraceData struct {
	Entries   []TraceEntry `json:"entries"`               // All trace entries
	TotalCost float64      `json:"total_cost,omitempty"`  // Estimated cost in USD
	Model     string       `json:"model,omitempty"`       // Primary model used
	Duration  int64        `json:"duration_ms,omitempty"` // Total duration in ms
}

// ============================================================================
// GORM JSONB SUPPORT
// ============================================================================

// JSONB is a wrapper for storing JSON data in PostgreSQL JSONB columns.
// USAGE: Use this type for any field that needs JSONB storage.
type JSONB map[string]any

// Value implements driver.Valuer for database writes.
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner for database reads.
func (j *JSONB) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case json.RawMessage:
		data = []byte(v)
	default:
		// Be permissive in case the driver returns a non-standard type.
		b, err := json.Marshal(v)
		if err != nil {
			return errors.New("type assertion to []byte failed")
		}
		data = b
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*j = nil
		return nil
	}
	return json.Unmarshal(data, j)
}

// TraceDataJSON is a GORM-compatible wrapper for TraceData.
type TraceDataJSON struct {
	TraceData
}

// Value implements driver.Valuer.
func (t TraceDataJSON) Value() (driver.Value, error) {
	if len(t.Entries) == 0 {
		return nil, nil
	}
	return json.Marshal(t.TraceData)
}

// Scan implements sql.Scanner.
func (t *TraceDataJSON) Scan(value any) error {
	if value == nil {
		t.TraceData = TraceData{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case json.RawMessage:
		data = []byte(v)
	default:
		// Be permissive in case the driver returns a non-standard type.
		b, err := json.Marshal(v)
		if err != nil {
			return errors.New("type assertion to []byte failed")
		}
		data = b
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		t.TraceData = TraceData{}
		return nil
	}
	return json.Unmarshal(data, &t.TraceData)
}

// ============================================================================
// TRACE BUILDER HELPERS
// ============================================================================

// NewTraceEntry creates a new trace entry with the current time.
func NewTraceEntry(traceType, name string) TraceEntry {
	return TraceEntry{
		ID:        generateTraceID(),
		Type:      traceType,
		Name:      name,
		StartTime: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// Complete marks the trace entry as complete with current time.
func (t *TraceEntry) Complete() {
	t.EndTime = time.Now()
}

// AddChild adds a nested trace entry.
func (t *TraceEntry) AddChild(child TraceEntry) {
	t.Children = append(t.Children, child)
}

// SetLLMMetadata sets LLM-specific metadata.
func (t *TraceEntry) SetLLMMetadata(model string, promptTokens, completionTokens int) {
	t.Model = model
	t.PromptTokens = promptTokens
	t.CompletionTokens = completionTokens
	t.TotalTokens = promptTokens + completionTokens
}

// generateTraceID generates a simple trace ID.
// EXTENSION: Replace with UUID or other ID format as needed.
func generateTraceID() string {
	return time.Now().Format("20060102150405.000000")
}
