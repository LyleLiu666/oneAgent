package sbe

import (
	"os"
	"path/filepath"
	"testing"
)

func countBareLF(b []byte) int {
	count := 0
	for i := 0; i < len(b); i++ {
		if b[i] != '\n' {
			continue
		}
		if i == 0 || b[i-1] != '\r' {
			count++
		}
	}
	return count
}

func TestApplyEditBlocks_CRLFFile_DoesNotProduceMixedLineEndings(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")

	if err := os.WriteFile(path, []byte("a\r\nb\r\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := ApplyEditBlocks([]EditBlock{{
		FilePath: path,
		Search:   []string{"b"},
		Replace:  []string{"c"},
	}})
	if err != nil {
		t.Fatalf("ApplyEditBlocks: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if mixed := countBareLF(out); mixed != 0 {
		t.Fatalf("expected no bare LF newlines, got %d; content=%q", mixed, string(out))
	}
	if string(out) != "a\r\nc\r\n" {
		t.Fatalf("expected CRLF file to stay CRLF, got %q", string(out))
	}
}
