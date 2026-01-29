package llm

import "strings"

func extractSSEDataLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	data := strings.TrimPrefix(line, "data:")
	return strings.TrimSpace(data), true
}

