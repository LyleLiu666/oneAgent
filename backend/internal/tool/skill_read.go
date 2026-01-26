package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/skill"
)

type skillReadRequest struct {
	Name    string `json:"name,omitempty"`
	SkillID string `json:"skill_id,omitempty"`
	ID      string `json:"id,omitempty"`
}

type skillReadResult struct {
	OK bool `json:"ok"`

	SkillID string       `json:"skill_id"`
	Name    string       `json:"name"`
	Source  skill.Source `json:"source"`
	Path    string       `json:"path"`

	SkillMD string `json:"skill_md"`
}

func skillReadDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "skill.read",
			Description: "按技能名称/ID 读取该技能的 SKILL.md 原文（无需提供文件路径；同名冲突按优先级 .oneagent > .claude > .codex 选择最终生效版本）。返回 skill_md 作为工具输出，供你在后续执行中遵循其中的指令。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "技能名称（大小写不敏感；会做 -/_/空格 归一化）。",
					},
					"skill_id": map[string]any{
						"type":        "string",
						"description": "技能 ID（通常等于规范化 name）。",
					},
					"id": map[string]any{
						"type":        "string",
						"description": "skill_id 的别名（兼容）。",
					},
				},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDSkillRead, spec, runSkillReadTool)
}

func runSkillReadTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req skillReadRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	ident := strings.TrimSpace(req.SkillID)
	if ident == "" {
		ident = strings.TrimSpace(req.ID)
	}
	if ident == "" {
		ident = strings.TrimSpace(req.Name)
	}
	if ident == "" {
		return nil, errors.New("name or skill_id is required")
	}

	ws := WorkspaceFromContext(ctx)
	workspaceRoot := ""
	if ws.Enabled {
		workspaceRoot = ws.Root
	}

	manager := SkillManagerFromContext(ctx)
	if manager == nil {
		manager = skill.NewManager(0)
	}

	loadCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cat, err := manager.Load(loadCtx, workspaceRoot)
	if err != nil {
		return nil, err
	}

	norm := skill.NormalizeName(ident)
	var s skill.Skill
	var ok bool
	if norm != "" {
		s, ok = cat.ByID(norm)
		if !ok {
			s, ok = cat.ByName(norm)
		}
	}
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", ident)
	}

	data, err := skill.ReadSkillFile(s.Path, 512*1024)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if truncated, ok, _ := truncateToRunes(content, 20000); ok {
		content = truncated + "\n\n...(truncated)...\n"
	}

	return skillReadResult{
		OK:      true,
		SkillID: s.ID,
		Name:    s.Name,
		Source:  s.Source,
		Path:    s.Path,
		SkillMD: content,
	}, nil
}
