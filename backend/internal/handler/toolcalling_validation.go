package handler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func extractSchemaRequiredFields(parameters map[string]any) []string {
	if parameters == nil {
		return nil
	}
	raw, ok := parameters["required"]
	if !ok || raw == nil {
		return nil
	}

	out := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	switch v := raw.(type) {
	case []string:
		for _, item := range v {
			add(item)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}

	return out
}

func validateToolArgsRequired(toolName string, parameters map[string]any, rawArgs string) *tool.InvalidArgumentsError {
	required := extractSchemaRequiredFields(parameters)
	if len(required) == 0 {
		return nil
	}

	rawArgs = strings.TrimSpace(rawArgs)
	if rawArgs == "" {
		return &tool.InvalidArgumentsError{
			ToolName: toolName,
			Message:  "arguments is empty",
		}
	}
	if !json.Valid([]byte(rawArgs)) {
		return &tool.InvalidArgumentsError{
			ToolName: toolName,
			Message:  "arguments must be valid JSON",
		}
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(rawArgs), &obj); err != nil {
		return &tool.InvalidArgumentsError{
			ToolName: toolName,
			Message:  "arguments must be a JSON object",
		}
	}

	missing := make([]string, 0, len(required))
	for _, field := range required {
		v, ok := obj[field]
		if !ok || v == nil {
			missing = append(missing, field)
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return &tool.InvalidArgumentsError{
			ToolName:      toolName,
			MissingFields: missing,
			Message:       fmt.Sprintf("missing required fields: %s", strings.Join(missing, ", ")),
		}
	}

	return nil
}

func deriveToolOutcome(outputJSON []byte) (ok bool, errMsg string) {
	trimmed := strings.TrimSpace(string(outputJSON))
	if trimmed == "" {
		return true, ""
	}
	if !json.Valid([]byte(trimmed)) {
		return false, "tool output is not valid JSON"
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return false, "tool output is not a JSON object"
	}

	if v, ok := obj["ok"].(bool); ok {
		if v {
			return true, ""
		}
		msg := strings.TrimSpace(asString(obj["message"]))
		if msg != "" {
			return false, msg
		}
		errVal := strings.TrimSpace(asString(obj["error"]))
		if errVal != "" {
			return false, errVal
		}
		return false, "tool returned ok=false"
	}

	if _, hasErr := obj["error"]; hasErr {
		msg := strings.TrimSpace(asString(obj["message"]))
		if msg != "" {
			return false, msg
		}
		errVal := strings.TrimSpace(asString(obj["error"]))
		if errVal != "" {
			return false, errVal
		}
		return false, "tool returned error"
	}

	return true, ""
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

