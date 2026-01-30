package tool

import (
	"strings"
)

// InvalidArgumentsError indicates a tool call was rejected before execution due to invalid arguments.
// It is used for schema-level validation (e.g., invalid JSON or missing required fields).
type InvalidArgumentsError struct {
	ToolName      string
	MissingFields []string
	Message       string
}

func (e *InvalidArgumentsError) Error() string {
	if e == nil {
		return "invalid arguments"
	}
	msg := strings.TrimSpace(e.Message)
	if msg != "" {
		return msg
	}
	if len(e.MissingFields) > 0 {
		return "missing required fields: " + strings.Join(e.MissingFields, ", ")
	}
	return "invalid arguments"
}

