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
		// Special-case: models sometimes double-quote the entire JSON payload, which becomes a JSON string.
		if strings.HasPrefix(trimmed, "\"") {
			var unquoted string
			if err := json.Unmarshal([]byte(trimmed), &unquoted); err == nil {
				unquoted = strings.TrimSpace(unquoted)
				if unquoted != "" && (strings.HasPrefix(unquoted, "{") || strings.HasPrefix(unquoted, "[")) && json.Valid([]byte(unquoted)) {
					return unquoted
				}
			}
		}
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
