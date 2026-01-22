package sbe

import "testing"

func TestParseSmartEditCommand_TrimsDirectives_AndAppendsFinalBlock(t *testing.T) {
	raw := "apply_edit <<'EOF'\n file: foo.txt\n <<<< SEARCH\nhello\n ==== REPLACE\nworld\nEOF\n"

	blocks, err := ParseSmartEditCommand(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	block := blocks[0]
	if block.FilePath != "foo.txt" {
		t.Fatalf("expected filePath %q, got %q", "foo.txt", block.FilePath)
	}
	if len(block.Search) != 1 || block.Search[0] != "hello" {
		t.Fatalf("expected search [hello], got %#v", block.Search)
	}
	if len(block.Replace) != 1 || block.Replace[0] != "world" {
		t.Fatalf("expected replace [world], got %#v", block.Replace)
	}
}
