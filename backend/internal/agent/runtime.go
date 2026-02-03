package agent

import (
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/toolxml"
)

// AgentRuntime is a built, executable agent configuration.
//
// This object is intended to be reused across turns as long as its stable inputs
// (tool set, protocol, system prompt assets) don't change.
type AgentRuntime struct {
	Spec AgentSpec

	Client llm.Client

	ToolProtocol         ToolProtocol
	ToolProtocolFellBack bool

	ToolDefs  []tool.Definition
	ToolNames []string

	StablePrefix  string
	PromptModules []string
}

func (r AgentRuntime) FullSystemPrompt() string {
	base := strings.TrimSpace(r.StablePrefix)
	if base == "" {
		return ""
	}
	if r.ToolProtocol == ToolProtocolXML && len(r.ToolDefs) > 0 {
		return strings.TrimSpace(base + "\n\n" + toolxml.SystemPrompt(r.ToolDefs))
	}
	return base
}

func (r AgentRuntime) ToolsForLLM() []llm.Tool {
	if r.ToolProtocol != ToolProtocolJSON || len(r.ToolDefs) == 0 {
		return nil
	}
	return tool.ToolsForLLM(r.ToolDefs)
}

// BuildMessages assembles an LLM message list from:
// - stable system prompt
// - persisted history (already converted to llm.ChatMessage)
// - volatile per-turn TurnContext (best-effort)
// - current user message
func (r AgentRuntime) BuildMessages(persisted []llm.ChatMessage, turnContext string, userMessage string) []llm.ChatMessage {
	out := make([]llm.ChatMessage, 0, 1+len(persisted)+2)

	if sys := strings.TrimSpace(r.FullSystemPrompt()); sys != "" {
		out = append(out, llm.BuildSystemMessage(sys))
	}
	if len(persisted) > 0 {
		out = append(out, persisted...)
	}
	if msg, ok := llm.BuildTurnContextMessage(turnContext); ok {
		out = append(out, msg)
	}
	out = append(out, llm.BuildUserMessage(userMessage))
	return out
}

