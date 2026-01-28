package prompt

import (
	"strings"
	"testing"
)

func TestAssembleStablePrefix_IncludesToolManualsWhenEnabled(t *testing.T) {
	out, err := AssembleStablePrefix(AssembleInput{
		ToolNames: []string{"write_file", "bash"},
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if out.StablePrefix == "" {
		t.Fatalf("expected stable prefix")
	}
	if !strings.Contains(out.StablePrefix, "bash 工具使用说明") {
		t.Fatalf("expected bash manual included")
	}
	if !strings.Contains(out.StablePrefix, "write_file 工具使用说明") {
		t.Fatalf("expected write_file manual included")
	}
	// Key constraints must be present.
	if !strings.Contains(out.StablePrefix, "CDATA") {
		t.Fatalf("expected CDATA constraint present")
	}
	if !strings.Contains(out.StablePrefix, "heredoc") {
		t.Fatalf("expected heredoc constraint present")
	}
}

func TestAssembleStablePrefix_ExcludesToolManualsWhenNotEnabled(t *testing.T) {
	out, err := AssembleStablePrefix(AssembleInput{
		ToolNames: []string{"write_file"},
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if strings.Contains(out.StablePrefix, "bash 工具使用说明") {
		t.Fatalf("expected bash manual excluded when bash not enabled")
	}
}

func TestAssembleStablePrefix_SortsToolsDeterministically(t *testing.T) {
	a, err := AssembleStablePrefix(AssembleInput{ToolNames: []string{"bash", "write_file"}})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	b, err := AssembleStablePrefix(AssembleInput{ToolNames: []string{"write_file", "bash"}})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if a.StablePrefix != b.StablePrefix {
		t.Fatalf("expected stable output regardless of input order")
	}
}
