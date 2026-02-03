package agent

import "strings"

// AgentSpec is a declarative definition of an agent profile.
//
// It describes only stable configuration (KV-cache friendly). Per-turn dynamic
// inputs (e.g. tasks snapshot, skill recall results) MUST be injected via
// volatile TurnContext at runtime.
type AgentSpec struct {
	// ID is a stable identifier for the agent profile (e.g. "worker-chat", "secretary-su").
	ID string

	// BaseOverride replaces the default base persona module in the stable prefix when non-empty.
	BaseOverride string

	// ToolIDs is the allowlist of mounted tools (policy filtered).
	ToolIDs []string

	// ToolProtocol controls the tool loop protocol: "none" | "json" | "xml".
	// When empty, the factory selects a best-effort default.
	ToolProtocol ToolProtocol
}

func (s AgentSpec) Normalize() AgentSpec {
	out := s
	out.ID = strings.TrimSpace(out.ID)
	out.BaseOverride = strings.TrimSpace(out.BaseOverride)
	out.ToolProtocol = ToolProtocol(strings.ToLower(strings.TrimSpace(string(out.ToolProtocol))))
	return out
}

