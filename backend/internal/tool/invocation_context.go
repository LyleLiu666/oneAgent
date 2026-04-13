package tool

import "context"

const contextKeyInvocationMeta contextKey = "invocationMeta"

type InvocationMeta struct {
	RunID      string
	TurnID     string
	ToolCallID string
	ToolName   string
	Protocol   string
}

func ContextWithInvocationMeta(ctx context.Context, meta InvocationMeta) context.Context {
	return context.WithValue(ctx, contextKeyInvocationMeta, meta)
}

func InvocationMetaFromContext(ctx context.Context) (InvocationMeta, bool) {
	if ctx == nil {
		return InvocationMeta{}, false
	}
	v, ok := ctx.Value(contextKeyInvocationMeta).(InvocationMeta)
	return v, ok
}
