package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceCheckpoint_RestoreRestoresWorkspaceFiles(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "dir"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "dir", "b.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}

	outDir := t.TempDir()
	cp, err := CreateWorkspaceCheckpoint(context.Background(), workspace, outDir)
	if err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	if cp.ArchivePath == "" {
		t.Fatalf("expected archive path")
	}

	// Mutate workspace.
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatalf("mutate a.txt: %v", err)
	}
	if err := os.Remove(filepath.Join(workspace, "dir", "b.txt")); err != nil {
		t.Fatalf("remove b.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "new.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatalf("write new.txt: %v", err)
	}

	if err := RestoreWorkspaceCheckpoint(context.Background(), workspace, cp.ArchivePath); err != nil {
		t.Fatalf("restore checkpoint: %v", err)
	}

	gotA, err := os.ReadFile(filepath.Join(workspace, "a.txt"))
	if err != nil {
		t.Fatalf("read a.txt: %v", err)
	}
	if string(gotA) != "one\n" {
		t.Fatalf("unexpected a.txt: %q", string(gotA))
	}

	gotB, err := os.ReadFile(filepath.Join(workspace, "dir", "b.txt"))
	if err != nil {
		t.Fatalf("read b.txt: %v", err)
	}
	if string(gotB) != "two\n" {
		t.Fatalf("unexpected b.txt: %q", string(gotB))
	}

	if _, err := os.Stat(filepath.Join(workspace, "new.txt")); err == nil {
		t.Fatalf("expected new.txt to be removed on restore")
	}
}

func TestWorkspaceCheckpoint_RefusesFilesystemRoot(t *testing.T) {
	outDir := t.TempDir()
	_, err := CreateWorkspaceCheckpoint(context.Background(), string(filepath.Separator), outDir)
	if err == nil {
		t.Fatalf("expected error for filesystem root")
	}
}
