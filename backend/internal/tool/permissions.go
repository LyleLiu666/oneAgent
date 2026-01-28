package tool

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/scope"
)

type PolicyDecision struct {
	Decision permissions.Decision
}

type PolicyDenyError struct {
	ToolID      string
	PrincipalID string
	PolicyID    string
	PolicyHash  string
	Decision    permissions.Decision
}

func (e *PolicyDenyError) Error() string {
	if e == nil {
		return "permission denied"
	}
	if strings.TrimSpace(e.Decision.RuleID) != "" {
		return fmt.Sprintf("permission denied: tool=%s reason=%s rule=%s", e.ToolID, e.Decision.Reason, e.Decision.RuleID)
	}
	return fmt.Sprintf("permission denied: tool=%s reason=%s", e.ToolID, e.Decision.Reason)
}

func RequirePolicy(ctx context.Context, toolID string) (permissions.Decision, error) {
	snap, ok := PolicySnapshotFromContext(ctx)
	policy := permissions.DefaultPolicy()
	if ok {
		policy = snap.Policy
	} else {
		snap = permissions.ResolveSnapshot("", policy, time.Now())
	}
	dec := permissions.Evaluate(policy, toolID)
	if !dec.Allowed {
		deny := newPolicyDenyError(toolID, snap, dec)
		logPolicyDeny(deny)
		return dec, deny
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

func newPolicyDenyError(toolID string, snap permissions.Snapshot, dec permissions.Decision) *PolicyDenyError {
	toolID = strings.TrimSpace(toolID)
	policyID := strings.TrimSpace(snap.Policy.ID)
	if policyID == "" {
		policyID = "default"
	}
	policyHash := strings.TrimSpace(snap.PolicyHash)
	if policyHash == "" {
		policyHash = permissions.HashPolicy(snap.Policy)
	}
	principalID := strings.TrimSpace(snap.PrincipalID)
	if principalID == "" {
		principalID = "local"
	}
	return &PolicyDenyError{
		ToolID:      toolID,
		PrincipalID: principalID,
		PolicyID:    policyID,
		PolicyHash:  policyHash,
		Decision:    dec,
	}
}

func logPolicyDeny(err *PolicyDenyError) {
	if err == nil {
		return
	}
	log.Printf("tool permission denied tool=%s principal=%s reason=%s rule=%s policy_id=%s policy_hash=%s",
		err.ToolID,
		err.PrincipalID,
		err.Decision.Reason,
		err.Decision.RuleID,
		err.PolicyID,
		err.PolicyHash,
	)
}
