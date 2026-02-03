package agent

import (
	"context"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type ToolProtocol string

const (
	ToolProtocolNone ToolProtocol = "none"
	ToolProtocolJSON ToolProtocol = "json"
	ToolProtocolXML  ToolProtocol = "xml"
)

type toolCaller interface {
	ChatCompletionWithTools(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error)
}

// SelectToolProtocol determines the effective tool protocol for an agent loop.
//
// Rules (best-effort):
// - If tools are not enabled, default to JSON (unless explicitly requested XML).
// - If provider supports native tools, use JSON (unless explicitly requested XML).
// - If tools are requested but provider lacks native tools support, fallback to XML.
func SelectToolProtocol(requested string, toolDefs []tool.Definition, client llm.Client) (protocol ToolProtocol, fellBack bool) {
	requested = strings.ToLower(strings.TrimSpace(requested))

	if requested == string(ToolProtocolXML) {
		return ToolProtocolXML, false
	}
	if requested == string(ToolProtocolNone) {
		return ToolProtocolNone, false
	}

	if len(toolDefs) == 0 {
		return ToolProtocolJSON, false
	}

	if _, ok := client.(toolCaller); ok {
		return ToolProtocolJSON, false
	}

	return ToolProtocolXML, true
}
