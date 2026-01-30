package toolxml

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStreamFilter_DoesNotSplitUTF8Runes_WhenKeepingSearchTail(t *testing.T) {
	filter := &streamFilter{}
	input := "你" + strings.Repeat("a", 14) // 17 bytes; forces boundary inside 3-byte rune if sliced naively.

	emitted := filter.Feed(input)
	if !utf8.ValidString(emitted) {
		t.Fatalf("expected emitted chunk to be valid UTF-8, got bytes=%x", []byte(emitted))
	}

	tail := filter.Flush()
	if !utf8.ValidString(tail) {
		t.Fatalf("expected tail chunk to be valid UTF-8, got bytes=%x", []byte(tail))
	}

	if got := emitted + tail; got != input {
		t.Fatalf("expected combined output %q, got %q", input, got)
	}
}

func TestKeepTail_PreservesUTF8Boundaries(t *testing.T) {
	input := "你" + strings.Repeat("a", 14) // 17 bytes; keepTail(16) starts inside rune.
	tail := keepTail(input, 16)

	if !utf8.ValidString(tail) {
		t.Fatalf("expected tail to be valid UTF-8, got bytes=%x", []byte(tail))
	}
	if !strings.HasSuffix(input, tail) {
		t.Fatalf("expected %q to have suffix %q", input, tail)
	}
	if len(tail) < 16 || len(tail) > 16+utf8.UTFMax {
		t.Fatalf("expected tail length within [%d,%d], got %d", 16, 16+utf8.UTFMax, len(tail))
	}
}
