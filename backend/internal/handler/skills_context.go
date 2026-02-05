package handler

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/skillrecall"
)

type skillTurnContextCaps struct {
	HasSkillRead bool
	HasSubagent  bool
}

func buildSkillSuggestionTurnContext(ctx context.Context, manager *skill.Manager, workspaceRoot string, userMessage string, caps skillTurnContextCaps) string {
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

	if wantsSkillsHelpTurnContext(userMessage) {
		return formatSkillsHelpTurnContext(catalog)
	}

	eligibleCatalog := skill.FilterEligibleCatalog(catalog, nil)
	if eligibleCatalog == nil || len(eligibleCatalog.Skills) == 0 {
		return ""
	}

	if explicit, ok := skillrecall.ParseExplicitSkill(eligibleCatalog, userMessage); ok {
		return formatSkillSuggestion(explicit, caps)
	}

	result, err := skillrecall.Search(loadCtx, eligibleCatalog, userMessage, skillrecall.Options{MaxResults: 8, Timeout: 2 * time.Second}, nil)
	if err != nil || len(result.Candidates) == 0 {
		return ""
	}

	best := result.Candidates[0]
	if best.Score <= 0 {
		return ""
	}

	return formatSkillSuggestion(best.Skill, caps)
}

func formatSkillSuggestion(s skill.Skill, caps skillTurnContextCaps) string {
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
	if len(s.ToolIDs) > 0 {
		b.WriteString("  - 工具包 (tool_ids): ")
		b.WriteString(strings.Join(s.ToolIDs, ", "))
		b.WriteString("\n")
		if caps.HasSubagent {
			b.WriteString("  - 工具包用法: 建议调用 `subagent` 并把该 skill 放入 `skill_ids`；`tool_ids` 可省略（系统会从 skill 元数据带上），或显式传入。\n")
		}
	}
	if caps.HasSkillRead {
		b.WriteString("  - 使用方式: 调用 `skill_read`（按技能名称）读取该技能的 `SKILL.md`，再遵循其中指令执行（兼容旧名：`skill.read`）\n")
	}
	return b.String()
}

func wantsSkillsHelpTurnContext(userMessage string) bool {
	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		return false
	}

	lower := strings.ToLower(msg)
	hasSkillWord := strings.Contains(msg, "技能") || strings.Contains(lower, "skill") || strings.Contains(lower, "skills")
	if !hasSkillWord {
		return false
	}

	hasListIntent := strings.Contains(msg, "有哪些") ||
		strings.Contains(msg, "有什么") ||
		strings.Contains(msg, "列出") ||
		strings.Contains(msg, "列表") ||
		strings.Contains(msg, "可用") ||
		strings.Contains(lower, "list") ||
		strings.Contains(lower, "available") ||
		strings.Contains(lower, "what")
	return hasListIntent
}

func formatSkillsHelpTurnContext(catalog *skill.Catalog) string {
	if catalog == nil || len(catalog.Skills) == 0 {
		return ""
	}

	eligible := skill.FilterEligibleCatalog(catalog, nil)
	list := eligible.Skills
	if len(list) == 0 {
		return ""
	}

	ordered := make([]skill.Skill, len(list))
	copy(ordered, list)
	sort.Slice(ordered, func(i, j int) bool {
		return strings.Compare(ordered[i].ID, ordered[j].ID) < 0
	})

	const maxPreview = 10
	if len(ordered) > maxPreview {
		ordered = ordered[:maxPreview]
	}

	var b strings.Builder
	b.WriteString("## 技能帮助（可用 skills）\n")
	b.WriteString("你似乎在询问当前环境有哪些 skills 可用。为了避免靠猜导致连续 `skill_read` 错误，请优先使用以下入口：\n\n")
	b.WriteString("- UI：打开 Skills Governance 页面查看完整列表（/governance/skills）\n")
	b.WriteString("- CLI：运行 `oneagent skills status` 检查可用性/缺失依赖\n\n")
	b.WriteString("可用技能摘要（Top-10）：\n")
	for _, s := range ordered {
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
	b.WriteString("\n提示：确定要用某个技能时，调用 `skill_read`（按技能名称或 skill_id）读取 `SKILL.md` 再执行（兼容旧名：`skill.read`）。\n")
	return b.String()
}
