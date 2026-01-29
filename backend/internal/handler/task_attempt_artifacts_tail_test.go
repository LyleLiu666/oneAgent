package handler

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileTailLimited_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, truncated, err := readFileTailLimited(path, 10)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if truncated {
		t.Fatalf("expected truncated=false")
	}
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestReadFileTailLimited_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	content := strings.Repeat("a", 50) + "TAIL"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, truncated, err := readFileTailLimited(path, 8)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !truncated {
		t.Fatalf("expected truncated=true")
	}
	want := content[len(content)-8:]
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestReadFileTailLimited_MissingFile(t *testing.T) {
	_, _, err := readFileTailLimited("/path/does/not/exist", 10)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

