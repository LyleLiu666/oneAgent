package scope

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWritePath_NoWorkspace(t *testing.T) {
	if _, err := ResolveWritePath("", "a.txt", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveWritePath_OutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	_, err := ResolveWritePath(root, filepath.Join(filepath.Dir(root), "outside.txt"), nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrPathOutsideWorkspace {
		t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
	}
}

func TestResolveWritePath_SymlinkEscapeDenied(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := ResolveWritePath(root, filepath.Join("link", "file.txt"), nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrPathOutsideWorkspace {
		t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
	}
}

func TestResolveReadPath_SymlinkEscapeDenied(t *testing.T) {
	root := t.TempDir()

	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Dir(root), link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := ResolveReadPath(root, filepath.Join("link", "outside.txt"))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
	}
}

func TestResolveWritePath_ScopePatterns(t *testing.T) {
	root := t.TempDir()

	allowed, err := ResolveWritePath(root, "a/b/c.txt", []string{"a/**"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if allowed != filepath.Join(root, "a", "b", "c.txt") {
		t.Fatalf("unexpected resolved path: %q", allowed)
	}

	_, err = ResolveWritePath(root, "a/b/c.txt", []string{"a/*.txt"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrPathOutsideScope {
		t.Fatalf("expected ErrPathOutsideScope, got %v", err)
	}
}

func TestResolveWritePath_WorkspaceAlias(t *testing.T) {
	root := t.TempDir()

	got, err := ResolveWritePath(root, "/workspace/a.txt", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != filepath.Join(root, "a.txt") {
		t.Fatalf("unexpected resolved path: %q", got)
	}

	got, err = ResolveWritePath(root, "/b.txt", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != filepath.Join(root, "b.txt") {
		t.Fatalf("unexpected resolved path: %q", got)
	}

	got, err = ResolveWritePath(root, "workspace/b.txt", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != filepath.Join(root, "b.txt") {
		t.Fatalf("unexpected resolved path: %q", got)
	}

	_, err = ResolveWritePath(root, "/workspace/../escape.txt", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrPathOutsideWorkspace {
		t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
	}

	_, err = ResolveWritePath(root, "/../escape.txt", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrPathOutsideWorkspace {
		t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
	}
}

func TestResolveReadPath_WorkspaceAlias(t *testing.T) {
	root := t.TempDir()

	got, err := ResolveReadPath(root, "/workspace/a.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != filepath.Join(root, "a.txt") {
		t.Fatalf("unexpected resolved path: %q", got)
	}

	got, err = ResolveReadPath(root, "workspace/b.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != filepath.Join(root, "b.txt") {
		t.Fatalf("unexpected resolved path: %q", got)
	}
}
