package agent

import (
	"errors"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/prompt"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/toolxml"
)

type BuildRequest struct {
	Spec AgentSpec

	Client llm.Client

	PolicySnapshot permissions.Snapshot
}

type Factory struct {
	AssembleStablePrefix func(input prompt.AssembleInput) (prompt.AssembleOutput, error)
}

func NewFactory() *Factory {
	return &Factory{
		AssembleStablePrefix: prompt.AssembleStablePrefix,
	}
}

func (f *Factory) Build(req BuildRequest) (AgentRuntime, error) {
	if f == nil {
		return AgentRuntime{}, errors.New("agent factory is nil")
	}
	if f.AssembleStablePrefix == nil {
		return AgentRuntime{}, errors.New("assemble stable prefix is required")
	}
	if req.Client == nil {
		return AgentRuntime{}, errors.New("llm client is required")
	}

	spec := req.Spec.Normalize()

	toolIDs := make([]string, 0, len(spec.ToolIDs))
	for _, id := range spec.ToolIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			toolIDs = append(toolIDs, trimmed)
		}
	}

	var (
		defs []tool.Definition
		err  error
	)

	if spec.ToolProtocol != ToolProtocolNone && len(toolIDs) > 0 {
		defs, err = tool.MountWithSnapshot(toolIDs, req.PolicySnapshot)
		if err != nil {
			return AgentRuntime{}, err
		}
	}

	toolProtocol, fellBack := SelectToolProtocol(string(spec.ToolProtocol), defs, req.Client)
	if spec.ToolProtocol == ToolProtocolNone {
		toolProtocol = ToolProtocolNone
		fellBack = false
		defs = nil
	}

	if toolProtocol == ToolProtocolXML && len(defs) > 0 {
		filtered, _ := toolxml.FilterSupportedDefinitions(defs)
		defs = filtered
	}

	toolNames := make([]string, 0, len(defs))
	for _, d := range defs {
		toolNames = append(toolNames, d.Spec.Function.Name)
	}

	assembled, err := f.AssembleStablePrefix(prompt.AssembleInput{
		BaseOverride: spec.BaseOverride,
		ToolNames:    toolNames,
	})
	if err != nil {
		return AgentRuntime{}, err
	}

	return AgentRuntime{
		Spec:                 spec,
		Client:               req.Client,
		ToolProtocol:         toolProtocol,
		ToolProtocolFellBack: fellBack,
		ToolDefs:             defs,
		ToolNames:            toolNames,
		StablePrefix:         strings.TrimSpace(assembled.StablePrefix),
		PromptModules:        append([]string{}, assembled.Modules...),
	}, nil
}
