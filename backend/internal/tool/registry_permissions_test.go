package tool

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestMount_DisabledToolByEnv(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_TOOL_BASH", "1")

	_, err := Mount([]string{ToolIDBash})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tool disabled by configuration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfos_ExcludesDisabledToolsByEnv(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_TOOL_BASH", "1")

	for _, info := range Infos() {
		if info.ID == ToolIDBash {
			t.Fatalf("expected bash to be excluded, got %+v", info)
		}
	}
}

func TestMountWithSnapshot_DeniedTool(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectAllow,
		Rules: []permissions.Rule{
			{ID: "deny-bash", Effect: permissions.EffectDeny, ToolID: ToolIDBash},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	_, err := MountWithSnapshot([]string{ToolIDBash}, snap)
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
}
