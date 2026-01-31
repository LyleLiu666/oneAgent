package tool

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestBashTool_DefaultSandboxMode_UsesNativeOnDarwinWhenAvailable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only: default sandbox mode is native when available")
	}
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		t.Skip("sandbox-exec not available")
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
					// No sandbox_mode provided: should default to native on darwin when available.
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	root := t.TempDir()
	// Workspace root is normalized via EvalSymlinks; keep command inputs consistent to avoid /var -> /private/var mismatch on macOS.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	ctx = ContextWithPolicySnapshot(ctx, snap)

	raw, _ := json.Marshal(map[string]any{
		"command": "cd " + root + " && rm -rf ./tmp",
	})
	anyRes, err := runBashTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	res, ok := anyRes.(BashToolResult)
	if !ok {
		t.Fatalf("expected BashToolResult, got %T", anyRes)
	}
	if res.SandboxMode != "native" {
		t.Fatalf("expected sandbox_mode=native, got %q", res.SandboxMode)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(err) {
		t.Fatalf("expected tmp deleted, stat err=%v", err)
	}
}
