package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

func TestBashTool_CodingProfile_NoSandbox_FailsClosed(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-bash",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDBash,
				Constraints: permissions.Constraints{
					CommandProfile: "coding",
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
		"command": "ls",
	})
	_, err := runBashTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "coding profile requires sandbox_mode=docker|native") {
		t.Fatalf("expected docker-only rejection, got %q", err.Error())
	}
}

func TestRunCommandTool_CodingProfile_NoSandbox_FailsClosed(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-run-command",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDRunCommand,
				Constraints: permissions.Constraints{
					CommandProfile: "coding",
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
		"command":             "ls",
		"wait_seconds":        0,
		"max_runtime_seconds": 5,
	})
	_, err := runCommandTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "coding profile requires sandbox_mode=docker|native") {
		t.Fatalf("expected docker-only rejection, got %q", err.Error())
	}
}

func TestBashTool_CodingProfile_NativeSandbox_AllowsInRootDelete(t *testing.T) {
	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-bash",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDBash,
				Constraints: permissions.Constraints{
					CommandProfile: "coding",
					SandboxMode:    "native",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	ctx = ContextWithPolicySnapshot(ctx, snap)

	raw, _ := json.Marshal(map[string]any{
		"command": "rm -rf ./tmp",
	})
	anyRes, err := runBashTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	res, ok := anyRes.(BashToolResult)
	if !ok {
		t.Fatalf("expected BashToolResult, got %T", anyRes)
	}
	if strings.TrimSpace(res.SandboxMode) != "native" {
		t.Fatalf("expected sandbox_mode=native, got %q", res.SandboxMode)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected tmp deleted, stat err=%v", err)
	}
}

func TestBashTool_SystemInstall_NoSandbox_DoesNotDegradeToReadonly(t *testing.T) {
	// This is best-effort: Windows might not have bash in PATH in our environment.
	if runtime.GOOS == "windows" {
		t.Skip("bash sandbox not supported on windows in tests")
	}

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
		"command": "brew --version",
	})
	anyRes, err := runBashTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	res, ok := anyRes.(BashToolResult)
	if !ok {
		t.Fatalf("expected BashToolResult, got %T", anyRes)
	}
	if strings.TrimSpace(res.SandboxMode) != "host" {
		t.Fatalf("expected sandbox_mode=host, got %q", res.SandboxMode)
	}
}
