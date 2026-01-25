package tool

import "context"

type WorkspaceConfig struct {
	Enabled    bool
	Root       string
	WriteScope []string
}

const (
	contextKeyWorkspace contextKey = "workspace"
)

func ContextWithWorkspace(ctx context.Context, workspace WorkspaceConfig) context.Context {
	return context.WithValue(ctx, contextKeyWorkspace, workspace)
}

func WorkspaceFromContext(ctx context.Context) WorkspaceConfig {
	if v, ok := ctx.Value(contextKeyWorkspace).(WorkspaceConfig); ok {
		return v
	}
	return WorkspaceConfig{}
}

