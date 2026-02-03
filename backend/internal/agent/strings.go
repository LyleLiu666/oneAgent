package agent

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 || s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

