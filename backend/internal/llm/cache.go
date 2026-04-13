package llm

import (
	"strings"

	agentsdkprovider "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git/provider"
)

type CacheControl struct {
	Type string `json:"type"`
}

type cacheSelector struct {
	SystemCount int
	TailCount   int
}

func defaultCacheSelector() cacheSelector {
	return cacheSelector{
		SystemCount: 2,
		TailCount:   2,
	}
}

type cacheControlStyle int

const (
	cacheControlStyleNone cacheControlStyle = iota
	cacheControlStyleCacheControl
	cacheControlStyleCachePoint
)

type providerCacheCapabilities struct {
	SupportsPromptCacheKey bool
	CacheStyle             cacheControlStyle
	UsesAnthropicCaching   bool
}

func cacheCapabilitiesForProvider(providerType string) providerCacheCapabilities {
	pt := strings.ToLower(strings.TrimSpace(providerType))
	switch pt {
	case ProviderTypeClaude:
		return providerCacheCapabilities{
			SupportsPromptCacheKey: false,
			CacheStyle:             cacheControlStyleNone,
			UsesAnthropicCaching:   true,
		}
	case ProviderTypeOpenRouter:
		return providerCacheCapabilities{
			SupportsPromptCacheKey: false,
			CacheStyle:             cacheControlStyleCacheControl,
			UsesAnthropicCaching:   false,
		}
	case ProviderTypeBedrock:
		return providerCacheCapabilities{
			SupportsPromptCacheKey: false,
			CacheStyle:             cacheControlStyleCachePoint,
			UsesAnthropicCaching:   false,
		}
	case ProviderTypeOpenAI,
		ProviderTypeOpenAIResponse,
		ProviderTypeDeepSeek,
		ProviderTypeZhipuAI,
		ProviderTypeMiniMax,
		ProviderTypeAntigravity,
		ProviderTypeCodex:
		return providerCacheCapabilities{
			SupportsPromptCacheKey: agentsdkprovider.SupportsPromptCacheKey(pt),
			CacheStyle:             cacheControlStyleNone,
			UsesAnthropicCaching:   false,
		}
	default:
		return providerCacheCapabilities{
			SupportsPromptCacheKey: false,
			CacheStyle:             cacheControlStyleNone,
			UsesAnthropicCaching:   false,
		}
	}
}

// SupportsPromptCacheKey reports whether a provider accepts prompt_cache_key.
func SupportsPromptCacheKey(providerType string) bool {
	return agentsdkprovider.SupportsPromptCacheKey(strings.ToLower(strings.TrimSpace(providerType)))
}

func maybeDowngradePromptCaching(opts *ChatCompletionOptions, status int, respBody []byte) bool {
	if opts == nil || !opts.EnablePromptCache {
		return false
	}
	if status < 400 || status >= 500 {
		return false
	}
	msg := strings.ToLower(string(respBody))
	if !strings.Contains(msg, "cache") && !strings.Contains(msg, "prompt") && !strings.Contains(msg, "anthropic-beta") {
		return false
	}

	switch {
	case strings.Contains(msg, "prompt_cache_key") || strings.Contains(msg, "prompt cache key"):
		opts.PromptCacheKey = ""
		opts.PromptCacheDowngraded = true
		if opts.PromptCacheDowngradeReason == "" {
			opts.PromptCacheDowngradeReason = "prompt_cache_key unsupported"
		}
		return true
	case strings.Contains(msg, "cache_control") || strings.Contains(msg, "cachepoint") || strings.Contains(msg, "cache_point"):
		opts.EnablePromptCache = false
		opts.PromptCacheKey = ""
		opts.PromptCacheDowngraded = true
		if opts.PromptCacheDowngradeReason == "" {
			opts.PromptCacheDowngradeReason = "cache_control/cachePoint unsupported"
		}
		return true
	case strings.Contains(msg, "prompt-caching") || strings.Contains(msg, "anthropic-beta"):
		opts.EnablePromptCache = false
		opts.PromptCacheKey = ""
		opts.PromptCacheDowngraded = true
		if opts.PromptCacheDowngradeReason == "" {
			opts.PromptCacheDowngradeReason = "prompt caching unsupported"
		}
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

// CacheableMessageIndexes returns the cacheable message indexes selected by the
// current default cache selector. The indexes are returned in ascending order.
func CacheableMessageIndexes(messages []ChatMessage) []int {
	return agentsdkprovider.CacheableMessageIndexes(toSDKCacheMessages(messages))
}

func cacheMessageIndexes(messages []ChatMessage) map[int]bool {
	indexes := CacheableMessageIndexes(messages)
	out := make(map[int]bool, len(indexes))
	for _, idx := range indexes {
		out[idx] = true
	}
	return out
}

func (s cacheSelector) indexes(messages []ChatMessage) map[int]bool {
	indexes := make(map[int]bool)
	systemCount := 0

	for i, msg := range messages {
		if msg.Volatile {
			continue
		}
		if msg.Role == "system" {
			indexes[i] = true
			systemCount++
			if systemCount >= s.SystemCount {
				break
			}
		}
	}

	tailCount := 0
	for i := len(messages) - 1; i >= 0 && tailCount < s.TailCount; i-- {
		if messages[i].Volatile {
			continue
		}
		indexes[i] = true
		tailCount++
	}

	for i, msg := range messages {
		if msg.Volatile || !msg.ForceCacheable {
			continue
		}
		indexes[i] = true
	}

	return indexes
}

func toSDKCacheMessages(messages []ChatMessage) []agentsdkprovider.ChatMessage {
	out := make([]agentsdkprovider.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, agentsdkprovider.ChatMessage{
			Role:                msg.Role,
			Content:             msg.Content,
			PromptCacheBehavior: toSDKPromptCacheBehavior(msg),
		})
	}
	return out
}

func toSDKPromptCacheBehavior(msg ChatMessage) agentsdkprovider.PromptCacheBehavior {
	switch {
	case msg.Volatile:
		return agentsdkprovider.PromptCacheBehaviorVolatile
	case msg.ForceCacheable:
		return agentsdkprovider.PromptCacheBehaviorForceCacheable
	default:
		return agentsdkprovider.PromptCacheBehaviorDefault
	}
}
