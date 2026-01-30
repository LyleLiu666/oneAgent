package handler

import (
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

// selectToolProtocol determines the effective tool protocol for this request.
//
// Rules (best-effort):
// - If tools are not enabled, default to JSON (unless explicitly requested XML).
// - If provider supports native tools, use JSON (unless explicitly requested XML).
// - If tools are requested but provider lacks native tools support, fallback to XML.
func selectToolProtocol(requested string, toolDefs []tool.Definition, client llm.Client) (protocol string, fellBack bool) {
	requested = strings.ToLower(strings.TrimSpace(requested))

	if requested == "xml" {
		return "xml", false
	}

	if len(toolDefs) == 0 {
		return "json", false
	}

	if _, ok := client.(toolCaller); ok {
		return "json", false
	}

	return "xml", true
}
