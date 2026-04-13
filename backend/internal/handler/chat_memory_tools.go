package handler

import (
	"context"
	"encoding/json"

	"github.com/liu_y/oneAgent/backend/internal/formalmemory"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/toolxml"
)

const chatFormalMemoryAgentID = "oneagent.chat.assistant"

type formalMemoryToolExecutor interface {
	ExecuteTool(ctx context.Context, req formalmemory.ToolInvokeRequest) (any, error)
}

type chatToolSet struct {
	RuntimeDefs        []tool.Definition
	RuntimeToolIDs     []string
	InheritableToolIDs []string
	MemoryToolsActive  bool
}

func buildChatLocalMemoryToolDefinitions(exec formalMemoryToolExecutor) []tool.Definition {
	if exec == nil {
		return nil
	}

	return []tool.Definition{
		newChatLocalMemoryToolDefinition(
			tool.ToolIDMemoryRecall,
			"memory.recall",
			"Recall formal memory items for the current chat context.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "要回忆的查询内容。"},
					"intent": map[string]any{
						"type":        "string",
						"description": "召回意图。",
						"enum":        []string{"task_continuity", "semantic_lookup", "evidence_lookup", "temporal_lookup", "general"},
					},
					"limit":            map[string]any{"type": "integer", "description": "返回条数，范围 1..20。"},
					"include_evidence": map[string]any{"type": "boolean", "description": "是否包含 evidence。"},
				},
				"required":             []string{"query", "intent", "limit"},
				"additionalProperties": false,
			},
			exec,
		),
		newChatLocalMemoryToolDefinition(
			tool.ToolIDMemoryRemember,
			"memory.remember",
			"Write a candidate memory for later promotion.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"candidate_type": map[string]any{
						"type":        "string",
						"description": "候选记忆类型。",
						"enum":        []string{"session_summary", "semantic", "evidence", "temporal_fact"},
					},
					"scope_kind": map[string]any{
						"type":        "string",
						"description": "目标 scope 类型。",
						"enum":        []string{"thread", "user", "project", "org", "agent"},
					},
					"source_kind": map[string]any{"type": "string", "description": "来源类型。"},
					"source_ref":  map[string]any{"type": "string", "description": "稳定来源引用。"},
					"confidence":  map[string]any{"type": "number", "description": "置信度。"},
					"payload":     map[string]any{"type": "object", "description": "候选记忆载荷。"},
				},
				"required":             []string{"candidate_type", "scope_kind", "source_kind", "source_ref", "confidence", "payload"},
				"additionalProperties": false,
			},
			exec,
		),
		newChatLocalMemoryToolDefinition(
			tool.ToolIDMemoryForget,
			"memory.forget",
			"Invalidate a formal memory or delete a candidate within the current host scope.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target_id": map[string]any{"type": "string", "description": "目标 memory 或 candidate id。"},
					"mode": map[string]any{
						"type":        "string",
						"description": "forget 模式。",
						"enum":        []string{"invalidate", "delete_candidate"},
					},
					"reason": map[string]any{"type": "string", "description": "执行 forget 的原因。"},
				},
				"required":             []string{"target_id", "mode", "reason"},
				"additionalProperties": false,
			},
			exec,
		),
	}
}

func newChatLocalMemoryToolDefinition(id, name, description string, parameters map[string]any, exec formalMemoryToolExecutor) tool.Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
	spec.ID = id

	return tool.Definition{
		ID:   id,
		Spec: spec,
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			meta, _ := tool.InvocationMetaFromContext(ctx)
			ws := tool.WorkspaceFromContext(ctx)
			return exec.ExecuteTool(ctx, formalmemory.ToolInvokeRequest{
				Name:          name,
				Arguments:     raw,
				RunID:         meta.RunID,
				TurnID:        meta.TurnID,
				UserID:        tool.UserIDFromContext(ctx),
				SessionID:     tool.SessionIDFromContext(ctx),
				WorkspaceRoot: ws.Root,
				AgentID:       chatFormalMemoryAgentID,
				Protocol:      meta.Protocol,
				ToolCallID:    meta.ToolCallID,
			})
		},
	}
}

func resolveChatToolSet(baseDefs, memoryDefs []tool.Definition, protocol string) chatToolSet {
	runtimeDefs := append([]tool.Definition(nil), baseDefs...)
	inheritableDefs := append([]tool.Definition(nil), baseDefs...)
	memoryActive := false

	if protocol == "xml" {
		runtimeDefs, _ = toolxml.FilterSupportedDefinitions(runtimeDefs)
		inheritableDefs = runtimeDefs
	} else if protocol == "json" && len(memoryDefs) > 0 {
		runtimeDefs = append(runtimeDefs, memoryDefs...)
		memoryActive = true
	}

	return chatToolSet{
		RuntimeDefs:        runtimeDefs,
		RuntimeToolIDs:     toolIDsFromDefinitions(runtimeDefs),
		InheritableToolIDs: toolIDsFromDefinitions(inheritableDefs),
		MemoryToolsActive:  memoryActive,
	}
}
