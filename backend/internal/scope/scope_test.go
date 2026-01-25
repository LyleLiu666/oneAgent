package scope

import (
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

