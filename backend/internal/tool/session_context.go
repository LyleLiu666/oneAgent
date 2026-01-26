package tool

import "context"

const contextKeySessionID contextKey = "sessionID"

func ContextWithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, contextKeySessionID, sessionID)
}

func SessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeySessionID).(string); ok {
		return v
	}
	return ""
}

