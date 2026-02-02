package llm

import "strings"

func extractSSEDataLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	data := strings.TrimPrefix(line, "data:")
	return strings.TrimSpace(data), true
}

func looksLikeSSELine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	switch {
	case strings.HasPrefix(line, "data:"),
		strings.HasPrefix(line, "event:"),
		strings.HasPrefix(line, "id:"),
		strings.HasPrefix(line, "retry:"),
		strings.HasPrefix(line, ":"):
		return true
	default:
		return false
	}
}
