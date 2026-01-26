package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

const (
	ToolIDBash       = "bash"
	ToolIDEdit       = "edit"
	ToolIDGlob       = "glob"
	ToolIDLs         = "ls"
	ToolIDMultiEdit  = "multiedit"
	ToolIDPlan       = "plan"
	ToolIDRg         = "rg"
	ToolIDSearch     = "search"
	ToolIDRunCommand = "run_command"
	ToolIDSkillRead  = "skill_read"
	ToolIDSubagent   = "subagent"
	ToolIDWriteFile  = "write_file"
)

// Context keys for passing user information to tool handlers
type contextKey string

const (
	contextKeyUserID contextKey = "userID"
)

// ContextWithUserID returns a new context with the userID value
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// UserIDFromContext extracts the userID from context, returns empty string if not found
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyUserID).(string); ok {
		return v
	}
	return ""
}

// Handler executes a tool with raw JSON arguments.
type Handler func(ctx context.Context, raw json.RawMessage) (any, error)

// Definition describes a tool plus its execution handler.
type Definition struct {
	ID      string
	Spec    llm.Tool
	Handler Handler
}

type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func newDefinition(id string, spec llm.Tool, handler Handler) Definition {
	spec.ID = id
	return Definition{
		ID:      id,
		Spec:    spec,
		Handler: handler,
	}
}

var registry = map[string]Definition{
	ToolIDBash:       bashDefinition(),
	ToolIDEdit:       smartEditDefinition(),
	ToolIDGlob:       globDefinition(),
	ToolIDLs:         lsDefinition(),
	ToolIDMultiEdit:  multiEditDefinition(),
	ToolIDPlan:       planDefinition(),
	ToolIDRg:         rgDefinition(),
	ToolIDSearch:     searchDefinition(),
	ToolIDRunCommand: runCommandDefinition(),
	ToolIDSkillRead:  skillReadDefinition(),
	ToolIDSubagent:   subagentDefinition(),
	ToolIDWriteFile:  writeFileDefinition(),
}

// All returns every tool in stable ID order.
func All() []Definition {
	defs := make([]Definition, 0, len(registry))
	for _, def := range registry {
		defs = append(defs, def)
	}
	return sortDefinitionsByID(defs)
}

// Mount returns the requested tools in stable ID order.
func Mount(ids []string) ([]Definition, error) {
	if len(ids) == 0 {
		return []Definition{}, nil
	}

	seen := make(map[string]bool)
	defs := make([]Definition, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		def, ok := registry[trimmed]
		if !ok {
			return nil, fmt.Errorf("unknown tool id: %s", trimmed)
		}
		seen[trimmed] = true
		defs = append(defs, def)
	}

	return sortDefinitionsByID(defs), nil
}

// ToolsForLLM converts tool definitions into LLM tool specs using stable order.
func ToolsForLLM(defs []Definition) []llm.Tool {
	ordered := make([]Definition, len(defs))
	copy(ordered, defs)
	ordered = sortDefinitionsByID(ordered)

	tools := make([]llm.Tool, 0, len(ordered))
	for _, def := range ordered {
		tools = append(tools, def.Spec)
	}
	return tools
}

// Infos returns tool metadata in stable ID order.
func Infos() []Info {
	defs := All()
	out := make([]Info, 0, len(defs))
	for _, def := range defs {
		out = append(out, Info{
			ID:          def.ID,
			Name:        def.Spec.Function.Name,
			Description: def.Spec.Function.Description,
		})
	}
	return out
}

func sortDefinitionsByID(defs []Definition) []Definition {
	sort.SliceStable(defs, func(i, j int) bool {
		return defs[i].ID < defs[j].ID
	})
	return defs
}
