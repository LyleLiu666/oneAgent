package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile_ReplacesContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := AtomicWriteFile(p, []byte("new"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != "new" {
		t.Fatalf("unexpected: %q", string(b))
	}
}
