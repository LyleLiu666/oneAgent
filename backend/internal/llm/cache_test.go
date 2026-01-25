package llm

import "testing"

func TestCacheMessageIndexes_SkipsVolatileMessages(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "stable-system-1"},
		{Role: "system", Content: "turn-context", Volatile: true},
		{Role: "system", Content: "stable-system-2"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "system", Content: "turn-context-2", Volatile: true},
	}

	idx := cacheMessageIndexes(msgs)

	if !idx[0] || !idx[2] {
		t.Fatalf("expected stable system messages to be cacheable (indexes 0,2), got %v", idx)
	}
	if idx[1] || idx[5] {
		t.Fatalf("expected volatile messages to be non-cacheable, got %v", idx)
	}

	if !idx[3] || !idx[4] {
		t.Fatalf("expected last two non-volatile messages to be cacheable (indexes 3,4), got %v", idx)
	}
}

func TestCacheMessageIndexes_IncludesForceCacheableMessages(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "stable-system-1"},
		{Role: "assistant", Content: "session-summary", ForceCacheable: true},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
	}

	idx := cacheMessageIndexes(msgs)
	if !idx[1] {
		t.Fatalf("expected force-cacheable message to be selected, got %v", idx)
	}
}

func TestCacheableMessageIndexes_TypicalSequence_WithSummaryAndToolMessages(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "assistant", Content: "summary", ForceCacheable: true},
		{Role: "user", Content: "u1"},
		{
			Role:    "assistant",
			Content: "calling tool",
			ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "rg",
					Arguments: `{"pattern":"x"}`,
				},
			}},
		},
		{Role: "tool", Content: `{"ok":true}`, ToolCallID: "call_1", Name: "rg"},
		{Role: "user", Content: "turn-context", Volatile: true},
		{Role: "user", Content: "u2"},
	}

	got := CacheableMessageIndexes(msgs)
	want := []int{0, 1, 4, 6}
	if !equalIntSlices(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
