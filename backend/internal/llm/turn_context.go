package llm

import "strings"

// BuildTurnContextMessage builds a volatile system message intended for per-turn
// dynamic context (skills recommendation, plan status, observer results, etc).
//
// This message is intentionally excluded from cacheable selection.
func BuildTurnContextMessage(content string) (ChatMessage, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ChatMessage{}, false
	}
	return ChatMessage{
		Role:     "user",
		Content:  "【TurnContext（每轮变化，不参与缓存；由系统生成）】\n" + trimmed,
		Volatile: true,
	}, true
}
