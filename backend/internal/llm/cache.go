package llm

type CacheControl struct {
	Type string `json:"type"`
}

type cacheControlStyle int

const (
	cacheControlStyleNone cacheControlStyle = iota
	cacheControlStyleCacheControl
	cacheControlStyleCachePoint
)

// SupportsPromptCacheKey reports whether a provider accepts prompt_cache_key.
func SupportsPromptCacheKey(providerType string) bool {
	switch providerType {
	case ProviderTypeOpenAI,
		ProviderTypeOpenAIResponse,
		ProviderTypeDeepSeek,
		ProviderTypeZhipuAI,
		ProviderTypeMiniMax,
		ProviderTypeAntigravity,
		ProviderTypeCodex:
		return true
	default:
		return false
	}
}

func applyMessageCacheControl(messages []ChatMessage, style cacheControlStyle) []ChatMessage {
	if style == cacheControlStyleNone {
		return messages
	}

	indexes := cacheMessageIndexes(messages)
	out := make([]ChatMessage, len(messages))

	for i, msg := range messages {
		out[i] = msg
		if !indexes[i] {
			continue
		}

		switch style {
		case cacheControlStyleCacheControl:
			out[i].CacheControl = &CacheControl{Type: "ephemeral"}
			out[i].CachePoint = nil
		case cacheControlStyleCachePoint:
			out[i].CachePoint = &CacheControl{Type: "ephemeral"}
			out[i].CacheControl = nil
		}
	}

	return out
}

func cacheMessageIndexes(messages []ChatMessage) map[int]bool {
	indexes := make(map[int]bool)
	systemCount := 0

	for i, msg := range messages {
		if msg.Role == "system" {
			indexes[i] = true
			systemCount++
			if systemCount >= 2 {
				break
			}
		}
	}

	for i := len(messages) - 1; i >= 0 && i >= len(messages)-2; i-- {
		indexes[i] = true
	}

	return indexes
}
