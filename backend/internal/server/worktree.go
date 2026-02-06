package server

import (
	"context"
	"os"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/gitutil"
)

func worktreeKeepEnabled() bool {
	return strings.TrimSpace(os.Getenv("ONEAGENT_KEEP_WORKTREES")) == "1"
}

var worktreeCreateFn = gitutil.CreateWorktree
var worktreeRemoveFn = gitutil.RemoveWorktree

func createWorktree(ctx context.Context, workspaceRoot, worktreeRoot, baseCommitSHA string) error {
	return worktreeCreateFn(ctx, workspaceRoot, worktreeRoot, baseCommitSHA)
}

func removeWorktree(ctx context.Context, workspaceRoot, worktreeRoot string) error {
	return worktreeRemoveFn(ctx, workspaceRoot, worktreeRoot)
}
