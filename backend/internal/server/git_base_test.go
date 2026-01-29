package server

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestResolveGitBase_GitWorkspace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	ws := t.TempDir()
	runGit(t, ws, "init")
	runGit(t, ws, "config", "user.email", "test@example.com")
	runGit(t, ws, "config", "user.name", "Test")
	writeFile(t, ws+"/a.txt", "hello\n")
	runGit(t, ws, "add", ".")
	runGit(t, ws, "commit", "-m", "init")

	got, err := resolveGitBase(context.Background(), ws)
	if err != nil {
		t.Fatalf("resolveGitBase: %v", err)
	}
	if len(strings.TrimSpace(got.BaseCommitSHA)) < 8 {
		t.Fatalf("expected base_commit_sha, got %+v", got)
	}
	if strings.TrimSpace(got.BaseRef) == "" {
		t.Fatalf("expected base_ref, got %+v", got)
	}
}

func TestResolveGitBase_NonGitWorkspace(t *testing.T) {
	ws := t.TempDir()
	_, err := resolveGitBase(context.Background(), ws)
	if err == nil {
		t.Fatalf("expected error for non-git workspace")
	}
}
