package tool

import "strings"

var legacyToolNameAliases = map[string]string{
	// Skill management.
	"skill.read": "skill_read",

	// Document export.
	"document.export": "document_export",

	// Code intelligence (LSP).
	"lsp.definition":     "lsp_definition",
	"lsp.references":     "lsp_references",
	"lsp.rename_preview": "lsp_rename_preview",
}

// CanonicalToolName maps legacy dotted tool names to their canonical underscore form.
// It returns the input name unchanged when it is already canonical or unknown.
func CanonicalToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if mapped, ok := legacyToolNameAliases[name]; ok {
		return mapped
	}
	return name
}

