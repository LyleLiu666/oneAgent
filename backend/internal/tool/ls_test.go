package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestLsTool_ListsDirectory(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(resolvedRoot, "dir", "sub"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resolvedRoot, "dir", "file.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"path": "dir",
	})
	gotAny, err := runLsTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(LsToolResult)
	if !ok {
		t.Fatalf("expected LsToolResult, got %T", gotAny)
	}
	if got.Path != filepath.Join(resolvedRoot, "dir") {
		t.Fatalf("expected path %q, got %q", filepath.Join(resolvedRoot, "dir"), got.Path)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got.Entries))
	}
	if got.Entries[0].Name != "file.txt" || got.Entries[0].IsDir {
		t.Fatalf("expected file.txt entry first, got %+v", got.Entries[0])
	}
	if got.Entries[1].Name != "sub" || !got.Entries[1].IsDir {
		t.Fatalf("expected sub dir entry second, got %+v", got.Entries[1])
	}
}

func TestLsTool_FilePath(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	if err := os.WriteFile(filepath.Join(resolvedRoot, "file.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"path": "file.txt",
	})
	gotAny, err := runLsTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(LsToolResult)
	if !ok {
		t.Fatalf("expected LsToolResult, got %T", gotAny)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got.Entries))
	}
	if got.Entries[0].Name != "file.txt" || got.Entries[0].IsDir {
		t.Fatalf("expected file.txt entry, got %+v", got.Entries[0])
	}
}
