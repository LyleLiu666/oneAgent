package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/scope"
)

type PolicyDecision struct {
	Decision permissions.Decision
}

func RequirePolicy(ctx context.Context, toolID string) (permissions.Decision, error) {
	snap, ok := PolicySnapshotFromContext(ctx)
	policy := permissions.DefaultPolicy()
	if ok {
		policy = snap.Policy
	}
	dec := permissions.Evaluate(policy, toolID)
	if !dec.Allowed {
		return dec, fmt.Errorf("permission denied: %s", dec.Reason)
	}
	return dec, nil
}

func EnforceFileScope(root string, rel string, dec permissions.Decision) error {
	if len(dec.Constraints.FileScope) == 0 {
		return nil
	}
	if rel == "" {
		return nil
	}
	matched, err := scope.MatchAny(dec.Constraints.FileScope, rel)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("file_scope violation: %s", rel)
	}
	return nil
}

func IsReadOutsideWorkspaceDenied(dec permissions.Decision) bool {
	if dec.Constraints.ReadOutsideWorkspace == nil {
		return false
	}
	return !*dec.Constraints.ReadOutsideWorkspace
}

func CommandProfile(dec permissions.Decision, fallback string) string {
	if strings.TrimSpace(dec.Constraints.CommandProfile) != "" {
		return strings.TrimSpace(dec.Constraints.CommandProfile)
	}
	return strings.TrimSpace(fallback)
}

