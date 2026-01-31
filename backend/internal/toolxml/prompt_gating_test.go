package toolxml

import (
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestSystemPrompt_OnlyIncludesSupportedTools(t *testing.T) {
	defs, err := tool.Mount([]string{
		tool.ToolIDBash,
		tool.ToolIDReadFile,
		tool.ToolIDLSPDefinition,
		tool.ToolIDDocumentExport,
	})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	prompt := SystemPrompt(defs)
	if !strings.Contains(prompt, "<tool_name>bash</tool_name>") {
		t.Fatalf("expected prompt to include bash, got %q", prompt)
	}
	if !strings.Contains(prompt, "<tool_name>read_file</tool_name>") {
		t.Fatalf("expected prompt to include read_file, got %q", prompt)
	}
	if strings.Contains(prompt, "lsp_definition") {
		t.Fatalf("expected prompt to not include unsupported tool lsp_definition, got %q", prompt)
	}
	if strings.Contains(prompt, "document_export") {
		t.Fatalf("expected prompt to not include unsupported tool document_export, got %q", prompt)
	}
}
