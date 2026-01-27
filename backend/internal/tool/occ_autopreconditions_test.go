package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOCC_AutoInjectWriteFileRejectsOnDrift(t *testing.T) {
	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	ctx = ContextWithOCC(ctx, true)

	target := filepath.Join(root, "a.txt")
	if err := os.WriteFile(target, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Record fingerprint via read_file.
	readRaw, _ := json.Marshal(map[string]any{"filePath": "a.txt"})
	if _, err := runReadFileTool(ctx, readRaw); err != nil {
		t.Fatalf("read_file failed: %v", err)
	}

	// External modification.
	if err := os.WriteFile(target, []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeRaw, _ := json.Marshal(map[string]any{"filePath": "a.txt", "content": "new\n"})
	_, err := runWriteFileTool(ctx, writeRaw)
	if err == nil {
		t.Fatalf("expected precondition error, got nil")
	}
	if !strings.Contains(err.Error(), "precondition failed") {
		t.Fatalf("expected precondition failure, got: %v", err)
	}
}

func TestOCC_AutoInjectWriteFileUpdatesFingerprint(t *testing.T) {
	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	ctx = ContextWithOCC(ctx, true)

	target := filepath.Join(root, "a.txt")
	if err := os.WriteFile(target, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	readRaw, _ := json.Marshal(map[string]any{"filePath": "a.txt"})
	if _, err := runReadFileTool(ctx, readRaw); err != nil {
		t.Fatalf("read_file failed: %v", err)
	}

	writeRaw1, _ := json.Marshal(map[string]any{"filePath": "a.txt", "content": "v2\n"})
	if _, err := runWriteFileTool(ctx, writeRaw1); err != nil {
		t.Fatalf("write_file(1) failed: %v", err)
	}

	// If OCC fingerprint is not updated after write, this second write would fail.
	writeRaw2, _ := json.Marshal(map[string]any{"filePath": "a.txt", "content": "v3\n"})
	if _, err := runWriteFileTool(ctx, writeRaw2); err != nil {
		t.Fatalf("write_file(2) failed: %v", err)
	}
}

