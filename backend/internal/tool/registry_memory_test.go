package tool

import "testing"

func TestAll_DoesNotExposeChatLocalMemoryTools(t *testing.T) {
	defs := All()
	for _, def := range defs {
		switch def.ID {
		case ToolIDMemoryRecall, ToolIDMemoryRemember, ToolIDMemoryForget:
			t.Fatalf("expected chat-local formal memory tools to stay out of global registry, found %q", def.ID)
		}
	}
}
