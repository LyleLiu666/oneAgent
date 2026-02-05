package tool

import "context"

const (
	contextKeyPromptBaseOverride contextKey = "promptBaseOverride"
	contextKeyMountedToolIDs     contextKey = "mountedToolIDs"
)

// ContextWithPromptBaseOverride stores the raw base prompt override string (before tool manuals injection).
// Empty means "use built-in base persona".
func ContextWithPromptBaseOverride(ctx context.Context, baseOverride string) context.Context {
	return context.WithValue(ctx, contextKeyPromptBaseOverride, baseOverride)
}

func PromptBaseOverrideFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyPromptBaseOverride).(string); ok {
		return v
	}
	return ""
}

// ContextWithMountedToolIDs stores the tool IDs currently mounted in the parent context.
// This enables tools (e.g. subagent) to inherit the parent's effective tool set by default.
func ContextWithMountedToolIDs(ctx context.Context, toolIDs []string) context.Context {
	copied := append([]string(nil), toolIDs...)
	return context.WithValue(ctx, contextKeyMountedToolIDs, copied)
}

func MountedToolIDsFromContext(ctx context.Context) ([]string, bool) {
	if v, ok := ctx.Value(contextKeyMountedToolIDs).([]string); ok {
		return append([]string(nil), v...), true
	}
	return nil, false
}
