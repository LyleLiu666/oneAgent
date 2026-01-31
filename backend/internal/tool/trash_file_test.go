package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestTrashFileTool_TrashesFile(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	target := filepath.Join(resolvedRoot, "tmp", "a.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{"filePath": "tmp/a.txt"})
	gotAny, err := runTrashFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(trashFileResult)
	if !ok {
		t.Fatalf("expected trashFileResult, got %T", gotAny)
	}
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	if got.TrashID == "" {
		t.Fatalf("expected trash_id, got %+v", got)
	}
	if got.OriginalPath != target {
		t.Fatalf("expected original_path=%q, got %q", target, got.OriginalPath)
	}
	if !strings.Contains(got.TrashedPath, filepath.Join(resolvedRoot, ".oneagent", "trash")) {
		t.Fatalf("expected trashed_path under trash root, got %q", got.TrashedPath)
	}
	if got.IsDir {
		t.Fatalf("expected is_dir=false, got %+v", got)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected original to be gone, stat err=%v", err)
	}
	content, err := os.ReadFile(got.TrashedPath)
	if err != nil {
		t.Fatalf("read trashed payload: %v", err)
	}
	if string(content) != "hello\n" {
		t.Fatalf("unexpected payload content: %q", string(content))
	}
	if _, err := os.Stat(got.ManifestPath); err != nil {
		t.Fatalf("expected manifest to exist: %v", err)
	}
}

func TestTrashFileTool_TrashesDirectory(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	dir := filepath.Join(resolvedRoot, "tmp", "dir")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{"filePath": "tmp/dir"})
	gotAny, err := runTrashFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(trashFileResult)
	if !ok {
		t.Fatalf("expected trashFileResult, got %T", gotAny)
	}
	if !got.IsDir {
		t.Fatalf("expected is_dir=true, got %+v", got)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected original dir to be gone, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(got.TrashedPath, "a.txt")); err != nil {
		t.Fatalf("expected payload dir content to exist: %v", err)
	}
}

func TestTrashFileTool_EnforcesFileScopeOnTarget(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	policy := permissions.Policy{
		ID:            "test",
		DefaultEffect: permissions.EffectAllow,
		Rules: []permissions.Rule{
			{
				ID:     "allow-trash",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDTrashFile,
				Constraints: permissions.Constraints{
					FileScope: []string{"allowed/**"},
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("local", policy, time.Now())

	ctx := context.Background()
	ctx = ContextWithWorkspace(ctx, WorkspaceConfig{Enabled: true, Root: root})
	ctx = ContextWithPolicySnapshot(ctx, snap)
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	disallowed := filepath.Join(resolvedRoot, "tmp", "a.txt")
	if err := os.MkdirAll(filepath.Dir(disallowed), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(disallowed, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{"filePath": "tmp/a.txt"})
	_, err = runTrashFileTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected file_scope violation error, got nil")
	}
	if !strings.Contains(err.Error(), "file_scope violation") {
		t.Fatalf("expected file_scope violation, got %v", err)
	}
}
