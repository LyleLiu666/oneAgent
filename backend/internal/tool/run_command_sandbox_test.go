package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestRunCommandTool_SandboxModeDocker_MissingDockerIsActionable(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	t.Setenv("PATH", t.TempDir())

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-run-command-docker",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDRunCommand,
				Constraints: permissions.Constraints{
					CommandProfile: "dev",
					SandboxMode:    "docker",
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
		"command":             "echo hi",
		"wait_seconds":        0,
		"max_runtime_seconds": 5,
	})

	_, err := runCommandTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "docker") {
		t.Fatalf("expected error to mention docker, got %q", err.Error())
	}
}
