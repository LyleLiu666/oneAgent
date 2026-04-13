package tool

import "strings"

var legacyToolNameAliases = map[string]string{
	// Skill management.
	"skill.read": "skill_read",

	// Document export.
	"document.export": "document_export",

	// Compatibility with common agent tool names.
	"read":           "read_file",
	"read_from_file": "read_file",
	"view_file":      "read_file",
	"write":          "write_file",
	"write_to_file":  "write_file",

	// Formal memory tools.
	"memory.recall":   "memory_recall",
	"memory.remember": "memory_remember",
	"memory.forget":   "memory_forget",

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
