package tool

import (
	"context"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

func resolveWorkspaceRoot(ctx context.Context) (string, error) {
	cfg := config.GetConfig()
	if cfg.BashRootDirExplicit && strings.TrimSpace(cfg.BashRootDir) != "" {
		return shell.ResolveBashRoot(cfg.BashRootDir)
	}

	ws := WorkspaceFromContext(ctx)
	if ws.Enabled && strings.TrimSpace(ws.Root) != "" {
		return scope.NormalizeWorkspaceRoot(ws.Root)
	}

	return "", scope.ErrWorkspaceNotSet
}

func resolvePathForRead(ctx context.Context, input string) (string, string, error) {
	root, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		return "", "", err
	}
	target, err := scope.ResolveReadPath(root, input)
	if err != nil {
		return "", "", err
	}
	return root, target, nil
}

func resolvePathForWrite(ctx context.Context, input string) (string, string, error) {
	root, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		return "", "", err
	}
	ws := WorkspaceFromContext(ctx)
	target, err := scope.ResolveWritePath(root, input, ws.WriteScope)
	if err != nil {
		return "", "", err
	}
	return root, target, nil
}

