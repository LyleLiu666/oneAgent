package tool

import "testing"

func TestCanonicalToolName_AliasMapping(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"skill.read", "skill_read"},
		{"skill_read", "skill_read"},
		{"document.export", "document_export"},
		{"document_export", "document_export"},
		{"lsp.definition", "lsp_definition"},
		{"lsp_definition", "lsp_definition"},
		{"lsp.references", "lsp_references"},
		{"lsp_references", "lsp_references"},
		{"lsp.rename_preview", "lsp_rename_preview"},
		{"lsp_rename_preview", "lsp_rename_preview"},
		{"unknown", "unknown"},
	}

	for _, tc := range cases {
		got := CanonicalToolName(tc.in)
		if got != tc.want {
			t.Fatalf("CanonicalToolName(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

