package handler

import (
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/agent"
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
	p, fellBack := agent.SelectToolProtocol(requested, toolDefs, client)
	return string(p), fellBack
}
