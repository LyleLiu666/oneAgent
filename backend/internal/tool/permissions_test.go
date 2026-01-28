package tool

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestRequirePolicy_DenyIncludesRuleID(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectAllow,
		Rules: []permissions.Rule{
			{ID: "deny-bash", Effect: permissions.EffectDeny, ToolID: ToolIDBash},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())
	ctx := ContextWithPolicySnapshot(context.Background(), snap)

	_, err := RequirePolicy(ctx, ToolIDBash)
	if err == nil {
		t.Fatalf("expected deny error")
	}
	var deny *PolicyDenyError
	if !errors.As(err, &deny) {
		t.Fatalf("expected PolicyDenyError, got %T", err)
	}
	if deny.Decision.RuleID != "deny-bash" {
		t.Fatalf("expected rule_id deny-bash, got %q", deny.Decision.RuleID)
	}
	if !strings.Contains(err.Error(), "deny-bash") {
		t.Fatalf("expected error to include rule id, got %q", err.Error())
	}
}

func TestRequirePolicy_DefaultDenyReason(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p2",
		DefaultEffect: permissions.EffectDeny,
	}
	snap := permissions.ResolveSnapshot("bob", policy, time.Now())
	ctx := ContextWithPolicySnapshot(context.Background(), snap)

	_, err := RequirePolicy(ctx, ToolIDReadFile)
	if err == nil {
		t.Fatalf("expected deny error")
	}
	var deny *PolicyDenyError
	if !errors.As(err, &deny) {
		t.Fatalf("expected PolicyDenyError, got %T", err)
	}
	if deny.Decision.Reason != "default_deny" {
		t.Fatalf("expected default_deny, got %q", deny.Decision.Reason)
	}
}
