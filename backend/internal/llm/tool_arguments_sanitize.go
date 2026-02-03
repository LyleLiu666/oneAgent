package llm

import (
	"encoding/json"
	"strings"
)

const maxRawToolArgumentsRunes = 4096

// SanitizeToolArgumentsJSON guarantees a non-empty, valid JSON string for provider tool arguments.
// If the input cannot be parsed as JSON, it is wrapped as `{"_raw": "<original>"}`.
func SanitizeToolArgumentsJSON(args string) string {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return "{}"
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}

	raw := trimmed
	if len([]rune(raw)) > maxRawToolArgumentsRunes {
		raw = string([]rune(raw)[:maxRawToolArgumentsRunes]) + "…"
	}

	payload := map[string]any{
		"_raw": raw,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}
