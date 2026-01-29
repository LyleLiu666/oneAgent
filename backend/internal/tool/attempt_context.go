package tool

import "context"

const contextKeyAttemptID contextKey = "attemptID"

func ContextWithAttemptID(ctx context.Context, attemptID string) context.Context {
	return context.WithValue(ctx, contextKeyAttemptID, attemptID)
}

func AttemptIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyAttemptID).(string); ok {
		return v
	}
	return ""
}
