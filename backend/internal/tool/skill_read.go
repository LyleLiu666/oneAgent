package tool

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
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
		return nil, errors.New(formatSkillNotFoundError(cat, ident))
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

func formatSkillNotFoundError(cat *skill.Catalog, ident string) string {
	ident = strings.TrimSpace(ident)
	norm := skill.NormalizeName(ident)

	suggestions := suggestSkills(cat, ident, 5)

	var b strings.Builder
	b.WriteString("skill not found\n")
	if ident != "" {
		b.WriteString("- requested: ")
		b.WriteString(ident)
		b.WriteString("\n")
	}
	if norm != "" && norm != ident {
		b.WriteString("- normalized: ")
		b.WriteString(norm)
		b.WriteString("\n")
	}

	if len(suggestions) > 0 {
		b.WriteString("\nDid you mean:\n")
		for _, s := range suggestions {
			name := strings.TrimSpace(s.Name)
			if name == "" {
				name = s.ID
			}
			b.WriteString("- ")
			b.WriteString(name)
			if strings.TrimSpace(s.ID) != "" && strings.TrimSpace(s.ID) != strings.TrimSpace(name) {
				b.WriteString(" (id=")
				b.WriteString(s.ID)
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\nNext steps:\n")
	b.WriteString("- Open Skills Governance UI: /governance/skills\n")
	b.WriteString("- Check availability: oneagent skills status\n")
	b.WriteString("- Verify spelling and try again with the exact skill_id\n")

	return strings.TrimSpace(b.String())
}

func suggestSkills(cat *skill.Catalog, ident string, limit int) []skill.Skill {
	if cat == nil || len(cat.Skills) == 0 || limit <= 0 {
		return nil
	}
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return nil
	}

	norm := skill.NormalizeName(ident)
	if norm == "" {
		return nil
	}

	tokens := strings.FieldsFunc(norm, func(r rune) bool {
		return r == '-' || r == '_' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	tokens = uniqueStrings(tokens)

	type scored struct {
		skill skill.Skill
		score int
	}
	scoredList := make([]scored, 0, len(cat.Skills))
	for _, s := range cat.Skills {
		id := skill.NormalizeName(s.ID)
		name := skill.NormalizeName(s.Name)
		if id == "" && name == "" {
			continue
		}

		score := 0
		if strings.Contains(id, norm) || strings.Contains(name, norm) {
			score += 5
		}
		if strings.HasPrefix(id, norm) || strings.HasPrefix(name, norm) {
			score += 2
		}
		for _, tok := range tokens {
			if tok == "" {
				continue
			}
			if strings.Contains(id, tok) || strings.Contains(name, tok) {
				score++
			}
		}
		if score <= 0 {
			continue
		}

		scoredList = append(scoredList, scored{skill: s, score: score})
	}

	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		return strings.Compare(scoredList[i].skill.ID, scoredList[j].skill.ID) < 0
	})

	if len(scoredList) > limit {
		scoredList = scoredList[:limit]
	}

	out := make([]skill.Skill, 0, len(scoredList))
	for _, item := range scoredList {
		out = append(out, item.skill)
	}
	return out
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
