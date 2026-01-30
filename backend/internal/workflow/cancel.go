package workflow

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (s *Store) CancelRun(ctx context.Context, workspaceRoot string, workflowID string, runID string, reason string) (WorkflowRun, error) {
	if s == nil {
		return WorkflowRun{}, errors.New("store is nil")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "canceled"
	}
	return s.UpdateRun(ctx, workspaceRoot, workflowID, runID, func(cur *WorkflowRun) error {
		if cur.Status == RunStatusSucceeded || cur.Status == RunStatusFailed || cur.Status == RunStatusCanceled {
			return nil
		}
		now := time.Now().UTC()
		cur.Status = RunStatusCanceled
		cur.Error = reason
		cur.FinishedAt = now
		for id, nr := range cur.NodeRuns {
			if nr.Status == NodeStatusQueued || nr.Status == NodeStatusRunning {
				nr.Status = NodeStatusCanceled
				if nr.FinishedAt.IsZero() {
					nr.FinishedAt = now
				}
				cur.NodeRuns[id] = nr
			}
		}
		return nil
	})
}

