package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func worktreeKeepEnabled() bool {
	return strings.TrimSpace(os.Getenv("ONEAGENT_KEEP_WORKTREES")) == "1"
}

func createWorktree(ctx context.Context, workspaceRoot, worktreeRoot, baseCommitSHA string) error {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	worktreeRoot = strings.TrimSpace(worktreeRoot)
	baseCommitSHA = strings.TrimSpace(baseCommitSHA)
	if workspaceRoot == "" {
		return errors.New("workspace_root is required")
	}
	if worktreeRoot == "" {
		return errors.New("worktree_root is required")
	}
	if baseCommitSHA == "" {
		return errors.New("base_commit_sha is required")
	}
	if !isGitWorkspace(ctx, workspaceRoot) {
		return errors.New("workspace is not a git repository")
	}

	// Best-effort prune before creating a new worktree (helps after unclean shutdowns).
	_, _ = runCmd(ctx, workspaceRoot, "git", "worktree", "prune")

	// If the path already exists, try to remove it first to keep the operation idempotent.
	if _, err := os.Stat(worktreeRoot); err == nil {
		_ = removeWorktree(ctx, workspaceRoot, worktreeRoot)
		_ = os.RemoveAll(worktreeRoot)
	}

	if err := os.MkdirAll(filepath.Dir(worktreeRoot), 0o700); err != nil {
		return fmt.Errorf("create worktree parent dir: %w", err)
	}
	if _, err := runCmd(ctx, workspaceRoot, "git", "worktree", "add", "--detach", worktreeRoot, baseCommitSHA); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

func removeWorktree(ctx context.Context, workspaceRoot, worktreeRoot string) error {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	worktreeRoot = strings.TrimSpace(worktreeRoot)
	if workspaceRoot == "" {
		return errors.New("workspace_root is required")
	}
	if worktreeRoot == "" {
		return errors.New("worktree_root is required")
	}
	if !isGitWorkspace(ctx, workspaceRoot) {
		return errors.New("workspace is not a git repository")
	}

	// git worktree remove is best-effort; if it fails we still attempt to delete the dir.
	_, err := runCmd(ctx, workspaceRoot, "git", "worktree", "remove", "--force", worktreeRoot)
	_ = os.RemoveAll(worktreeRoot)
	_, _ = runCmd(ctx, workspaceRoot, "git", "worktree", "prune")
	return err
}

