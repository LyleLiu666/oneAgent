package server

import (
	"context"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/gitutil"
)

type GitBase = gitutil.GitBase

func resolveGitBase(ctx context.Context, workspaceRoot string) (GitBase, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	return gitutil.ResolveGitBase(ctx, workspaceRoot)
}
