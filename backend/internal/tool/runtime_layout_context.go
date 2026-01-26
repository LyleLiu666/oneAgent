package tool

import (
	"context"

	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

const contextKeyRuntimeLayout contextKey = "runtimeLayout"

func ContextWithRuntimeLayout(ctx context.Context, layout *runtime.Layout) context.Context {
	return context.WithValue(ctx, contextKeyRuntimeLayout, layout)
}

func RuntimeLayoutFromContext(ctx context.Context) *runtime.Layout {
	if v, ok := ctx.Value(contextKeyRuntimeLayout).(*runtime.Layout); ok {
		return v
	}
	return nil
}

