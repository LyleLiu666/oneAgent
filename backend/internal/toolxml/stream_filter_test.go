package toolxml

import (
	"strings"
	"testing"
)

func TestStreamFilter_PanicRepro(t *testing.T) {
	// U+0130 'İ' (LATIN CAPITAL LETTER I WITH DOT ABOVE)
	// UTF-8: C4 B0 (2 bytes)
	// Lowercase: 'i' (69) + Combining Dot Above (CC 87) -> 3 bytes
	// This shrinks the string length in bytes if we just count characters, but actually expands in bytes?
	// Input: "İ" (len 2). Lower: "i\u0307" (len 3).
	// Strings.ToLower expands it.

	// U+023A Ⱥ (LATIN CAPITAL LETTER A WITH STROKE)
	// Input: "Ⱥ" (len 2). Lower: "ⱥ" (len 3).
	// Expansion causes index to be out of bounds if applied to original.

	// Case 1: Expansion causing panic
	t.Run("ExpansionPanic", func(t *testing.T) {
		input := "\u023a</thinking>"
		filter := &streamFilter{
			inThinking:    true,
			thinkingClose: "</thinking>",
			pending:       "", // set pending via Feed
		}

		// detailed check:
		// pending len is 2 (Ⱥ) + 11 (</thinking>) = 13.
		// lower pending len is 3 (ⱥ) + 11 = 14.
		// tag index in lower is 3.
		// original code: f.pending[3+11:] -> 14 > 13 -> panic.

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Recovered from panic: %v", r)
			}
		}()

		// Feed the input.
		out := filter.Feed(input)
		if out != "" {
			t.Logf("Output: %q", out)
		}

		// detailed verification: should find the tag correctly.
		// indexCaseInsensitive("\u023a</thinking>", "</thinking>") should be 2.
		// f.pending should become empty or processed.

		if filter.inThinking {
			t.Errorf("Should have exited thinking mode")
		}
	})

	// Case 2: Shrinking (if any) or just general weirdness.
	// Kelvin sign U+212A 'K' lowercases to 'k'. 3 bytes -> 1 byte.
	t.Run("Shrinking", func(t *testing.T) {
		input := "\u212a</thinking>" // K (3 bytes)
		// Lower: k (1 byte).
		// tag index in lower: 1.
		// original code using lower index 1 on original string:
		// f.pending[1+11] = 12.
		// original string len: 3 + 11 = 14.
		// slice [12:] -> "</" (wrong cut).

		filter := &streamFilter{
			inThinking:    true,
			thinkingClose: "</thinking>",
			pending:       "",
		}

		filter.Feed(input)

		if filter.inThinking {
			t.Errorf("Should have exited thinking mode")
		}
		// The tag starts at 3 in original.
		// indexCaseInsensitive should return 3.
		// If it worked, pending is empty.
		if len(filter.pending) > 0 {
			t.Errorf("Pending should be empty, got %q", filter.pending)
		}
	})
}

func TestStreamFilter_Normal(t *testing.T) {
	// Normal ASCII case
	filter := &streamFilter{}
	// input := "Hello <thinking>think content</thinking> <tool_data>tool content</tool_data>"

	// We feed it in chunks to exercise buffering
	chunks := []string{
		"Hello <thin",
		"king>think ",
		"content</thi",
		"nking> <tool",
		"_data>tool ",
		"content</tool_data>",
	}

	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString(filter.Feed(c))
	}
	sb.WriteString(filter.Flush())

	got := sb.String()
	// Thinking and ToolData are strictly filtered out?
	// The original code:
	// If in thinking/tool_data, it consumes and discards.
	// If not, it outputs.
	// So expected: "Hello  " (plus spaces).

	expected := "Hello  "
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestIndexCaseInsensitive(t *testing.T) {
	tests := []struct {
		s, sub string
		want   int
	}{
		{"abc", "b", 1},
		{"ABC", "b", 1},
		{"aBc", "b", 1},
		{"", "a", -1},
		{"a", "", 0},
		{"hello", "ll", 2},
		{"HELLO", "ll", 2},
		{"HeLLo", "ll", 2},
		// Unicode
		{"İ", "i", -1}, // I-dot vs i. ToLower('İ') is 'i'+dot. strict ascii check: 'İ' != 'i'.
		// Wait, my implementation assumes `sub` is lowercase ASCII.
		// And `hasPrefixCaseInsensitive` assumes `sub` is lowercase ASCII.
		// "İ" (C4 B0) vs "i" (69).
		// s[0] is C4. 'A'<=C4<='Z' is false. C4 != 'i'. Return -1. Correct.
	}

	for _, tt := range tests {
		got := indexCaseInsensitive(tt.s, tt.sub)
		if got != tt.want {
			t.Errorf("indexCaseInsensitive(%q, %q) = %d, want %d", tt.s, tt.sub, got, tt.want)
		}
	}
}
