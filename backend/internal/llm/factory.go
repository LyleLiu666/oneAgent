package llm

import (
	"fmt"
	"time"
)

const (
	ProviderTypeOpenAI         = "openai"
	ProviderTypeOpenAIResponse = "openai_response"
	ProviderTypeClaude         = "claude"
	ProviderTypeOpenRouter     = "openrouter"
	ProviderTypeBedrock        = "bedrock"
	ProviderTypeDeepSeek       = "deepseek"
	ProviderTypeZhipuAI        = "zhipuai"
	ProviderTypeMiniMax        = "minimax"
	ProviderTypeAntigravity    = "antigravity"
	ProviderTypeCodex          = "codex"
)

// ProviderConfig describes how to connect to an LLM provider.
type ProviderConfig struct {
	ProviderType string
	Endpoint     string
	APIKey       string
	Model        string
	Timeout      time.Duration
}

// NewClientForProvider creates an LLM client based on provider type.
func NewClientForProvider(cfg ProviderConfig) (Client, error) {
	switch cfg.ProviderType {
	case ProviderTypeOpenAI:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleNone), nil
	case ProviderTypeOpenAIResponse:
		return NewOpenAIResponsesClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}), nil
	case ProviderTypeClaude:
		return NewAnthropicClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}), nil
	case ProviderTypeOpenRouter:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleCacheControl), nil
	case ProviderTypeBedrock:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleCachePoint), nil
	case ProviderTypeDeepSeek:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleNone), nil
	case ProviderTypeZhipuAI:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleNone), nil
	case ProviderTypeMiniMax:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleNone), nil
	case ProviderTypeAntigravity:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleNone), nil
	case ProviderTypeCodex:
		return newOpenAIClient(ClientConfig{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
		}, cacheControlStyleNone), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.ProviderType)
	}
}
