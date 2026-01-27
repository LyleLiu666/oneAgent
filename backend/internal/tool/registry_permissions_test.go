package tool

import (
	"strings"
	"testing"
)

func TestMount_DisabledToolByEnv(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_TOOL_BASH", "1")

	_, err := Mount([]string{ToolIDBash})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tool disabled by configuration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfos_ExcludesDisabledToolsByEnv(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_TOOL_BASH", "1")

	for _, info := range Infos() {
		if info.ID == ToolIDBash {
			t.Fatalf("expected bash to be excluded, got %+v", info)
		}
	}
}

