package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func cleanupOrphanAttemptWorktrees(ctx context.Context, rt *runtime.Runtime) {
	if rt == nil || rt.Layout == nil || rt.Tasks == nil {
		return
	}
	if worktreeKeepEnabled() {
		return
	}

	tasks, err := rt.Tasks.ListTasks("", "")
	if err != nil {
		return
	}
	tasksDir := filepath.Clean(rt.Layout.TasksDir) + string(os.PathSeparator)

	for _, t := range tasks {
		workspaceRoot := strings.TrimSpace(t.Workspace)
		for _, a := range t.Attempts {
			if a.Status == taskqueue.AttemptQueued || a.Status == taskqueue.AttemptRunning {
				continue
			}
			worktreeRoot := strings.TrimSpace(a.WorktreeRoot)
			if worktreeRoot == "" {
				continue
			}
			cleaned := filepath.Clean(worktreeRoot)
			if !strings.HasPrefix(cleaned, tasksDir) {
				continue
			}
			if _, err := os.Stat(cleaned); err != nil {
				continue
			}

			if err := removeWorktree(ctx, workspaceRoot, cleaned); err != nil {
				_ = rt.Tasks.AppendEvent(taskqueue.Event{
					TaskID:    t.ID,
					AttemptID: a.ID,
					Type:      "attempt.worktree.orphan_cleanup.failed",
					Message:   "orphan worktree cleanup failed",
					Data: map[string]any{
						"worktree_root": cleaned,
						"error":         err.Error(),
					},
				})
				continue
			}

			_ = rt.Tasks.AppendEvent(taskqueue.Event{
				TaskID:    t.ID,
				AttemptID: a.ID,
				Type:      "attempt.worktree.orphan_cleanup.succeeded",
				Message:   "orphan worktree cleaned up",
				Data: map[string]any{
					"worktree_root": cleaned,
				},
			})
		}
	}
}

