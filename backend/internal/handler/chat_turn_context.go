package handler

import (
	"context"
	"log"
	"strings"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/formalmemory"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type formalMemoryPreRecall interface {
	PreRecallTurnContext(ctx context.Context, req formalmemory.PreRecallRequest) (formalmemory.PreRecallResult, error)
}

const formalMemoryPreRecallDegradedTrace = "Formal memory pre-recall degraded; continuing without recalled context."

type chatTurnContextInput struct {
	Skills        *skill.Manager
	FormalMemory  formalMemoryPreRecall
	WorkspaceRoot string
	UserMessage   string
	UserID        string
	SessionID     string
	TurnID        string
	AgentID       string
	ToolDefs      []tool.Definition
}

type chatTurnContextResult struct {
	Content       string
	TraceMessages []string
}

func buildChatTurnContext(ctx context.Context, input chatTurnContextInput) chatTurnContextResult {
	sections := make([]string, 0, 2)
	traceMessages := make([]string, 0, 1)

	caps := skillCapsFromToolDefs(input.ToolDefs)
	if caps.HasSkillRead || caps.HasSubagent {
		skillSection := strings.TrimSpace(buildSkillSuggestionTurnContext(ctx, input.Skills, input.WorkspaceRoot, input.UserMessage, caps))
		if skillSection != "" {
			sections = append(sections, skillSection)
		}
	}

	if input.FormalMemory != nil {
		turnID := strings.TrimSpace(input.TurnID)
		if turnID == "" {
			turnID = uuid.NewString()
		}
		memorySection, err := input.FormalMemory.PreRecallTurnContext(ctx, formalmemory.PreRecallRequest{
			RunID:         "chat:" + strings.TrimSpace(input.SessionID),
			TurnID:        turnID,
			UserID:        input.UserID,
			SessionID:     input.SessionID,
			WorkspaceRoot: input.WorkspaceRoot,
			AgentID:       input.AgentID,
		})
		if err != nil {
			log.Printf("formalmemory prerecall failed: %v", err)
			traceMessages = append(traceMessages, formalMemoryPreRecallDegradedTrace)
		} else {
			if strings.TrimSpace(memorySection.TurnContext) != "" {
				sections = append(sections, strings.TrimSpace(memorySection.TurnContext))
			}
			if memorySection.Degraded {
				traceMessages = append(traceMessages, formalMemoryPreRecallDegradedTrace)
			}
		}
	}

	return chatTurnContextResult{
		Content:       strings.TrimSpace(strings.Join(sections, "\n\n")),
		TraceMessages: traceMessages,
	}
}

func skillCapsFromToolDefs(toolDefs []tool.Definition) skillTurnContextCaps {
	caps := skillTurnContextCaps{}
	for _, def := range toolDefs {
		switch def.ID {
		case tool.ToolIDSkillRead:
			caps.HasSkillRead = true
		case tool.ToolIDSubagent:
			caps.HasSubagent = true
		}
	}
	return caps
}
