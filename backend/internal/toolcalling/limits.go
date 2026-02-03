package toolcalling

import (
	"context"
	"os"
	"strconv"
	"strings"
)

const (
	defaultChatToolMaxSteps = 200
	maxChatToolMaxStepsCap  = 2000
)

type contextKey string

const contextKeyChatToolMaxSteps contextKey = "oneagent_chat_tool_max_steps"

// ContextWithChatToolMaxSteps sets an override for the tool-loop max steps on the context.
// Values <= 0 are ignored. The value is clamped using the same cap rules as ChatToolMaxSteps.
func ContextWithChatToolMaxSteps(ctx context.Context, maxSteps int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if maxSteps <= 0 {
		return ctx
	}
	// Clamp using the same cap rules as ChatToolMaxSteps.
	if v := envInt("ONEAGENT_CHAT_TOOL_MAX_STEPS_CAP"); v > 0 && maxSteps > v {
		maxSteps = v
	}
	if maxSteps < 1 {
		maxSteps = 1
	}
	if maxSteps > maxChatToolMaxStepsCap {
		maxSteps = maxChatToolMaxStepsCap
	}
	return context.WithValue(ctx, contextKeyChatToolMaxSteps, maxSteps)
}

// ChatToolMaxStepsFromContext returns the effective max steps limit, honoring a context override.
func ChatToolMaxStepsFromContext(ctx context.Context) int {
	if ctx != nil {
		if v, ok := ctx.Value(contextKeyChatToolMaxSteps).(int); ok && v > 0 {
			return v
		}
	}
	return ChatToolMaxSteps()
}

// ChatToolMaxSteps returns the max number of tool-loop iterations allowed for chat.
//
// This limit applies to both JSON-native tool calling and XML tool_data fallback loops.
//
// Env overrides:
// - ONEAGENT_CHAT_TOOL_MAX_STEPS
// - ONEAGENT_CHAT_TOOL_MAX_STEPS_CAP
func ChatToolMaxSteps() int {
	maxSteps := defaultChatToolMaxSteps
	if v := envInt("ONEAGENT_CHAT_TOOL_MAX_STEPS"); v > 0 {
		maxSteps = v
	}
	if v := envInt("ONEAGENT_CHAT_TOOL_MAX_STEPS_CAP"); v > 0 && maxSteps > v {
		maxSteps = v
	}
	if maxSteps < 1 {
		return 1
	}
	if maxSteps > maxChatToolMaxStepsCap {
		return maxChatToolMaxStepsCap
	}
	return maxSteps
}

func envInt(key string) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return v
}
