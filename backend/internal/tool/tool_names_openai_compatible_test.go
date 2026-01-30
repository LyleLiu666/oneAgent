package tool

import (
	"strings"
	"testing"
)

func TestToolFunctionNames_AreOpenAICompatible_NoDots(t *testing.T) {
	for _, def := range All() {
		name := strings.TrimSpace(def.Spec.Function.Name)
		if name == "" {
			continue
		}
		if strings.Contains(name, ".") {
			t.Fatalf("tool %q has a dotted function name %q; canonical tool names must not include dots", def.ID, name)
		}
	}
}

