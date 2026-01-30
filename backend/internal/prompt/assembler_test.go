package prompt

import (
	"strings"
	"testing"
)

func TestAssembleStablePrefix_IncludesToolManualsWhenEnabled(t *testing.T) {
	toolNames := []string{
		"write_file",
		"bash",
		"read_file",
		"edit",
		"edit_v2",
		"multiedit",
		"ls",
		"glob",
		"rg",
		"search",
		"run_command",
		"plan",
		"subagent",
		"skill_read",
		"document_export",
		"lsp_definition",
		"lsp_references",
		"lsp_rename_preview",
	}

	out, err := AssembleStablePrefix(AssembleInput{
		ToolNames: toolNames,
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
	if !strings.Contains(out.StablePrefix, "read_file 工具使用说明") {
		t.Fatalf("expected read_file manual included")
	}
	if !strings.Contains(out.StablePrefix, "lsp_definition 工具使用说明") {
		t.Fatalf("expected lsp_definition manual included")
	}

	expectModules := []string{
		"assets/tools/bash.md",
		"assets/tools/write_file.md",
		"assets/tools/read_file.md",
		"assets/tools/edit.md",
		"assets/tools/edit_v2.md",
		"assets/tools/multiedit.md",
		"assets/tools/ls.md",
		"assets/tools/glob.md",
		"assets/tools/rg.md",
		"assets/tools/search.md",
		"assets/tools/run_command.md",
		"assets/tools/plan.md",
		"assets/tools/subagent.md",
		"assets/tools/skill_read.md",
		"assets/tools/document_export.md",
		"assets/tools/lsp_definition.md",
		"assets/tools/lsp_references.md",
		"assets/tools/lsp_rename_preview.md",
	}
	for _, rel := range expectModules {
		if !contains(out.Modules, rel) {
			t.Fatalf("expected module injected: %s", rel)
		}
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

func TestAssembleStablePrefix_DottedToolNameLoadsUnderscoreManual(t *testing.T) {
	out, err := AssembleStablePrefix(AssembleInput{ToolNames: []string{"skill.read"}})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if !strings.Contains(out.StablePrefix, "skill_read 工具使用说明") {
		t.Fatalf("expected skill_read manual included for dotted tool name")
	}
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
