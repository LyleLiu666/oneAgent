package tool

import (
	"context"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

const (
	contextKeyLLMClient    contextKey = "llmClient"
	contextKeyModelName    contextKey = "modelName"
	contextKeySystemPrompt contextKey = "systemPrompt"
)

func ContextWithLLMClient(ctx context.Context, client llm.Client) context.Context {
	return context.WithValue(ctx, contextKeyLLMClient, client)
}

func LLMClientFromContext(ctx context.Context) llm.Client {
	if v, ok := ctx.Value(contextKeyLLMClient).(llm.Client); ok {
		return v
	}
	return nil
}

func ContextWithModelName(ctx context.Context, modelName string) context.Context {
	return context.WithValue(ctx, contextKeyModelName, modelName)
}

func ModelNameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyModelName).(string); ok {
		return v
	}
	return ""
}

func ContextWithSystemPrompt(ctx context.Context, systemPrompt string) context.Context {
	return context.WithValue(ctx, contextKeySystemPrompt, systemPrompt)
}

func SystemPromptFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeySystemPrompt).(string); ok {
		return v
	}
	return ""
}

