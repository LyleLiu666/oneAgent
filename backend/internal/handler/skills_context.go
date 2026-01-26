package handler

import (
	"context"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/skillrecall"
)

func buildSkillSuggestionTurnContext(ctx context.Context, manager *skill.Manager, workspaceRoot string, userMessage string) string {
	if manager == nil {
		return ""
	}
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return ""
	}

	loadCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	catalog, err := manager.Load(loadCtx, workspaceRoot)
	if err != nil || catalog == nil || len(catalog.Skills) == 0 {
		return ""
	}

	if explicit, ok := skillrecall.ParseExplicitSkill(catalog, userMessage); ok {
		return formatSkillSuggestion(explicit)
	}

	result, err := skillrecall.Search(loadCtx, catalog, userMessage, skillrecall.Options{MaxResults: 8, Timeout: 2 * time.Second}, nil)
	if err != nil || len(result.Candidates) == 0 {
		return ""
	}

	best := result.Candidates[0]
	if best.Score <= 0 {
		return ""
	}

	return formatSkillSuggestion(best.Skill)
}

func formatSkillSuggestion(s skill.Skill) string {
	name := strings.TrimSpace(s.Name)
	desc := strings.TrimSpace(s.Description)
	if name == "" {
		return ""
	}
	if desc == "" {
		desc = "（无描述）"
	}

	var b strings.Builder
	b.WriteString("## 技能建议（自动挑选）\n")
	b.WriteString("以下是为本轮任务自动挑选的“最相关技能建议”。你可以自行判断是否需要使用。\n\n")
	b.WriteString("- ")
	b.WriteString(name)
	b.WriteString(": ")
	b.WriteString(desc)
	b.WriteString("\n")
	b.WriteString("  - 来源: ")
	b.WriteString(string(s.Source))
	b.WriteString("\n")
	b.WriteString("  - 使用方式: 调用 `skill.read`（按技能名称）读取该技能的 `SKILL.md`，再遵循其中指令执行\n")
	return b.String()
}

