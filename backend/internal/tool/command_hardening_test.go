package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestBashTool_NoSandbox_DegradesToReadonly(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-bash",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDBash,
				Constraints: permissions.Constraints{
					CommandProfile: "dev",
					SandboxMode:    "none",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	ctx = ContextWithPolicySnapshot(ctx, snap)

	raw, _ := json.Marshal(map[string]any{
		"command": "touch hi.txt",
	})
	_, err := runBashTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "profile=readonly") {
		t.Fatalf("expected readonly rejection, got %q", err.Error())
	}
}

func TestRunCommandTool_NoSandbox_DegradesToReadonly(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-run-command",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDRunCommand,
				Constraints: permissions.Constraints{
					CommandProfile: "dev",
					SandboxMode:    "none",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	ctx = ContextWithPolicySnapshot(ctx, snap)

	raw, _ := json.Marshal(map[string]any{
		"action":              "start",
		"command":             "touch hi.txt",
		"wait_seconds":        0,
		"max_runtime_seconds": 5,
	})
	_, err := runCommandTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "profile=readonly") {
		t.Fatalf("expected readonly rejection, got %q", err.Error())
	}
}

