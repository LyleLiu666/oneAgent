package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type GitBase struct {
	BaseCommitSHA string
	BaseRef       string
}

func resolveGitBase(ctx context.Context, workspaceRoot string) (GitBase, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return GitBase{}, errors.New("workspace_root is required")
	}
	if !isGitWorkspace(ctx, workspaceRoot) {
		return GitBase{}, errors.New("workspace is not a git repository")
	}

	sha, err := runCmd(ctx, workspaceRoot, "git", "rev-parse", "HEAD")
	if err != nil {
		return GitBase{}, fmt.Errorf("resolve base_commit_sha: %w", err)
	}
	ref, err := runCmd(ctx, workspaceRoot, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return GitBase{}, fmt.Errorf("resolve base_ref: %w", err)
	}

	return GitBase{
		BaseCommitSHA: strings.TrimSpace(string(sha)),
		BaseRef:       strings.TrimSpace(string(ref)),
	}, nil
}
