package llm

// BuildSessionSummaryMessage creates a cacheable assistant message for long-session
// compression summaries. This message is intended to be stable across subsequent turns
// until the next compression event.
func BuildSessionSummaryMessage(content string) ChatMessage {
	return ChatMessage{
		Role:           "assistant",
		Content:        content,
		ForceCacheable: true,
	}
}

